package qqmusic

// 歌曲收藏 / 加入歌单：
// music.musicasset.PlaylistDetailWrite / AddSonglist | DelSonglist
// 参数 (dirId, tid, v_songInfo[{songId, songType}])：
//   - "我喜欢"：dirId=201，tid=0（服务端按 dirId 解析）
//   - 自建歌单：dirId=歌单 dirId，tid=歌单 tid
// 参考公开实现 L-1124/QQMusicApi songlist.py 的 add_songs/like_song。

import (
	"fmt"
	"strconv"
)

// songRef 通过 mid 解析写入歌单需要的数字 songId 与 songType。
func (c *Client) songRef(mid string) (int64, int, error) {
	data, err := c.cgiAndroid("music.pf_song_detail_svr", "get_song_detail_yqq", map[string]interface{}{
		"song_mid": mid,
	})
	if err != nil {
		return 0, 0, err
	}
	track, _ := dig(data, "track_info")
	if track == nil {
		return 0, 0, fmt.Errorf("song detail missing")
	}
	n, ok := numberAt(track, "id")
	if !ok || n <= 0 {
		return 0, 0, fmt.Errorf("song id missing")
	}
	songType := 0
	if t, ok := numberAt(track, "type"); ok {
		songType = int(t)
	}
	return int64(n), songType, nil
}

// playlistWrite 把一首歌加入/移出指定歌单。
func (c *Client) playlistWrite(mid string, dirID int, tid int64, add bool) error {
	cred := c.Credential()
	if cred.StrMusicID == "" || cred.MusicKey == "" {
		return ErrNotLoggedIn
	}
	songID, songType, err := c.songRef(mid)
	if err != nil {
		return err
	}
	method := "AddSonglist"
	if !add {
		method = "DelSonglist"
	}
	_, err = c.cgiAndroid("music.musicasset.PlaylistDetailWrite", method, map[string]interface{}{
		"dirId":    dirID,
		"tid":      tid,
		"bFmtUtf8": true,
		"v_songInfo": []interface{}{
			map[string]interface{}{"songId": songID, "songType": songType},
		},
	})
	return err
}

// favoriteWrite 收藏(add)/取消收藏(del)一首歌到"我喜欢"。
func (c *Client) favoriteWrite(mid string, add bool) error {
	return c.playlistWrite(mid, 201, 0, add)
}

// AddFavorite 收藏歌曲到"我喜欢"。
func (c *Client) AddFavorite(mid string) error {
	return c.favoriteWrite(mid, true)
}

// DelFavorite 从"我喜欢"取消收藏。
func (c *Client) DelFavorite(mid string) error {
	return c.favoriteWrite(mid, false)
}

// AddSongToPlaylist 把歌曲加入指定歌单（自建歌单）。
// playlistID 为歌单 tid（字符串），dirID 为该歌单的 dirId。
func (c *Client) AddSongToPlaylist(mid, playlistID string, dirID int) error {
	tid, err := strconv.ParseInt(playlistID, 10, 64)
	if err != nil || tid <= 0 {
		return fmt.Errorf("歌单 ID 无效: %s", playlistID)
	}
	if dirID <= 0 {
		return fmt.Errorf("歌单目录 ID 无效: %d", dirID)
	}
	return c.playlistWrite(mid, dirID, tid, true)
}

// ResolveSongId 通过 mid 解析数字 songId。
func (c *Client) ResolveSongId(mid string) (string, error) {
	n, _, err := c.songRef(mid)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(n, 10), nil
}
