package qqmusic

// 账号数据：创建的歌单（含"我喜欢"）、收藏的歌曲。
//
// "我喜欢"（dirId=201）本身就是一个真实歌单（tid），直接用 PlaylistDetail 拉取，
// 无需加密 uin；收藏歌单接口（PlaylistFavRead）需要 enc_uin，暂未接入。

// UserAccount 账号概览。
type UserAccount struct {
	Uin         string         `json:"uin"`
	Nickname    string         `json:"nickname"`
	Avatar      string         `json:"avatar,omitempty"`
	FavSongs    []Track        `json:"favSongs"`
	MyPlaylists []PlaylistInfo `json:"myPlaylists"`
}

// GetUserAccount 拉取账号歌单与"我喜欢"歌曲。
func (c *Client) GetUserAccount() (*UserAccount, error) {
	cred := c.Credential()
	if cred.StrMusicID == "" || cred.MusicKey == "" {
		return nil, ErrNotLoggedIn
	}
	account := &UserAccount{Uin: cred.StrMusicID, Nickname: cred.Nickname}

	// 1) 用户资料（昵称/头像兜底）
	if data, err := c.cgiAndroid("userInfo.BaseUserInfoServer", "get_user_baseinfo_v2", map[string]interface{}{
		"vec_uin":      []interface{}{cred.StrMusicID},
		"need_profile": 1,
	}); err == nil {
		profile := locateProfile(data, cred.StrMusicID)
		if profile != nil {
			if n := firstField(profile, "nick", "nickname", "name"); n != "" {
				account.Nickname = n
			}
			if a := firstField(profile, "headurl", "headUrl", "avatar", "logo"); a != "" {
				account.Avatar = a
			}
		}
	}

	// 2) 我创建的歌单（含 dirId=201 的"我喜欢"）
	myData, myErr := c.cgiAndroid("music.musicasset.PlaylistBaseRead", "GetPlaylistByUin", map[string]interface{}{
		"uin": cred.StrMusicID,
	})
	if myErr == nil {
		account.MyPlaylists = parseUserPlaylists(myData)
	}

	// 3) 收藏的歌曲：取 dirId=201（我喜欢）的 tid，用通用歌单接口拉取（支持分页）
	var favTid string
	for _, pl := range account.MyPlaylists {
		if pl.DirID == 201 {
			favTid = pl.ID
			break
		}
	}
	if favTid == "" && len(account.MyPlaylists) > 0 {
		favTid = account.MyPlaylists[0].ID
	}
	if favTid != "" {
		detail, err := c.PlaylistDetail(favTid, 0)
		if err == nil {
			account.FavSongs = detail.Tracks
			if detail.HasMore {
				// 简单翻页拉全（每页 100）
				for begin := 100; begin < detail.Total && begin < 500; begin += 100 {
					if more, err := c.PlaylistDetail(favTid, begin); err == nil && len(more.Tracks) > 0 {
						account.FavSongs = append(account.FavSongs, more.Tracks...)
					} else {
						break
					}
				}
			}
		}
	}

	return account, nil
}

// locateProfile 从 baseinfo 响应中定位当前用户资料节点。
func locateProfile(data map[string]interface{}, uin string) map[string]interface{} {
	if mu, ok := dig(data, "map_userinfo"); ok {
		if p, ok := dig(mu, uin); ok {
			return p
		}
	}
	if p, ok := dig(data, "userInfo"); ok {
		return p
	}
	if p, ok := dig(data, "info"); ok {
		return p
	}
	return data
}

// firstField 按候选键名取第一个非空字符串字段。
func firstField(m map[string]interface{}, names ...string) string {
	for _, name := range names {
		if v := stringAt(m, name); v != "" {
			return v
		}
	}
	return ""
}

// parseUserPlaylists 解析 GetPlaylistByUin 的 v_playlist 数组。
func parseUserPlaylists(data map[string]interface{}) []PlaylistInfo {
	out := make([]PlaylistInfo, 0, 16)
	seen := map[string]bool{}

	for _, item := range arrayAt(data, "v_playlist") {
		v, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id := firstField(v, "tid", "dirId", "dirid")
		title := firstField(v, "dirName", "dissname", "name")
		if id == "" || id == "0" || title == "" || seen[id] {
			continue
		}
		seen[id] = true
		count := 0
		if n, ok := numberAt(v, "songNum"); ok {
			count = int(n)
		} else if n, ok := numberAt(v, "song_count"); ok {
			count = int(n)
		}
		cover := firstField(v, "picUrl", "bigpicUrl", "picurl", "logo")
		dirID, _ := numberAt(v, "dirId")
		out = append(out, PlaylistInfo{
			ID:        id,
			Title:     stripEm(title),
			Cover:     cover,
			SongCount: count,
			DirID:     int(dirID),
		})
	}
	return out
}

// ErrNotLoggedIn 未登录错误。
var ErrNotLoggedIn = errNotLoggedIn{}

type errNotLoggedIn struct{}

func (errNotLoggedIn) Error() string { return "未登录 QQ 音乐账号" }
