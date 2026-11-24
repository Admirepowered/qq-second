// Package recognize 实现「听歌识曲」：把 16kHz 单声道 PCM 转成 Shazam 指纹签名，
// 调用公开的 Shazam 识别接口拿到曲目信息。
//
// 签名算法移植自公开实现（shazamio 的 SignatureGenerator / DecodedMessage，
// 与 songrec 的 Shazam 算法一致）：
//   - 2048 点 FFT，每 128 个采样推进一帧，Hann 窗
//   - 频谱峰值提取 + 时间/频率邻域扩散
//   - 按 250-520 / 520-1450 / 1450-3500 Hz 三个频段打包成带 48 字节头的消息
package recognize

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
)

const (
	// SampleRate 是签名算法要求的采样率。
	SampleRate = 16000

	fftSize    = 2048
	binCount   = fftSize/2 + 1
	hopSize    = 128
	fftRingLen = 256

	maxTimeSeconds = 3.1
	maxPeaks       = 255

	magic1       = 0xCAFE2580
	magic2       = 0x94119C00
	bandMagic    = 0x60030040
	sampleRateID = 3 // SampleRate._16000
	fixedValue   = (15 << 19) + 0x40000
)

// ErrTooShort 表示音频长度不足一帧（128 采样）。
var ErrTooShort = errors.New("音频太短，无法生成指纹")

// peak 是一个频谱峰值。
type peak struct {
	pass      int
	magnitude int
	freqBin   int
}

// Generator 累积采样并产出签名。
type Generator struct {
	window []float64

	ring    []float64
	ringPos int

	fftRing    [][]float64
	fftPos     int
	fftWritten int

	spread        [][]float64
	spreadPos     int
	spreadWritten int

	re, im []float64

	samples int
	peaks   [4][]peak
}

// NewGenerator 创建签名生成器。
func NewGenerator() *Generator {
	g := &Generator{
		window:  make([]float64, fftSize),
		ring:    make([]float64, fftSize),
		fftRing: make([][]float64, fftRingLen),
		spread:  make([][]float64, fftRingLen),
		re:      make([]float64, fftSize),
		im:      make([]float64, fftSize),
	}
	// np.hanning(2050)[1:-1]：两端各去掉一个零点
	for i := 0; i < fftSize; i++ {
		g.window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i+1)/float64(fftSize+1))
	}
	for i := 0; i < fftRingLen; i++ {
		g.fftRing[i] = make([]float64, binCount)
		g.spread[i] = make([]float64, binCount)
	}
	return g
}

// Feed 处理一段 16kHz 单声道采样；返回是否还需要继续喂数据。
func (g *Generator) Feed(pcm []int16) bool {
	processed := 0
	for len(pcm)-processed >= hopSize && g.needMore() {
		g.processChunk(pcm[processed : processed+hopSize])
		processed += hopSize
	}
	return g.needMore()
}

func (g *Generator) needMore() bool {
	if float64(g.samples)/SampleRate < maxTimeSeconds {
		return true
	}
	return g.totalPeaks() < maxPeaks
}

func (g *Generator) totalPeaks() int {
	n := 0
	for i := range g.peaks {
		n += len(g.peaks[i])
	}
	return n
}

func (g *Generator) processChunk(chunk []int16) {
	g.samples += len(chunk)

	// 写入环形缓冲
	for i, s := range chunk {
		g.ring[(g.ringPos+i)%fftSize] = float64(s)
	}
	g.ringPos = (g.ringPos + len(chunk)) % fftSize

	// 以当前写入位置为起点取一帧（最旧的采样），加 Hann 窗后做 FFT
	for i := 0; i < fftSize; i++ {
		g.re[i] = g.ring[(g.ringPos+i)%fftSize] * g.window[i]
		g.im[i] = 0
	}
	fft(g.re, g.im)

	// 功率谱（/2^17 与参考实现一致）
	out := g.fftRing[g.fftPos]
	for i := 0; i < binCount; i++ {
		v := (g.re[i]*g.re[i] + g.im[i]*g.im[i]) / float64(1<<17)
		if v < 1e-10 {
			v = 1e-10
		}
		out[i] = v
	}
	g.fftPos = (g.fftPos + 1) % fftRingLen
	g.fftWritten++

	g.spreadPeaks()
	if g.spreadWritten >= 46 {
		g.recognizePeaks()
	}
}

// spreadPeaks 在时间与频率两个方向做峰值扩散（取邻域最大值）。
func (g *Generator) spreadPeaks() {
	last := g.fftRing[(g.fftPos-1+fftRingLen)%fftRingLen]

	// 频率方向：每个 bin 取自身与右侧 1、2 个 bin 的最大值（末尾 3 个保持原值）
	origin := make([]float64, binCount)
	for j := 0; j < binCount-3; j++ {
		m := last[j]
		if last[j+1] > m {
			m = last[j+1]
		}
		if last[j+2] > m {
			m = last[j+2]
		}
		origin[j] = m
	}
	copy(origin[binCount-3:], last[binCount-3:])

	i1 := (g.spreadPos - 1 + fftRingLen) % fftRingLen
	i2 := (g.spreadPos - 3 + fftRingLen) % fftRingLen
	i3 := (g.spreadPos - 6 + fftRingLen) % fftRingLen

	t1 := maxInto(origin, g.spread[i1])
	t2 := maxInto(t1, g.spread[i2])
	t3 := maxInto(t2, g.spread[i3])

	copy(g.spread[i1], t1)
	copy(g.spread[i2], t2)
	copy(g.spread[i3], t3)

	copy(g.spread[g.spreadPos], origin)
	g.spreadPos = (g.spreadPos + 1) % fftRingLen
	g.spreadWritten++
}

func maxInto(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		if a[i] > b[i] {
			out[i] = a[i]
		} else {
			out[i] = b[i]
		}
	}
	return out
}

// recognizePeaks 在延迟 46 帧的谱上找峰值，并与邻域比较确认。
func (g *Generator) recognizePeaks() {
	cur := g.fftRing[(g.fftPos-46+fftRingLen*2)%fftRingLen]
	spread49 := g.spread[(g.spreadPos-49+fftRingLen*2)%fftRingLen]

	const minPower = 1.0 / 64
	freqOffsets := []int{-10, -7, -4, -3, 1, 2, 5, 8}
	timeOffsets := []int{-53, -45, 165, 172, 179, 186, 193, 214, 221, 228, 235, 242}

	for bin := 10; bin < 1015; bin++ {
		if cur[bin] < minPower || cur[bin] < spread49[bin-1] {
			continue
		}
		maxNeighbor := 0.0
		for _, off := range freqOffsets {
			if v := spread49[bin+off]; v > maxNeighbor {
				maxNeighbor = v
			}
		}
		if cur[bin] <= maxNeighbor {
			continue
		}
		maxOther := maxNeighbor
		for _, off := range timeOffsets {
			idx := (g.spreadPos + off + fftRingLen*2) % fftRingLen
			if v := g.spread[idx][bin-1]; v > maxOther {
				maxOther = v
			}
		}
		if cur[bin] <= maxOther {
			continue
		}

		pass := g.spreadWritten - 46
		mag := math.Log(math.Max(minPower, cur[bin]))*1477.3 + 6144
		before := math.Log(math.Max(minPower, cur[bin-1]))*1477.3 + 6144
		after := math.Log(math.Max(minPower, cur[bin+1]))*1477.3 + 6144
		variation1 := mag*2 - before - after
		if variation1 <= 0 {
			continue
		}
		variation2 := (after - before) * 32 / variation1
		corrected := float64(bin)*64 + variation2
		hz := corrected * (SampleRate / 2.0 / 1024 / 64)

		var band int
		switch {
		case hz > 250 && hz < 520:
			band = 0
		case hz > 520 && hz < 1450:
			band = 1
		case hz > 1450 && hz < 3500:
			band = 2
		default:
			// 3.5–5.5kHz 频段在 legacy 模式下不使用（与参考实现一致）
			continue
		}
		g.peaks[band] = append(g.peaks[band], peak{
			pass:      pass,
			magnitude: int(mag),
			freqBin:   int(corrected),
		})
	}
}

// Encode 把已提取的峰值打包成 Shazam 签名消息。
func (g *Generator) Encode() []byte {
	contents := make([]byte, 0, 2048)
	for band := 0; band < 4; band++ {
		peaks := g.peaks[band]
		if len(peaks) == 0 {
			continue
		}
		body := make([]byte, 0, len(peaks)*5)
		last := 0
		for _, p := range peaks {
			if p.pass-last >= 255 {
				body = append(body, 0xFF)
				var b [4]byte
				binary.LittleEndian.PutUint32(b[:], uint32(p.pass))
				body = append(body, b[:]...)
				last = p.pass
			}
			body = append(body, byte(p.pass-last))
			var mb [2]byte
			binary.LittleEndian.PutUint16(mb[:], uint16(p.magnitude))
			body = append(body, mb[:]...)
			var fb [2]byte
			binary.LittleEndian.PutUint16(fb[:], uint16(p.freqBin))
			body = append(body, fb[:]...)
			last = p.pass
		}

		var hdr [8]byte
		binary.LittleEndian.PutUint32(hdr[0:], uint32(bandMagic+band))
		binary.LittleEndian.PutUint32(hdr[4:], uint32(len(body)))
		contents = append(contents, hdr[:]...)
		contents = append(contents, body...)
		if pad := (4 - len(body)%4) % 4; pad > 0 {
			contents = append(contents, make([]byte, pad)...)
		}
	}

	sizeMinusHeader := len(contents) + 8
	out := make([]byte, 48+8+len(contents))
	binary.LittleEndian.PutUint32(out[0:], magic1)
	binary.LittleEndian.PutUint32(out[8:], uint32(sizeMinusHeader))
	binary.LittleEndian.PutUint32(out[12:], magic2)
	binary.LittleEndian.PutUint32(out[28:], uint32(sampleRateID<<27))
	binary.LittleEndian.PutUint32(out[40:], uint32(g.samples+int(SampleRate*0.24)))
	binary.LittleEndian.PutUint32(out[44:], uint32(fixedValue))
	binary.LittleEndian.PutUint32(out[48:], 0x40000000)
	binary.LittleEndian.PutUint32(out[52:], uint32(sizeMinusHeader))
	copy(out[56:], contents)
	binary.LittleEndian.PutUint32(out[4:], crc32.ChecksumIEEE(out[8:]))
	return out
}

// Samples 返回已处理的采样数。
func (g *Generator) Samples() int { return g.samples }

// Peaks 返回各频段峰值数量（调试/测试用）。
func (g *Generator) Peaks() [4]int {
	var out [4]int
	for i := range g.peaks {
		out[i] = len(g.peaks[i])
	}
	return out
}

// GenerateSignature 从 16kHz 单声道 PCM 生成签名消息，返回签名与已处理采样数。
func GenerateSignature(pcm []int16) ([]byte, int, error) {
	g := NewGenerator()
	if len(pcm) < hopSize {
		return nil, 0, ErrTooShort
	}
	g.Feed(pcm)
	if g.samples == 0 {
		return nil, 0, ErrTooShort
	}
	return g.Encode(), g.samples, nil
}

// ---- FFT（基 2 迭代 Cooley-Tukey，2048 点） ----

var (
	fftRev      []int
	fftCosTable []float64
	fftSinTable []float64
)

func init() {
	n := fftSize
	bits := 0
	for (1 << bits) < n {
		bits++
	}
	fftRev = make([]int, n)
	for i := 0; i < n; i++ {
		r := 0
		for b := 0; b < bits; b++ {
			r = (r << 1) | ((i >> b) & 1)
		}
		fftRev[i] = r
	}
	fftCosTable = make([]float64, n/2)
	fftSinTable = make([]float64, n/2)
	for i := 0; i < n/2; i++ {
		ang := -2 * math.Pi * float64(i) / float64(n)
		fftCosTable[i] = math.Cos(ang)
		fftSinTable[i] = math.Sin(ang)
	}
}

func fft(re, im []float64) {
	n := len(re)
	for i := 0; i < n; i++ {
		j := fftRev[i]
		if j > i {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		half := size / 2
		step := n / size
		for start := 0; start < n; start += size {
			for k := 0; k < half; k++ {
				wr := fftCosTable[k*step]
				wi := fftSinTable[k*step]
				i0 := start + k
				i1 := i0 + half
				tr := re[i1]*wr - im[i1]*wi
				ti := re[i1]*wi + im[i1]*wr
				re[i1] = re[i0] - tr
				im[i1] = im[i0] - ti
				re[i0] += tr
				im[i0] += ti
			}
		}
	}
}
