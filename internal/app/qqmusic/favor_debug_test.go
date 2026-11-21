package qqmusic

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestLiveFavVariants(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("live test")
	}
	raw, _ := os.ReadFile(`C:\Users\admir\AppData\Roaming\qq-second\store.json`)
	var store struct {
		Credential struct {
			Uin        string `json:"uin"`
			QQMusicKey string `json:"qqMusicKey"`
		} `json:"credential"`
	}
	json.Unmarshal(raw, &store)
	c := NewClient()
	c.SetCredential(Credential{
		StrMusicID: store.Credential.Uin,
		MusicID:    1204844044,
		MusicKey:   store.Credential.QQMusicKey,
		LoginType:  2,
	})

	variants := []struct {
		label  string
		module string
		method string
		param  map[string]interface{}
	}{
		{"A: SongFavor/AddFavSong dirid+songidlist", "music.musicfavor.SongFavor", "AddFavSong", map[string]interface{}{
			"dirid": 201, "songidlist": []interface{}{335601186}}},
		{"B: songidlist 字符串", "music.musicfavor.SongFavor", "AddFavSong", map[string]interface{}{
			"dirid": 201, "songidlist": []interface{}{"335601186"}}},
		{"C: 加 msgidlist", "music.musicfavor.SongFavor", "AddFavSong", map[string]interface{}{
			"dirid": 201, "songidlist": []interface{}{335601186}, "msgidlist": []interface{}{0}}},
		{"D: SongFavorCgi", "music.musicfavor.SongFavorCgi", "AddFavSong", map[string]interface{}{
			"dirid": 201, "songidlist": []interface{}{335601186}}},
	}
	for _, v := range variants {
		data, err := c.cgiAndroid(v.module, v.method, v.param)
		j, _ := json.Marshal(data)
		out := string(j)
		if len(out) > 300 {
			out = out[:300]
		}
		fmt.Printf("%s\n  err=%v\n  data=%s\n\n", v.label, err, out)
	}
}
