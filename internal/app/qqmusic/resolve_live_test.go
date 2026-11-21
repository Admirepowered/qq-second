package qqmusic

import (
	"os"
	"testing"
)

func TestLiveResolveDirect(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("live test")
	}
	c := NewClient()
	// 儿歌多多版本（live 测试曾解析成功的类型）
	r, err := c.Search("song", "小星星 儿歌多多", 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for i, s := range r.Songs {
		src, err := c.ResolvePlay(s.Mid, s.MediaMid, s.AlbumMid, "standard")
		if err == nil {
			t.Logf("#%d %s - %v q=%s f=%s", i, s.Title, artistNames(s), src.Quality, src.Format)
			return
		}
		t.Logf("#%d %s - %v err=%v", i, s.Title, artistNames(s), err)
	}
	t.Fatalf("all failed")
}
