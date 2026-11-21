package app

// QQ 音乐绑定方法（前端通过 wailsjs 调用）。

import (
	"time"

	"qqsecond/internal/app/qqmusic"
)

// QQSearch 分类搜索。
func (a *App) QQSearch(kind, query string, page int) (*qqmusic.SearchResult, error) {
	a.ensureQQ()
	return a.qq.Search(kind, query, page)
}

// QQHome 首页数据。
func (a *App) QQHome() (*qqmusic.HomeFeed, error) {
	a.ensureQQ()
	return a.qq.Home()
}

// QQCharts 榜单入口列表（静态）。
func (a *App) QQCharts() []qqmusic.ChartInfo {
	return qqmusic.Charts()
}

// QQChartDetail 榜单详情。
func (a *App) QQChartDetail(topID int) (*qqmusic.PlaylistDetail, error) {
	a.ensureQQ()
	return a.qq.ChartDetail(topID)
}

// QQPlaylist 歌单详情（分页）。
func (a *App) QQPlaylist(id string, begin int) (*qqmusic.PlaylistDetail, error) {
	a.ensureQQ()
	return a.qq.PlaylistDetail(id, begin)
}

// QQAlbum 专辑详情。
func (a *App) QQAlbum(mid string) (*qqmusic.AlbumDetail, error) {
	a.ensureQQ()
	return a.qq.AlbumDetail(mid)
}

// QQArtist 歌手详情。
func (a *App) QQArtist(mid string) (*qqmusic.ArtistDetail, error) {
	a.ensureQQ()
	return a.qq.ArtistDetail(mid)
}

// QQLyrics 歌词文档。
func (a *App) QQLyrics(mid string) (*qqmusic.LyricDocument, error) {
	a.ensureQQ()
	return a.qq.GetLyrics(mid)
}

// QQResolvePlay 解析播放源（返回 token，签名 URL 不出后端）。
func (a *App) QQResolvePlayBinding(track Track, preferredQuality string) (PlaybackSource, error) {
	return a.QQResolvePlay(track, preferredQuality)
}

// QQStartLogin 获取登录二维码。
func (a *App) QQStartLogin() (*qqmusic.QRCodeImage, error) {
	a.ensureQQ()
	return a.qq.StartQRLogin()
}

// QQPollLogin 轮询扫码状态。
func (a *App) QQPollLogin() (*qqmusic.QRLoginPoll, error) {
	a.ensureQQ()
	result, err := a.qq.PollQRLogin()
	if err != nil {
		return nil, err
	}
	if result != nil && result.Status == "done" && result.Credential != nil {
		a.mu.Lock()
		a.saveCredentialLocked(Credential{
			Uin:        result.Credential.StrMusicID,
			QQMusicKey: result.Credential.MusicKey,
			Nickname:   result.Credential.Nickname,
			LoginType:  result.Credential.LoginType,
			LoggedAt:   time.Now().Unix(),
		})
		a.mu.Unlock()
	}
	return result, nil
}

// QQLogout 退出登录。
func (a *App) QQLogout() error {
	a.ensureQQ()
	a.qq.Logout()
	a.mu.Lock()
	a.saveCredentialLocked(Credential{})
	a.mu.Unlock()
	return nil
}

func (a *App) ensureQQ() {
	if a.qq == nil {
		a.qq = qqmusic.NewClient()
	}
}

// QQUserAccount 账号概览（收藏歌曲 + 我的歌单 + 收藏歌单）。
func (a *App) QQUserAccount() (*qqmusic.UserAccount, error) {
	a.ensureQQ()
	return a.qq.GetUserAccount()
}

// QQListFavoriteMids 返回"我喜欢"歌单里全部歌曲的 mid 集合（需登录）。
func (a *App) QQListFavoriteMids() ([]string, error) {
	a.ensureQQ()
	// 拉账号歌单，找 dirId=201（我喜欢）的 tid
	account, err := a.qq.GetUserAccount()
	if err != nil {
		return nil, err
	}
	var favTid string
	for _, pl := range account.MyPlaylists {
		if pl.DirID == 201 {
			favTid = pl.ID
			break
		}
	}
	if favTid == "" {
		return []string{}, nil
	}
	mids := []string{}
	for begin := 0; ; begin += 100 {
		detail, err := a.qq.PlaylistDetail(favTid, begin)
		if err != nil || len(detail.Tracks) == 0 {
			break
		}
		for _, t := range detail.Tracks {
			mids = append(mids, t.Mid)
		}
		if !detail.HasMore || len(detail.Tracks) < 100 {
			break
		}
	}
	return mids, nil
}

// QQSetFavorite 收藏/取消收藏歌曲（需登录）。
func (a *App) QQSetFavorite(track Track, fav bool) error {
	a.ensureQQ()
	if fav {
		return a.qq.AddFavorite(track.Mid)
	}
	return a.qq.DelFavorite(track.Mid)
}
