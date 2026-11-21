package qqmusic

import (
	"os"
	"testing"
)

func TestLiveGuestFlow(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("set QQ_LIVE=1 to run live test")
	}
	c := NewClient()

	result, err := c.Search("song", "晴天", 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Songs) == 0 {
		t.Fatalf("search returned 0 songs")
	}
	t.Logf("song: %s - %s (mid=%s mediaMid=%s albumMid=%s cap=%s)", result.Songs[0].Title, result.Songs[0].Artists[0].Name, result.Songs[0].Mid, result.Songs[0].MediaMid, result.Songs[0].AlbumMid, result.Songs[0].Capability)

	// 找一首免费歌（搜"网络热歌"通常混有免费曲目）
	for _, q := range []string{"小星星", "两只老虎 群星", "自由飞翔 凤凰传奇"} {
		r, err := c.Search("song", q, 1)
		if err != nil || len(r.Songs) == 0 {
			continue
		}
		for _, s := range r.Songs {
			src, err := c.ResolvePlay(s.Mid, s.MediaMid, s.AlbumMid, "standard")
			if err == nil {
				t.Logf("resolved %q - %s: quality=%s format=%s", s.Title, artistNames(s), src.Quality, src.Format)
				doc, err := c.GetLyrics(s.Mid)
				if err != nil {
					t.Fatalf("lyrics: %v", err)
				}
				t.Logf("lyrics: mode=%s lines=%d", doc.SyncMode, len(doc.Lines))
				if len(doc.Lines) > 0 {
					l := doc.Lines[0]
					t.Logf("line0: [%d-%d] %q words=%d trans=%q", l.StartMs, l.EndMs, l.Text, len(l.Words), l.Translation)
				}
				return
			}
		}
	}
	t.Fatalf("no free song resolved")
}

func artistNames(s Track) string {
	out := ""
	for _, a := range s.Artists {
		if out != "" {
			out += "/"
		}
		out += a.Name
	}
	return out
}
