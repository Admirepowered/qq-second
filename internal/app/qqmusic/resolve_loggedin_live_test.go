package qqmusic

// 登录态播放解析回归测试（真实链路）。
//
// 背景（历史 bug「解析播放源失败: cgi 1000」）：vkey 接口在**登录态**下，
// 账号无权播放的曲目会返回顶层 req_0.code=1000（游客态则是 code=0 + 逐项
// result=104003），data.midurlinfo 里其实仍带着逐项结果。旧代码把 1000 当
// 整体失败直接抛错，前端匹配不到 104003 → 播放卡死。修复后：code=1000 时
// 继续走 midurlinfo 逐项降级，最终报「resolve failed result 104003」，
// 前端自动跳下一首。

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

	// 账号有权播放的曲目：应成功并给出 URL
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
		t.Fatalf("有权曲目解析失败: %v", err)
	}
	if src == nil || src.URL == "" {
		t.Fatal("有权曲目没有返回 URL")
	}

	// 无权曲目（登录态 cgi 1000 场景）：应报 104003 等可跳歌曲码，而不是 cgi 1000
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
		t.Fatalf("登录态无权曲目仍报 cgi 1000（回退了）: %v", err)
	}
	if !strings.Contains(msg, "104003") && !strings.Contains(msg, "104009") &&
		!strings.Contains(msg, "101404") && !strings.Contains(msg, "no playable") {
		t.Fatalf("无权曲目应报可被前端跳歌规则匹配的错误码，实际: %v", err)
	}
}
