package qqmusic

// 歌曲收藏（"我喜欢" dirid=201）：
// - 写接口 music.musicfavor.SongFavor / AddFavSong | DelFavSong（需数字 songId，需登录）
// - 状态查询 music.musicasset.SongFavRead / IsSongFanByMid（响应 schema 未完全验证，宽松解析）

import (
	"fmt"
	"strconv"
)

// AddFavorite 收藏歌曲到"我喜欢"。
func (c *Client) AddFavorite(songId string) error {
	return c.songFavorWrite(songId, "AddFavSong")
}

// DelFavorite 从"我喜欢"取消收藏。
func (c *Client) DelFavorite(songId string) error {
	return c.songFavorWrite(songId, "DelFavSong")
}

func (c *Client) songFavorWrite(songId, method string) error {
	cred := c.Credential()
	if cred.StrMusicID == "" || cred.MusicKey == "" {
		return ErrNotLoggedIn
	}
	n, err := strconv.ParseInt(songId, 10, 64)
	if err != nil {
		return fmt.Errorf("songId 必须为数字: %s", songId)
	}
	_, err = c.cgiAndroid("music.musicfavor.SongFavor", method, map[string]interface{}{
		"dirid":      201,
		"songidlist": []interface{}{n},
	})
	return err
}

// ResolveSongId 通过 mid 解析数字 songId（歌曲详情接口）。
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

// IsSongFavor 检查歌曲是否已收藏（宽松解析；拿不到结果时返回 false 与 err==nil）。
func (c *Client) IsSongFavor(mid string) (bool, error) {
	cred := c.Credential()
	if cred.StrMusicID == "" || cred.MusicKey == "" {
		return false, ErrNotLoggedIn
	}
	data, err := c.cgiAndroid("music.musicasset.SongFavRead", "IsSongFanByMid", map[string]interface{}{
		"songmid": []interface{}{mid},
	})
	if err != nil {
		return false, err
	}
	// 响应结构未完全验证：在整棵树里找 true 标记
	found := scanForTrueFlag(data, 0, 4)
	return found, nil
}

func scanForTrueFlag(node interface{}, depth, maxDepth int) bool {
	if depth > maxDepth || node == nil {
		return false
	}
	switch v := node.(type) {
	case map[string]interface{}:
		for _, child := range v {
			if b, ok := child.(bool); ok && b {
				return true
			}
			if scanForTrueFlag(child, depth+1, maxDepth) {
				return true
			}
		}
	case []interface{}:
		for _, child := range v {
			if scanForTrueFlag(child, depth+1, maxDepth) {
				return true
			}
		}
	}
	return false
}
