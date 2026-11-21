package qqmusic

// 播放地址解析：CgiGetVkey 明文流（不涉及任何加密媒体格式）。

import (
	"fmt"
	"net/url"
	"strings"
)

// ResolvedSource 是一次成功解析的播放源。
type ResolvedSource struct {
	URL       string
	Quality   string
	Format    string
	SizeBytes int64
}

// 文件类型候选：quality → 文件名前缀 + 扩展名。
// 仅使用明文（未加密）格式：mp3 / m4a / flac。
type fileKind struct {
	prefix string
	ext    string
	quality string
	format  string
}

var clearKinds = []fileKind{
	{"RS01", ".flac", "lossless", "flac"},  // hi-res flac
	{"F000", ".flac", "lossless", "flac"},  // flac
	{"M800", ".mp3", "high", "mp3"},        // 320k mp3
	{"M500", ".mp3", "standard", "mp3"},    // 128k mp3
	{"C400", ".m4a", "standard", "m4a"},    // 96k aac
}

// qualityCandidates 按偏好音质生成降级候选链。
func qualityCandidates(preferred string) []fileKind {
	switch preferred {
	case "lossless":
		return clearKinds
	case "high":
		return clearKinds[2:]
	case "standard":
		return clearKinds[3:]
	default:
		return clearKinds[2:]
	}
}

// ResolvePlay 解析一首歌的播放 URL（降级到第一个可用的候选）。
func (c *Client) ResolvePlay(songMid, mediaMid, albumMid, preferredQuality string) (*ResolvedSource, error) {
	if mediaMid == "" {
		mediaMid = songMid
	}
	candidates := qualityCandidates(preferredQuality)

	var filenames []string
	for _, k := range candidates {
		filenames = append(filenames, k.prefix+mediaMid+k.ext)
	}

	param := map[string]interface{}{
		"uin":      strconvOr0(c.Credential().StrMusicID),
		"filename": filenames,
		"guid":     getGUID(),
		"songmid":  []interface{}{songMid},
		"songtype": []interface{}{0},
		"ctx":      0,
	}
	data, err := c.cgiAndroid("music.vkey.GetVkey", "UrlGetVkey", param)
	if err != nil {
		return nil, err
	}

	items := arrayAt(data, "midurlinfo")
	if len(items) == 0 {
		return nil, fmt.Errorf("no midurlinfo")
	}
	entry, _ := items[0].(map[string]interface{})
	if entry == nil {
		return nil, fmt.Errorf("bad midurlinfo")
	}
	if purl := stringAt(entry, "purl"); purl != "" {
		full := normalizeCDN(purl)
		if full == "" {
			return nil, fmt.Errorf("playback URL host not allowed")
		}
		filename := stringAt(entry, "filename")
		for _, k := range candidates {
			if strings.HasPrefix(filename, k.prefix) && strings.HasSuffix(filename, k.ext) {
				return &ResolvedSource{URL: full, Quality: k.quality, Format: k.format}, nil
			}
		}
		return &ResolvedSource{URL: full, Quality: "standard", Format: "mp3"}, nil
	}
	// purl 为空：可能需要登录或无权限
	if code, ok := numberAt(entry, "result"); ok {
		return nil, fmt.Errorf("resolve failed result %d", code)
	}
	return nil, fmt.Errorf("no playable url (may require login)")
}

// normalizeCDN 校验并归一化 CDN 地址（仅允许 QQ 音乐 https CDN）。
func normalizeCDN(purl string) string {
	raw := purl
	if !strings.HasPrefix(raw, "http") {
		raw = "https://isure.stream.qqmusic.qq.com/" + strings.TrimPrefix(raw, "/")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return ""
	}
	host := u.Hostname()
	if !(strings.HasSuffix(host, ".qqmusic.qq.com") || strings.HasSuffix(host, ".tc.qq.com")) {
		return ""
	}
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	return u.String()
}

func strconvOr0(s string) string {
	if s == "" {
		return "0"
	}
	return s
}
