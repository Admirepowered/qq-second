package qqmusic

// 歌曲收藏（"我喜欢" dirid=201）：
// music.musicasset.PlaylistDetailWrite / AddSonglist | DelSonglist
// （songId + songType 来自歌曲详情；tid 传 0 服务端按 dirId 解析；
//   参考公开实现 L-1124/QQMusicApi songlist.py 的 like_song/unlike_song。）

import (
	"fmt"
	"strconv"
)

// favoriteWrite 收藏(add)/取消收藏(del)一首歌到"我喜欢"。
func (c *Client) favoriteWrite(mid string, add bool) error {
	cred := c.Credential()
	if cred.StrMusicID == "" || cred.MusicKey == "" {
		return ErrNotLoggedIn
	}

	// 歌曲详情 → 数字 songId + songType
	data, err := c.cgiAndroid("music.pf_song_detail_svr", "get_song_detail_yqq", map[string]interface{}{
		"song_mid": mid,
	})
	if err != nil {
		return err
	}
	track, _ := dig(data, "track_info")
	if track == nil {
		return fmt.Errorf("song detail missing")
	}
	n, ok := numberAt(track, "id")
	if !ok || n <= 0 {
		return fmt.Errorf("song id missing")
	}
	songType := 0
	if t, ok := numberAt(track, "type"); ok {
		songType = int(t)
	}
	method := "AddSonglist"
	if !add {
		method = "DelSonglist"
	}
	_, err = c.cgiAndroid("music.musicasset.PlaylistDetailWrite", method, map[string]interface{}{
		"dirId":    201,
		"tid":      0,
		"bFmtUtf8": true,
		"v_songInfo": []interface{}{
			map[string]interface{}{"songId": n, "songType": songType},
		},
	})
	return err
}

// AddFavorite 收藏歌曲到"我喜欢"。
func (c *Client) AddFavorite(mid string) error {
	return c.favoriteWrite(mid, true)
}

// DelFavorite 从"我喜欢"取消收藏。
func (c *Client) DelFavorite(mid string) error {
	return c.favoriteWrite(mid, false)
}

// ResolveSongId 通过 mid 解析数字 songId。
func (c *Client) ResolveSongId(mid string) (string, error) {
	data, err := c.cgiAndroid("music.pf_song_detail_svr", "get_song_detail_yqq", map[string]interface{}{
		"song_mid": mid,
	})
	if err != nil {
		return "", err
	}
	track, _ := dig(data, "track_info")
	if track == nil {
		return "", fmt.Errorf("song detail missing")
	}
	if n, ok := numberAt(track, "id"); ok && n > 0 {
		return strconv.FormatInt(n, 10), nil
	}
	return "", fmt.Errorf("song id missing")
}
