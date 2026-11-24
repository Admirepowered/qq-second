package qqmusic

// 详情页解析回归测试（真实链路）：首页推荐歌单、歌单/专辑/歌手详情。
// 这些接口的字段名随信封（Android/web）而变，历史 bug 都是"字段名读错 → 内容为空"。
//
// 注意：QQ 对新设备/IP 有频控（cgi 2001），所以这里刻意放慢节奏，并把频控导致
// 的失败判定为跳过（不是回归）。

import (
	"os"
	"strings"
	"testing"
	"time"
)

func livePause() { time.Sleep(700 * time.Millisecond) }

func isRateLimited(err error) bool {
	return err != nil && strings.Contains(err.Error(), "2001")
}

func TestLiveDetailParsing(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("set QQ_LIVE=1 to run live test")
	}
	c := NewClient()

	feed, err := c.Home()
	if isRateLimited(err) {
		t.Skip("频控中（cgi 2001），稍后再跑")
	}
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	if len(feed.Playlists) == 0 {
		t.Fatal("首页没有推荐歌单")
	}
	for i, p := range feed.Playlists {
		if i >= 5 {
			break
		}
		if p.Title == "" {
			t.Errorf("推荐歌单[%d] 没有标题", i)
		}
		if p.Cover == "" {
			t.Errorf("推荐歌单[%d] %q 没有封面", i, p.Title)
		}
	}
	livePause()

	detail, err := c.PlaylistDetail(feed.Playlists[0].ID, 0)
	if isRateLimited(err) {
		t.Skip("频控中（cgi 2001），稍后再跑")
	}
	if err != nil {
		t.Fatalf("playlist detail: %v", err)
	}
	if detail.Info.Title == "" || detail.Info.Cover == "" {
		t.Errorf("歌单详情缺少标题/封面: title=%q cover=%q", detail.Info.Title, detail.Info.Cover)
	}
	if len(detail.Tracks) == 0 {
		t.Error("歌单详情没有歌曲")
	}
	livePause()

	// 歌手详情
	ar, err := c.Search("artist", "周杰伦", 1)
	if isRateLimited(err) {
		t.Skip("频控中（cgi 2001），稍后再跑")
	}
	if err == nil && len(ar.Artists) == 0 {
		t.Error("歌手搜索为空（字段名可能又变了）")
	}
	if err == nil && len(ar.Artists) > 0 {
		livePause()
		ad, err := c.ArtistDetail(ar.Artists[0].ID)
		if err == nil {
			if ad.Name == "" {
				t.Error("歌手详情缺少名字")
			}
			if len(ad.Songs) == 0 {
				t.Error("歌手详情没有歌曲")
			}
			if len(ad.Albums) == 0 {
				t.Error("歌手详情没有专辑")
			}
		} else if !isRateLimited(err) {
			t.Errorf("歌手详情失败: %v", err)
		}
	}
	livePause()

	// 专辑详情（用搜索结果里第一个有 albumMid 的歌）
	songs, err := c.Search("song", "晴天", 1)
	if isRateLimited(err) {
		t.Skip("频控中（cgi 2001），稍后再跑")
	}
	if err == nil {
		for _, s := range songs.Songs {
			if s.AlbumMid == "" {
				continue
			}
			album, err := c.AlbumDetail(s.AlbumMid)
			if isRateLimited(err) {
				break
			}
			if err != nil {
				t.Errorf("专辑详情失败: %v", err)
				break
			}
			if album.Title == "" {
				t.Error("专辑详情缺少标题")
			}
			if len(album.Tracks) == 0 {
				t.Error("专辑详情没有歌曲")
			}
			break
		}
	}
}
