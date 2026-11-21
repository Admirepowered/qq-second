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
