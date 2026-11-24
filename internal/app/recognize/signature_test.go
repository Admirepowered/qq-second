package recognize

import (
	"encoding/binary"
	"hash/crc32"
	"math"
	"testing"
)

// 生成一段确定性的测试音频（多个正弦叠加 + 轻微噪声）。
func synth(seconds float64) []int16 {
	n := int(float64(SampleRate) * seconds)
	out := make([]int16, n)
	seed := uint32(12345)
	next := func() float64 {
		seed = seed*1664525 + 1013904223
		return float64(seed>>8)/float64(1<<24) - 0.5
	}
	freqs := []float64{220, 440, 660, 880, 1320, 1760, 2640}
	for i := 0; i < n; i++ {
		t := float64(i) / SampleRate
		v := 0.0
		for _, f := range freqs {
			v += math.Sin(2*math.Pi*f*t) / float64(len(freqs))
		}
		v += next() * 0.02
		out[i] = int16(v * 12000)
	}
	return out
}

func TestGenerateSignatureStructure(t *testing.T) {
	sig, samples, err := GenerateSignature(synth(6))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if samples == 0 || samples > 6*SampleRate {
		t.Fatalf("samples = %d", samples)
	}
	if len(sig) < 56 {
		t.Fatalf("signature too short: %d", len(sig))
	}
	if got := binary.LittleEndian.Uint32(sig[0:]); got != magic1 {
		t.Fatalf("magic1 = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(sig[12:]); got != magic2 {
		t.Fatalf("magic2 = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(sig[8:]); got != uint32(len(sig)-48) {
		t.Fatalf("size_minus_header = %d, len-48 = %d", got, len(sig)-48)
	}
	if got := crc32.ChecksumIEEE(sig[8:]); got != binary.LittleEndian.Uint32(sig[4:]) {
		t.Fatalf("crc32 mismatch")
	}
	if got := binary.LittleEndian.Uint32(sig[28:]); got != sampleRateID<<27 {
		t.Fatalf("shifted_sample_rate_id = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(sig[48:]); got != 0x40000000 {
		t.Fatalf("chunk marker = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(sig[52:]); got != uint32(len(sig)-48) {
		t.Fatalf("chunk size = %d", got)
	}

	// 解析 TLV：每个频段头必须是 bandMagic+band，峰值为 5 字节记录
	pos := 56
	bands := 0
	for pos+8 <= len(sig) {
		id := binary.LittleEndian.Uint32(sig[pos:])
		size := int(binary.LittleEndian.Uint32(sig[pos+4:]))
		if id < bandMagic || id > bandMagic+3 {
			t.Fatalf("bad band id %#x at %d", id, pos)
		}
		if size%5 != 0 {
			t.Fatalf("band size %d not multiple of 5", size)
		}
		pos += 8 + size
		pos += (4 - size%4) % 4
		bands++
	}
	if pos != len(sig) {
		t.Fatalf("TLV parse ended at %d, len %d", pos, len(sig))
	}
	if bands == 0 {
		t.Fatal("no bands in signature")
	}
}

func TestGenerateSignatureTooShort(t *testing.T) {
	if _, _, err := GenerateSignature([]int16{1, 2, 3}); err != ErrTooShort {
		t.Fatalf("want ErrTooShort, got %v", err)
	}
}

func TestResampleMono(t *testing.T) {
	in := make([]int16, 48000)
	for i := range in {
		in[i] = int16(math.Sin(2*math.Pi*440*float64(i)/48000) * 10000)
	}
	out := ResampleMono(in, 48000, 16000)
	if len(out) != 16000 {
		t.Fatalf("len = %d, want 16000", len(out))
	}
	if out == nil || len(out) == 0 {
		t.Fatal("empty output")
	}
}

func TestDecodePCM16(t *testing.T) {
	// 0x0001, 0xffff（-1）的小端编码
	pcm, err := DecodePCM16("AQD//w==")
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) != 2 || pcm[0] != 1 || pcm[1] != -1 {
		t.Fatalf("decoded = %v", pcm)
	}
}
