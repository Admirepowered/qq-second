package app

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
)

func randomRead(b []byte) (int, error) {
	return rand.Read(b)
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

func contentTypeByExt(ext string) string {
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".m4a", ".mp4", ".aac":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".ape", ".wv", ".aiff", ".aif":
		return "application/octet-stream"
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

var _ = http.StatusOK // 保持 net/http 引用（防误删）
var _ = io.EOF
