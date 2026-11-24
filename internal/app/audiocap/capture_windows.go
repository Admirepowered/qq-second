//go:build windows

package audiocap

// WASAPI 环回采集：直接抓取系统正在播放的声音（不依赖麦克风、不受环境噪声影响）。
// 仅 Windows：IMMDeviceEnumerator → 默认渲染设备 → IAudioClient(LOOPBACK) →
// IAudioCaptureClient 读包 → 混单声道 → 重采样到 16kHz。

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"qqsecond/internal/app/recognize"
)

// ErrNotSupported 表示当前平台不支持系统声音采集。
var ErrNotSupported = errors.New("当前平台不支持采集系统声音")

var (
	modOle32                  = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx        = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize        = modOle32.NewProc("CoUninitialize")
	procCoCreateInstance      = modOle32.NewProc("CoCreateInstance")
	procCoTaskMemFree         = modOle32.NewProc("CoTaskMemFree")
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidMMDeviceEnumerator   = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidAudioClient          = guid{0x1CB9AD4C, 0xDBFA, 0x4C32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidAudioCaptureClient   = guid{0xC8ADBD64, 0xE71E, 0x48A0, [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
	ksDataFormatSubtypePCM  = guid{0x00000001, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
	ksDataFormatSubtypeFloat = guid{0x00000003, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
)

const (
	clsctxAll                 = 0x17
	eRender                   = 0
	eConsole                  = 0
	audclntShareModeShared    = 0
	audclntStreamFlagsLoopback = 0x00020000
	audclntBufferFlagsSilent  = 0x2
	coinitMultithreaded       = 0x0
	waveFormatPCM             = 1
	waveFormatFloat           = 3
	waveFormatExtensible      = 0xFFFE
)

// comObject 是所有 COM 接口的公共前缀（vtable 指针）。
type comObject struct {
	vtbl unsafe.Pointer
}

func (o *comObject) call(idx int, args ...uintptr) (uintptr, error) {
	if o == nil || o.vtbl == nil {
		return 0, errors.New("COM 对象为空")
	}
	table := (*[64]uintptr)(o.vtbl)
	fn := table[idx]
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(unsafe.Pointer(o)))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	if int32(r) < 0 {
		return r, fmt.Errorf("COM 调用失败 (0x%08X)", uint32(r))
	}
	return r, nil
}

func (o *comObject) release() {
	if o != nil && o.vtbl != nil {
		_, _ = o.call(2)
	}
}

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

// sampleLayout 描述采集到的样本格式。
type sampleLayout struct {
	channels   int
	rate       int
	bits       int
	isFloat    bool
	bytesPerSm int
}

func layoutOf(format *waveFormatEx) (sampleLayout, error) {
	l := sampleLayout{
		channels: int(format.Channels),
		rate:     int(format.SamplesPerSec),
		bits:     int(format.BitsPerSample),
	}
	tag := format.FormatTag
	if tag == waveFormatExtensible {
		// cbSize 之后是 wValidBitsPerSample(2) + dwChannelMask(4) + SubFormat(16)
		sub := (*guid)(unsafe.Pointer(uintptr(unsafe.Pointer(format)) + 24))
		switch *sub {
		case ksDataFormatSubtypeFloat:
			tag = waveFormatFloat
		case ksDataFormatSubtypePCM:
			tag = waveFormatPCM
		default:
			return l, fmt.Errorf("不支持的音频子格式")
		}
	}
	switch tag {
	case waveFormatFloat:
		l.isFloat = true
		if l.bits != 32 {
			return l, fmt.Errorf("不支持的浮点位宽: %d", l.bits)
		}
	case waveFormatPCM:
		if l.bits != 16 && l.bits != 24 && l.bits != 32 {
			return l, fmt.Errorf("不支持的 PCM 位宽: %d", l.bits)
		}
	default:
		return l, fmt.Errorf("不支持的音频格式标签: %d", tag)
	}
	l.bytesPerSm = l.bits / 8
	if l.channels <= 0 || l.rate <= 0 {
		return l, errors.New("音频格式无效")
	}
	return l, nil
}

// CaptureSystem 采集 seconds 秒系统正在播放的声音，返回 16kHz 单声道 PCM。
func CaptureSystem(ctx context.Context, seconds int) ([]int16, error) {
	if seconds <= 0 {
		seconds = 8
	}
	// COM 与线程绑定，必须锁定到同一 OS 线程
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	if int32(hr) < 0 && uint32(hr) != 0x80010106 { // RPC_E_CHANGED_MODE 可继续
		return nil, fmt.Errorf("CoInitializeEx 失败 (0x%08X)", uint32(hr))
	}
	defer procCoUninitialize.Call()

	var enumerator *comObject
	if err := coCreateInstance(&clsidMMDeviceEnumerator, &iidMMDeviceEnumerator, &enumerator); err != nil {
		return nil, fmt.Errorf("创建音频设备枚举器失败: %w", err)
	}
	defer enumerator.release()

	var device *comObject
	if _, err := enumerator.call(4, eRender, eConsole, uintptr(unsafe.Pointer(&device))); err != nil {
		return nil, fmt.Errorf("获取默认播放设备失败: %w", err)
	}
	if device == nil {
		return nil, errors.New("没有可用的播放设备")
	}
	defer device.release()

	var audioClient *comObject
	if _, err := device.call(3, uintptr(unsafe.Pointer(&iidAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&audioClient))); err != nil {
		return nil, fmt.Errorf("打开音频客户端失败: %w", err)
	}
	if audioClient == nil {
		return nil, errors.New("音频客户端为空")
	}
	defer audioClient.release()

	var format *waveFormatEx
	if _, err := audioClient.call(8, uintptr(unsafe.Pointer(&format))); err != nil {
		return nil, fmt.Errorf("读取混音格式失败: %w", err)
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(format)))

	layout, err := layoutOf(format)
	if err != nil {
		return nil, err
	}
	if os.Getenv("AUDIOCAP_DEBUG") != "" {
		fmt.Printf("[audiocap] tag=%d ch=%d rate=%d bits=%d float=%v blockAlign=%d\n",
			format.FormatTag, layout.channels, layout.rate, layout.bits, layout.isFloat, format.BlockAlign)
	}

	// 共享模式 + 环回；缓冲 1 秒（REFERENCE_TIME 单位 100ns）
	const bufferDuration = 10_000_000
	if _, err := audioClient.call(3, audclntShareModeShared, audclntStreamFlagsLoopback,
		uintptr(bufferDuration), 0, uintptr(unsafe.Pointer(format)), 0); err != nil {
		return nil, fmt.Errorf("初始化环回采集失败: %w", err)
	}

	var capture *comObject
	if _, err := audioClient.call(14, uintptr(unsafe.Pointer(&iidAudioCaptureClient)), uintptr(unsafe.Pointer(&capture))); err != nil {
		return nil, fmt.Errorf("获取采集接口失败: %w", err)
	}
	if capture == nil {
		return nil, errors.New("采集接口为空")
	}
	defer capture.release()

	if _, err := audioClient.call(10); err != nil {
		return nil, fmt.Errorf("启动采集失败: %w", err)
	}
	defer audioClient.call(11) // Stop

	mono := make([]int16, 0, layout.rate*seconds+layout.rate)
	started := time.Now()
	deadline := started.Add(time.Duration(seconds) * time.Second)
	frameBytes := layout.bytesPerSm * layout.channels

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// 一次把当前可读的包全部取完：每包约 10ms，若每包都 sleep 10ms 会跟不上，
		// 采集缓冲溢出丢帧 → 音频被"加速"（表现为音高整体偏高、识曲失败）。
		readAny := false
		for {
			var packet uint32
			if _, err := capture.call(5, uintptr(unsafe.Pointer(&packet))); err != nil {
				break
			}
			if packet == 0 {
				break
			}
			var (
				data   *byte
				frames uint32
				flags  uint32
				devPos uint64
				qpcPos uint64
			)
			if _, err := capture.call(3,
				uintptr(unsafe.Pointer(&data)),
				uintptr(unsafe.Pointer(&frames)),
				uintptr(unsafe.Pointer(&flags)),
				uintptr(unsafe.Pointer(&devPos)),
				uintptr(unsafe.Pointer(&qpcPos)),
			); err != nil {
				break
			}
			if frames > 0 {
				if flags&audclntBufferFlagsSilent != 0 || data == nil {
					for i := uint32(0); i < frames; i++ {
						mono = append(mono, 0)
					}
				} else {
					raw := unsafe.Slice(data, int(frames)*frameBytes)
					appendFrames(&mono, raw, layout)
				}
				readAny = true
			}
			_, _ = capture.call(4, uintptr(frames)) // ReleaseBuffer
		}
		if !readAny {
			time.Sleep(5 * time.Millisecond)
		}
	}

	if len(mono) == 0 {
		return nil, errors.New("没有采集到系统声音（播放设备是否静音？）")
	}
	if os.Getenv("AUDIOCAP_DEBUG") != "" {
		elapsed := time.Since(started).Seconds()
		fmt.Printf("[audiocap] frames=%d elapsed=%.2fs effectiveRate=%.0f\n", len(mono), elapsed, float64(len(mono))/elapsed)
	}
	return recognize.ResampleMono(mono, layout.rate, recognize.SampleRate), nil
}

// appendFrames 把交织的多声道样本混成单声道 int16。
func appendFrames(dst *[]int16, raw []byte, l sampleLayout) {
	n := len(raw) / (l.bytesPerSm * l.channels)
	for i := 0; i < n; i++ {
		var sum float64
		for c := 0; c < l.channels; c++ {
			off := (i*l.channels + c) * l.bytesPerSm
			sum += sampleAt(raw[off:], l)
		}
		v := sum / float64(l.channels)
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		*dst = append(*dst, int16(math.Round(v*32767)))
	}
}

func sampleAt(b []byte, l sampleLayout) float64 {
	switch {
	case l.isFloat:
		bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		return float64(math.Float32frombits(bits))
	case l.bits == 16:
		return float64(int16(uint16(b[0])|uint16(b[1])<<8)) / 32768
	case l.bits == 24:
		v := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16
		if v&0x800000 != 0 {
			v |= ^0xFFFFFF
		}
		return float64(v) / 8388608
	case l.bits == 32:
		bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		return float64(int32(bits)) / 2147483648
	}
	return 0
}

func coCreateInstance(clsid, iid *guid, out **comObject) error {
	var ptr unsafe.Pointer
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&ptr)),
	)
	if int32(hr) < 0 {
		return fmt.Errorf("CoCreateInstance 失败 (0x%08X)", uint32(hr))
	}
	if ptr == nil {
		return errors.New("CoCreateInstance 返回空指针")
	}
	*out = (*comObject)(ptr)
	return nil
}
