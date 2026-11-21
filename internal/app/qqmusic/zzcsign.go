package qqmusic

// zzc 请求签名（QQ 桌面客户端 musics.fcg 签名，SHA1 简化实现）。
// 参考公开协议行为实现，与 qm-api-rs sign.rs 等价。

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
)

var part1Indexes = []int{23, 14, 6, 36, 16, 7, 19}
var part2Indexes = []int{16, 1, 32, 12, 19, 27, 8, 5}
var scrambleValues = []byte{89, 39, 179, 150, 218, 82, 58, 252, 177, 52, 186, 123, 120, 64, 242, 133, 143, 161, 121, 179}

// zzcSign 计算形如 zzc... 的小写签名串。
func zzcSign(payload []byte) string {
	sum := sha1.Sum(payload)
	hashHex := strings.ToUpper(hex.EncodeToString(sum[:]))

	var b1, b2 strings.Builder
	for _, i := range part1Indexes {
		b1.WriteByte(hashHex[i])
	}
	for _, i := range part2Indexes {
		b2.WriteByte(hashHex[i])
	}

	part3 := make([]byte, 20)
	for i, v := range scrambleValues {
		b, err := strconv.ParseUint(hashHex[i*2:i*2+2], 16, 8)
		if err != nil {
			b = 0
		}
		part3[i] = v ^ byte(b)
	}
	b64 := base64.StdEncoding.EncodeToString(part3)
	b64 = strings.NewReplacer("/", "", "\\", "", "+", "", "=", "").Replace(b64)
	return strings.ToLower("zzc" + b1.String() + b64 + b2.String())
}

// hash33 计算 g_tk / ptqrtoken。
func hash33(s string, h int64) int64 {
	for _, c := range s {
		h = ((h << 5) + h + int64(c)) & 0x7fffffff
	}
	return h
}
