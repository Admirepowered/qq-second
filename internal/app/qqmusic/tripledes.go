package qqmusic

// QRC 歌词解密所用的自定义 Triple-DES 变体（ECB、8 字节分块、自定义 PC-2 偏移）。
// 按协议行为独立移植，与 qm-api-rs tripledes.rs 等价。

var sbox = [8][64]byte{
	{14, 4, 13, 1, 2, 15, 11, 8, 3, 10, 6, 12, 5, 9, 0, 7, 0, 15, 7, 4, 14, 2, 13, 1, 10, 6, 12, 11, 9, 5, 3, 8, 4, 1, 14, 8, 13, 6, 2, 11, 15, 12, 9, 7, 3, 10, 5, 0, 15, 12, 8, 2, 4, 9, 1, 7, 5, 11, 3, 14, 10, 0, 6, 13},
	{15, 1, 8, 14, 6, 11, 3, 4, 9, 7, 2, 13, 12, 0, 5, 10, 3, 13, 4, 7, 15, 2, 8, 15, 12, 0, 1, 10, 6, 9, 11, 5, 0, 14, 7, 11, 10, 4, 13, 1, 5, 8, 12, 6, 9, 3, 2, 15, 13, 8, 10, 1, 3, 15, 4, 2, 11, 6, 7, 12, 0, 5, 14, 9},
	{10, 0, 9, 14, 6, 3, 15, 5, 1, 13, 12, 7, 11, 4, 2, 8, 13, 7, 0, 9, 3, 4, 6, 10, 2, 8, 5, 14, 12, 11, 15, 1, 13, 6, 4, 9, 8, 15, 3, 0, 11, 1, 2, 12, 5, 10, 14, 7, 1, 10, 13, 0, 6, 9, 8, 7, 4, 15, 14, 3, 11, 5, 2, 12},
	{7, 13, 14, 3, 0, 6, 9, 10, 1, 2, 8, 5, 11, 12, 4, 15, 13, 8, 11, 5, 6, 15, 0, 3, 4, 7, 2, 12, 1, 10, 14, 9, 10, 6, 9, 0, 12, 11, 7, 13, 15, 1, 3, 14, 5, 2, 8, 4, 3, 15, 0, 6, 10, 10, 13, 8, 9, 4, 5, 11, 12, 7, 2, 14},
	{2, 12, 4, 1, 7, 10, 11, 6, 8, 5, 3, 15, 13, 0, 14, 9, 14, 11, 2, 12, 4, 7, 13, 1, 5, 0, 15, 10, 3, 9, 8, 6, 4, 2, 1, 11, 10, 13, 7, 8, 15, 9, 12, 5, 6, 3, 0, 14, 11, 8, 12, 7, 1, 14, 2, 13, 6, 15, 0, 9, 10, 4, 5, 3},
	{12, 1, 10, 15, 9, 2, 6, 8, 0, 13, 3, 4, 14, 7, 5, 11, 10, 15, 4, 2, 7, 12, 9, 5, 6, 1, 13, 14, 0, 11, 3, 8, 9, 14, 15, 5, 2, 8, 12, 3, 7, 0, 4, 10, 1, 13, 11, 6, 4, 3, 2, 12, 9, 5, 15, 10, 11, 14, 1, 7, 6, 0, 8, 13},
	{4, 11, 2, 14, 15, 0, 8, 13, 3, 12, 9, 7, 5, 10, 6, 1, 13, 0, 11, 7, 4, 9, 1, 10, 14, 3, 5, 12, 2, 15, 8, 6, 1, 4, 11, 13, 12, 3, 7, 14, 10, 15, 6, 8, 0, 5, 9, 2, 6, 11, 13, 8, 1, 4, 10, 7, 9, 5, 0, 15, 14, 2, 3, 12},
	{13, 2, 8, 4, 6, 15, 11, 1, 10, 9, 3, 14, 5, 0, 12, 7, 1, 15, 13, 8, 10, 3, 7, 4, 12, 5, 6, 11, 0, 14, 9, 2, 7, 11, 4, 1, 9, 12, 14, 2, 0, 6, 10, 13, 15, 3, 5, 8, 2, 1, 14, 7, 4, 10, 8, 13, 15, 12, 9, 0, 3, 5, 6, 11},
}

func sboxBit(a uint32) uint32 {
	return (a & 32) | ((a & 31) >> 1) | ((a & 1) << 4)
}

func bit(v uint32, b uint32) uint32 {
	return (v >> b) & 1
}

func initialPermutation(input []byte) (uint32, uint32) {
	v0 := uint32(input[0]) | uint32(input[1])<<8 | uint32(input[2])<<16 | uint32(input[3])<<24
	v1 := uint32(input[4]) | uint32(input[5])<<8 | uint32(input[6])<<16 | uint32(input[7])<<24

	s0 := (bit(v1, 6) << 31) | (bit(v1, 14) << 30) | (bit(v1, 22) << 29) | (bit(v1, 30) << 28) |
		(bit(v0, 6) << 27) | (bit(v0, 14) << 26) | (bit(v0, 22) << 25) | (bit(v0, 30) << 24) |
		(bit(v1, 4) << 23) | (bit(v1, 12) << 22) | (bit(v1, 20) << 21) | (bit(v1, 28) << 20) |
		(bit(v0, 4) << 19) | (bit(v0, 12) << 18) | (bit(v0, 20) << 17) | (bit(v0, 28) << 16) |
		(bit(v1, 2) << 15) | (bit(v1, 10) << 14) | (bit(v1, 18) << 13) | (bit(v1, 26) << 12) |
		(bit(v0, 2) << 11) | (bit(v0, 10) << 10) | (bit(v0, 18) << 9) | (bit(v0, 26) << 8) |
		(bit(v1, 0) << 7) | (bit(v1, 8) << 6) | (bit(v1, 16) << 5) | (bit(v1, 24) << 4) |
		(bit(v0, 0) << 3) | (bit(v0, 8) << 2) | (bit(v0, 16) << 1) | bit(v0, 24)
	s1 := (bit(v1, 7) << 31) | (bit(v1, 15) << 30) | (bit(v1, 23) << 29) | (bit(v1, 31) << 28) |
		(bit(v0, 7) << 27) | (bit(v0, 15) << 26) | (bit(v0, 23) << 25) | (bit(v0, 31) << 24) |
		(bit(v1, 5) << 23) | (bit(v1, 13) << 22) | (bit(v1, 21) << 21) | (bit(v1, 29) << 20) |
		(bit(v0, 5) << 19) | (bit(v0, 13) << 18) | (bit(v0, 21) << 17) | (bit(v0, 29) << 16) |
		(bit(v1, 3) << 15) | (bit(v1, 11) << 14) | (bit(v1, 19) << 13) | (bit(v1, 27) << 12) |
		(bit(v0, 3) << 11) | (bit(v0, 11) << 10) | (bit(v0, 19) << 9) | (bit(v0, 27) << 8) |
		(bit(v1, 1) << 7) | (bit(v1, 9) << 6) | (bit(v1, 17) << 5) | (bit(v1, 25) << 4) |
		(bit(v0, 1) << 3) | (bit(v0, 9) << 2) | (bit(v0, 17) << 1) | bit(v0, 25)
	return s0, s1
}

func inversePermutation(s0, s1 uint32) [8]byte {
	var data [8]byte
	data[3] = byte((bit(s1, 24) << 7) | (bit(s0, 24) << 6) | (bit(s1, 16) << 5) | (bit(s0, 16) << 4) | (bit(s1, 8) << 3) | (bit(s0, 8) << 2) | (bit(s1, 0) << 1) | bit(s0, 0))
	data[2] = byte((bit(s1, 25) << 7) | (bit(s0, 25) << 6) | (bit(s1, 17) << 5) | (bit(s0, 17) << 4) | (bit(s1, 9) << 3) | (bit(s0, 9) << 2) | (bit(s1, 1) << 1) | bit(s0, 1))
	data[1] = byte((bit(s1, 26) << 7) | (bit(s0, 26) << 6) | (bit(s1, 18) << 5) | (bit(s0, 18) << 4) | (bit(s1, 10) << 3) | (bit(s0, 10) << 2) | (bit(s1, 2) << 1) | bit(s0, 2))
	data[0] = byte((bit(s1, 27) << 7) | (bit(s0, 27) << 6) | (bit(s1, 19) << 5) | (bit(s0, 19) << 4) | (bit(s1, 11) << 3) | (bit(s0, 11) << 2) | (bit(s1, 3) << 1) | bit(s0, 3))
	data[7] = byte((bit(s1, 28) << 7) | (bit(s0, 28) << 6) | (bit(s1, 20) << 5) | (bit(s0, 20) << 4) | (bit(s1, 12) << 3) | (bit(s0, 12) << 2) | (bit(s1, 4) << 1) | bit(s0, 4))
	data[6] = byte((bit(s1, 29) << 7) | (bit(s0, 29) << 6) | (bit(s1, 21) << 5) | (bit(s0, 21) << 4) | (bit(s1, 13) << 3) | (bit(s0, 13) << 2) | (bit(s1, 5) << 1) | bit(s0, 5))
	data[5] = byte((bit(s1, 30) << 7) | (bit(s0, 30) << 6) | (bit(s1, 22) << 5) | (bit(s0, 22) << 4) | (bit(s1, 14) << 3) | (bit(s0, 14) << 2) | (bit(s1, 6) << 1) | bit(s0, 6))
	data[4] = byte((bit(s1, 31) << 7) | (bit(s0, 31) << 6) | (bit(s1, 23) << 5) | (bit(s0, 23) << 4) | (bit(s1, 15) << 3) | (bit(s0, 15) << 2) | (bit(s1, 7) << 1) | bit(s0, 7))
	return data
}

func desF(state uint32, key *[6]byte) uint32 {
	t1 := ((state & 1) << 31) | ((state & 0xF8000000) >> 1) | ((state & 0x1F800000) >> 3) | ((state & 0x01F80000) >> 5) | ((state & 0x001F8000) >> 7)
	t2 := ((state & 0x0001F800) << 15) | ((state & 0x00001F80) << 13) | ((state & 0x000001F8) << 11) | ((state & 0x0000001F) << 9) | ((state & 0x80000000) >> 23)

	k0 := byte((t1>>24)&0xFF) ^ key[0]
	k1 := byte((t1>>16)&0xFF) ^ key[1]
	k2 := byte((t1>>8)&0xFF) ^ key[2]
	k3 := byte((t2>>24)&0xFF) ^ key[3]
	k4 := byte((t2>>16)&0xFF) ^ key[4]
	k5 := byte((t2>>8)&0xFF) ^ key[5]

	state2 := uint32(sbox[0][sboxBit(uint32(k0)>>2)]) << 28 |
		uint32(sbox[1][sboxBit(uint32(k0&0x03)<<4|uint32(k1>>4))]) << 24 |
		uint32(sbox[2][sboxBit(uint32(k1&0x0F)<<2|uint32(k2>>6))]) << 20 |
		uint32(sbox[3][sboxBit(uint32(k2&0x3F))]) << 16 |
		uint32(sbox[4][sboxBit(uint32(k3)>>2)]) << 12 |
		uint32(sbox[5][sboxBit(uint32(k3&0x03)<<4|uint32(k4>>4))]) << 8 |
		uint32(sbox[6][sboxBit(uint32(k4&0x0F)<<2|uint32(k5>>6))]) << 4 |
		uint32(sbox[7][sboxBit(uint32(k5&0x3F))])

	return (bit(state2, 16) << 31) | (bit(state2, 25) << 30) | (bit(state2, 12) << 29) | (bit(state2, 11) << 28) |
		(bit(state2, 3) << 27) | (bit(state2, 20) << 26) | (bit(state2, 4) << 25) | (bit(state2, 15) << 24) |
		(bit(state2, 31) << 23) | (bit(state2, 17) << 22) | (bit(state2, 9) << 21) | (bit(state2, 6) << 20) |
		(bit(state2, 27) << 19) | (bit(state2, 14) << 18) | (bit(state2, 1) << 17) | (bit(state2, 22) << 16) |
		(bit(state2, 30) << 15) | (bit(state2, 24) << 14) | (bit(state2, 8) << 13) | (bit(state2, 18) << 12) |
		(bit(state2, 0) << 11) | (bit(state2, 5) << 10) | (bit(state2, 29) << 9) | (bit(state2, 23) << 8) |
		(bit(state2, 13) << 7) | (bit(state2, 19) << 6) | (bit(state2, 2) << 5) | (bit(state2, 26) << 4) |
		(bit(state2, 10) << 3) | (bit(state2, 21) << 2) | (bit(state2, 28) << 1) | bit(state2, 7)
}

func desCrypt(input []byte, key *[16][6]byte) [8]byte {
	s0, s1 := initialPermutation(input)
	for i := 0; i < 15; i++ {
		prevS1 := s1
		s1 = desF(s1, &key[i]) ^ s0
		s0 = prevS1
	}
	s0 ^= desF(s1, &key[15])
	return inversePermutation(s0, s1)
}


var keyRndShift = [16]uint32{1, 1, 2, 2, 2, 2, 2, 2, 1, 2, 2, 2, 2, 2, 2, 1}
var keyPermC = [28]uint32{56, 48, 40, 32, 24, 16, 8, 0, 57, 49, 41, 33, 25, 17, 9, 1, 58, 50, 42, 34, 26, 18, 10, 2, 59, 51, 43, 35}
var keyPermD = [28]uint32{62, 54, 46, 38, 30, 22, 14, 6, 61, 53, 45, 37, 29, 21, 13, 5, 60, 52, 44, 36, 28, 20, 12, 4, 27, 19, 11, 3}
var keyCompression = [48]uint32{13, 16, 10, 23, 0, 4, 2, 27, 14, 5, 20, 9, 22, 18, 11, 3, 25, 7, 15, 6, 26, 19, 12, 1, 40, 51, 30, 36, 46, 54, 29, 39, 50, 44, 32, 47, 43, 48, 38, 55, 33, 52, 45, 41, 49, 35, 28, 31}

const (
	desEncrypt = 1
	desDecrypt = 0
)

func keySchedule(key []byte, mode int32) [16][6]byte {
	var schedule [16][6]byte
	v0 := uint32(key[0]) | uint32(key[1])<<8 | uint32(key[2])<<16 | uint32(key[3])<<24
	v1 := uint32(key[4]) | uint32(key[5])<<8 | uint32(key[6])<<16 | uint32(key[7])<<24

	var c, d uint32
	for i, b := range keyPermC {
		var bitVal uint32
		if b < 32 {
			bitVal = (v0 >> (31 - b)) & 1
		} else {
			bitVal = (v1 >> (63 - b)) & 1
		}
		c |= bitVal << (31 - uint32(i))
	}
	for i, b := range keyPermD {
		var bitVal uint32
		if b < 32 {
			bitVal = (v0 >> (31 - b)) & 1
		} else {
			bitVal = (v1 >> (63 - b)) & 1
		}
		d |= bitVal << (31 - uint32(i))
	}

	for i, shift := range keyRndShift {
		c = ((c << shift) | (c >> (28 - shift))) & 0xFFFFFFF0
		d = ((d << shift) | (d >> (28 - shift))) & 0xFFFFFFF0

		togen := i
		if mode == desDecrypt {
			togen = 15 - i
		}

		for j := 0; j < 24; j++ {
			bitVal := (c >> (31 - keyCompression[j])) & 1
			schedule[togen][j/8] |= byte(bitVal) << byte(7-(j%8))
		}
		for j := 24; j < 48; j++ {
			bitVal := (d >> (31 - (keyCompression[j] - 27))) & 1
			schedule[togen][j/8] |= byte(bitVal) << byte(7-(j%8))
		}
	}
	return schedule
}

func tripledesKeySetup(key []byte, mode int32) [3][16][6]byte {
	if mode == desEncrypt {
		return [3][16][6]byte{
			keySchedule(key[0:8], desEncrypt),
			keySchedule(key[8:16], desDecrypt),
			keySchedule(key[16:24], desEncrypt),
		}
	}
	return [3][16][6]byte{
		keySchedule(key[16:24], desDecrypt),
		keySchedule(key[8:16], desEncrypt),
		keySchedule(key[0:8], desDecrypt),
	}
}

func tripledesCrypt(data []byte, key *[3][16][6]byte) [8]byte {
	var out [8]byte
	copy(out[:], data)
	for i := range key {
		out = desCrypt(out[:], &key[i])
	}
	return out
}
