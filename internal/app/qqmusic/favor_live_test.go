package qqmusic

import (
	"encoding/json"
	"os"
	"testing"
)

// 实测收藏写接口（读本机 store.json 凭据；收藏后取消，恢复原状）
func TestLiveFavoriteWrite(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("live test")
	}
	raw, err := os.ReadFile(`C:\Users\admir\AppData\Roaming\qq-second\store.json`)
	if err != nil {
		t.Skip("no store.json")
	}
	var store struct {
		Credential struct {
			Uin        string `json:"uin"`
			QQMusicKey string `json:"qqMusicKey"`
		} `json:"credential"`
	}
	if err := json.Unmarshal(raw, &store); err != nil || store.Credential.QQMusicKey == "" {
		t.Skip("no credential")
	}
	c := NewClient()
	c.SetCredential(Credential{
		StrMusicID: store.Credential.Uin,
		MusicID:    1204844044,
		MusicKey:   store.Credential.QQMusicKey,
		LoginType:  2,
	})
	mid := "0008BWiu0BjO6O" // 突然好想你

	// 1. 收藏
	if err := c.AddFavorite(mid); err != nil {
		t.Fatalf("add fav: %v", err)
	}
	t.Log("added")

	// 2. 验证在我喜欢列表里
	found := false
	var favTid string
	acct, err := c.GetUserAccount()
	if err == nil {
		for _, pl := range acct.MyPlaylists {
			if pl.DirID == 201 {
				favTid = pl.ID
				break
			}
		}
	}
	if favTid != "" {
		for begin := 0; begin < 500; begin += 100 {
			detail, err := c.PlaylistDetail(favTid, begin)
			if err != nil || len(detail.Tracks) == 0 {
				break
			}
			for _, tr := range detail.Tracks {
				if tr.Mid == mid {
					found = true
				}
			}
			if !detail.HasMore {
				break
			}
		}
	}
	if !found {
		t.Fatalf("added but NOT in 我喜欢 list")
	}
	t.Log("verified in 我喜欢")

	// 3. 取消收藏（恢复原状）
	if err := c.DelFavorite(mid); err != nil {
		t.Fatalf("del fav: %v", err)
	}
	t.Log("removed")
}
