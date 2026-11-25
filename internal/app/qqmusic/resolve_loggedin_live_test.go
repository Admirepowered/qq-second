package qqmusic

// 登录态播放解析回归测试（真实链路）。
//
// 背景（历史 bug「解析播放源失败: cgi 1000」）：vkey 请求 filename 里只要
// 带一个账号无权的音质（M800 320k，普通账号/绿钻到期即无），服务端就把整组
// 请求全部拒绝（登录态 req_0.code=1000，midurlinfo 逐项也全变 104003），
// 本可用的 128k 也拿不到。用户 preferredQuality=high 时每首歌都带 M800
// → 大面积 cgi 1000 → 播放卡死。
//
// 修复：ResolvePlay 多级链重试——偏好链失败 → standard 链（M500/C400）再试。
// 本测试同时验证：①有权曲目正常出 URL；②无 320k 权益的曲目降到 standard
// 播放（而不是失败）；③真无权曲目（连 128k 都无权）报 104003（前端跳歌）。

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLiveResolvePlayLoggedIn(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("set QQ_LIVE=1 to run live test")
	}
	c := NewClient()
	raw, err := os.ReadFile(`C:\Users\admir\AppData\Roaming\qq-second\store.json`)
	if err != nil {
		t.Skip("no store.json on this machine")
	}
	var store struct {
		Credential struct {
			Uin        string `json:"uin"`
			QQMusicKey string `json:"qqMusicKey"`
		} `json:"credential"`
	}
	if err := json.Unmarshal(raw, &store); err != nil || store.Credential.QQMusicKey == "" {
		t.Skip("no credential on this machine")
	}
	c.SetCredential(Credential{
		StrMusicID: store.Credential.Uin,
		MusicID:    1204844044,
		MusicKey:   store.Credential.QQMusicKey,
		LoginType:  2,
	})

	// ① 免费曲（standard 链）：应成功并给出 URL
	res, err := c.Search("song", "两只老虎 群星", 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res.Songs) == 0 {
		t.Fatal("no songs")
	}
	s := res.Songs[0]
	src, err := c.ResolvePlay(s.Mid, s.MediaMid, s.AlbumMid, "standard")
	if err != nil {
		t.Fatalf("免费曲解析失败: %v", err)
	}
	if src == nil || src.URL == "" {
		t.Fatal("免费曲没有返回 URL")
	}

	// ② high 偏好（带 M800 的组合请求）：即便账号无 320k 权益，
	// 也必须通过降级链拿到 128k URL，而不是 cgi 1000 失败
	src2, err := c.ResolvePlay(s.Mid, s.MediaMid, s.AlbumMid, "high")
	if err != nil {
		if strings.Contains(err.Error(), "cgi 1000") {
			t.Fatalf("high 偏好组合请求被整体拒绝且未降级重试: %v", err)
		}
		t.Fatalf("high 偏好应可降级播放: %v", err)
	}
	if src2 == nil || src2.URL == "" {
		t.Fatal("high 偏好降级后没有返回 URL")
	}

	// ③ 真无权曲目：应报 104003 等可跳歌曲码，而不是 cgi 1000
	res2, err := c.Search("song", "小星星 儿歌", 1)
	if err != nil || len(res2.Songs) == 0 {
		t.Skipf("search2: %v", err)
	}
	s2 := res2.Songs[0]
	_, err = c.ResolvePlay(s2.Mid, s2.MediaMid, s2.AlbumMid, "standard")
	if err == nil {
		t.Skip("该曲目当前账号恰好可播（换个无权曲目再试）")
	}
	msg := err.Error()
	if strings.Contains(msg, "cgi 1000") {
		t.Fatalf("无权曲目仍报 cgi 1000（回退了）: %v", err)
	}
	if !strings.Contains(msg, "104003") && !strings.Contains(msg, "104009") &&
		!strings.Contains(msg, "101404") && !strings.Contains(msg, "no playable") {
		t.Fatalf("无权曲目应报可被前端跳歌规则匹配的错误码，实际: %v", err)
	}
}
