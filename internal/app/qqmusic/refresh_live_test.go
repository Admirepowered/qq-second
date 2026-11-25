package qqmusic

// 凭据刷新回归测试（真实链路）。
//
// 背景：用户实测「年费账号，但只有重新登录拿到的 cookie 才带播放权益」。
// 排查结论：musickey 有效期只有 3 天（keyExpiresIn=259200），且播放权益校验
// 依赖「新签发的 key + 新申请的 session」组合；key/session 变旧后 vkey 对
// 有权曲目也返回 code=1000（整组拒绝）。官方客户端靠 Login 刷新接口换新 key。
//
// 本测试验证：只凭 musickey（旧版登录没存 refresh_token）也能换到新 key。

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestLiveRefreshCredential(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("set QQ_LIVE=1 to run live test")
	}
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

	// 用真实凭据但只读：刷新结果不写回 store.json
	c := NewClient()
	c.SetCredential(Credential{
		MusicID:    1204844044,
		StrMusicID: store.Credential.Uin,
		MusicKey:   store.Credential.QQMusicKey,
		LoginType:  2,
	})

	fresh, err := c.RefreshCredential()
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}
	if fresh.MusicKey == "" {
		t.Fatal("刷新后没有返回新 key")
	}
	if fresh.StrMusicID != store.Credential.Uin {
		t.Fatalf("账号不匹配: %s != %s", fresh.StrMusicID, store.Credential.Uin)
	}
	if fresh.KeyExpiresIn <= 0 {
		t.Logf("提示：响应未带 keyExpiresIn（不致命）")
	} else {
		t.Logf("新 key 有效期 %d 秒（%.1f 天），到期 %s",
			fresh.KeyExpiresIn, float64(fresh.KeyExpiresIn)/86400,
			time.Unix(fresh.ExpiredAt, 0).Format("2006-01-02 15:04"))
	}
}

func TestInvalidateSession(t *testing.T) {
	c := NewClient()
	c.mu.Lock()
	c.uid, c.sid = "u", "s"
	c.mu.Unlock()
	c.InvalidateSession()
	if c.uid != "" || c.sid != "" {
		t.Fatal("session 未清空")
	}
}