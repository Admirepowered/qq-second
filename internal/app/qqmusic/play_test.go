package qqmusic

import "testing"

// 验证 midurlinfo 多候选遍历：高音质无权限(104003)时落到低音质可用项。
func TestPickPlayableItem(t *testing.T) {
	// 模拟: M800 无权限, M500 可用
	fakeData := map[string]interface{}{
		"midurlinfo": []interface{}{
			map[string]interface{}{"filename": "M800abc.mp3", "result": float64(104003), "purl": ""},
			map[string]interface{}{"filename": "M500abc.mp3", "result": float64(0), "purl": "aqqmusic.tc.qq.com/amobile.music.tc.qq.com/M500abc.mp3?guid=x&vkey=y"},
		},
	}
	items := arrayAt(fakeData, "midurlinfo")
	candidates := []fileKind{{"M800", ".mp3", "high", "mp3"}, {"M500", ".mp3", "standard", "mp3"}}

	var found *ResolvedSource
	for _, it := range items {
		entry, _ := it.(map[string]interface{})
		if entry == nil {
			continue
		}
		if result, ok := numberAt(entry, "result"); ok && result != 0 {
			continue
		}
		purl := stringAt(entry, "purl")
		if purl == "" {
			continue
		}
		full := normalizeCDN(purl)
		if full == "" {
			continue
		}
		filename := stringAt(entry, "filename")
		for _, k := range candidates {
			if len(filename) >= len(k.prefix) && filename[:len(k.prefix)] == k.prefix {
				found = &ResolvedSource{URL: full, Quality: k.quality, Format: k.format}
				break
			}
		}
		if found != nil {
			break
		}
	}
	if found == nil {
		t.Fatalf("should find M500 playable")
	}
	if found.Quality != "standard" {
		t.Fatalf("quality = %s, want standard", found.Quality)
	}
	if found.URL == "" {
		t.Fatalf("url empty")
	}
	t.Logf("fallback OK: %s -> %s", found.Quality, found.URL[:50])
}
