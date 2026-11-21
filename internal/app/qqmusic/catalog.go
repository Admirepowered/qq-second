package qqmusic

// 目录类接口：搜索 / 首页推荐 / 榜单 / 歌单 / 专辑 / 歌手。

import (
	"fmt"
	"strconv"
	"strings"
)

// Search 分类搜索（歌曲/歌手/专辑/歌单），Android 平台。
func (c *Client) Search(kind string, query string, page int) (*SearchResult, error) {
	searchType := 0
	switch kind {
	case "artist":
		searchType = 1
	case "album":
		searchType = 2
	case "playlist":
		searchType = 3
	}
	param := map[string]interface{}{
		"searchid":     getSearchID(),
		"query":        query,
		"search_type":  searchType,
		"num_per_page": 30,
		"page_num":     page,
		"highlight":    true,
		"grp":          true,
		"selectors":    map[string]interface{}{},
		"vec_selectors": []interface{}{},
	}
	data, err := c.cgiAndroid("music.search.SearchCgiService", "DoSearchForQQMusicMobile", param)
	if err != nil {
		return nil, err
	}
	body, _ := dig(data, "body")
	result := &SearchResult{More: true, Songs: []Track{}, Artists: []ArtistBrief{}, Albums: []AlbumBrief{}, Playlists: []PlaylistInfo{}}
	if body == nil {
		return result, nil
	}
	switch kind {
	case "song":
		result.Songs = normalizeSongList(arrayAt(body, "item_song"))
	case "artist":
		for _, item := range arrayAt(body, "singer") {
			s, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			mid := stringAt(s, "mid")
			name := stringAt(s, "name")
			if mid == "" {
				continue
			}
			result.Artists = append(result.Artists, ArtistBrief{ID: mid, Name: name, Cover: singerArtworkURL(mid)})
		}
	case "album":
		for _, item := range arrayAt(body, "item_album") {
			s, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if brief, ok := albumBriefFrom(s); ok {
				result.Albums = append(result.Albums, brief)
			}
		}
	case "playlist":
		for _, item := range arrayAt(body, "item_songlist") {
			s, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			id := stringAt(s, "dissid")
			if id == "" {
				continue
			}
			title := stripEm(stringAt(s, "dissname"))
			creator := ""
			if nick, ok := dig(s, "creator"); ok {
				creator = stringAt(nick, "nick")
			}
			songCount := 0
			if n, ok := numberAt(s, "songnum"); ok {
				songCount = int(n)
			}
			result.Playlists = append(result.Playlists, PlaylistInfo{
				ID: id, Title: title, Cover: stringAt(s, "logo"),
				Creator: creator, SongCount: songCount,
			})
		}
	}
	if meta, ok := dig(data, "meta"); ok {
		if sum, ok := numberAt(meta, "sum"); ok {
			result.SongTotal = sum
		}
	}
	return result, nil
}

// stripEm 去掉搜索高亮 <em> 标签。
func stripEm(s string) string {
	s = strings.ReplaceAll(s, "<em>", "")
	return strings.ReplaceAll(s, "</em>", "")
}

// HomeFeed 首页内容（推荐歌单 + 新歌）。
type HomeFeed struct {
	Playlists []PlaylistInfo `json:"playlists"`
	NewSongs  []Track        `json:"newSongs"`
}

// Home 获取首页数据（匿名 web，游客可用）。
func (c *Client) Home() (*HomeFeed, error) {
	feed := &HomeFeed{Playlists: []PlaylistInfo{}, NewSongs: []Track{}}

	// 推荐歌单
	data, err := c.cgiAnonymousWeb("music.playlist.PlaylistSquare", "GetRecommendFeed", map[string]interface{}{
		"From": 0, "Size": 30,
	})
	if err == nil {
		for _, item := range arrayAt(data, "List") {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			pl, ok := dig(entry, "Playlist")
			if !ok {
				continue
			}
			basic, hasBasic := dig(pl, "basic")
			if !hasBasic {
				basic = pl
			}
			id := stringAt(basic, "id")
			if id == "" {
				id = stringAt(basic, "tid")
			}
			if id == "" {
				continue
			}
			cover := stringAt(basic, "pic_url")
			if cover == "" {
				cover = stringAt(basic, "picurl")
			}
			feed.Playlists = append(feed.Playlists, PlaylistInfo{
				ID: id, Title: stringAt(basic, "title"), Cover: cover,
			})
		}
	} else {
		fmt.Println("PlaylistSquare failed:", err)
	}

	// 新歌
	data, err = c.cgiAnonymousWeb("newsong.NewSongServer", "get_new_song_info", map[string]interface{}{
		"type": 5,
	})
	if err == nil {
		feed.NewSongs = normalizeSongList(arrayAt(data, "songlist"))
	} else {
		fmt.Println("newsong failed:", err)
	}

	return feed, nil
}

// Charts 首页榜单入口（固定 ID 列表，游客可用）。
func Charts() []ChartInfo {
	return []ChartInfo{
		{ID: 62, Name: "热歌榜"},
		{ID: 26, Name: "飙升榜"},
		{ID: 27, Name: "新歌榜"},
		{ID: 4, Name: "流行指数榜"},
		{ID: 3, Name: "欧美热榜"},
		{ID: 5, Name: "韩国热榜"},
		{ID: 6, Name: "日本热榜"},
		{ID: 57, Name: "粤语热榜"},
	}
}

// ChartDetail 获取榜单详情（匿名 web）。
func (c *Client) ChartDetail(topID int) (*PlaylistDetail, error) {
	data, err := c.cgiAnonymousWeb("musicToplist.ToplistInfoServer", "GetDetail", map[string]interface{}{
		"topId": topID, "offset": 0, "num": 100, "period": "",
	})
	if err != nil {
		return nil, err
	}
	detail := &PlaylistDetail{HasMore: false, Tracks: []Track{}}
	if info, ok := dig(data, "topinfo"); ok {
		detail.Info = PlaylistInfo{
			ID:     "top:" + strconv.Itoa(topID),
			Title:  stringAt(info, "topName"),
			Cover:  stringAt(info, "picurl"),
			UpdateTime: stringAt(info, "updateTime"),
			Desc:   stringAt(info, "topInfo"),
		}
		if detail.Info.Cover == "" {
			detail.Info.Cover = stringAt(info, "frontPicurl")
		}
	}
	if list := arrayAt(data, "songInfoList"); list != nil {
		detail.Tracks = normalizeSongList(list)
		detail.Total = len(detail.Tracks)
	}
	return detail, nil
}

// PlaylistDetail 获取歌单详情（分页 song_begin，每页 100）。
func (c *Client) PlaylistDetail(disstid string, begin int) (*PlaylistDetail, error) {
	idNum, err := strconv.ParseInt(disstid, 10, 64)
	if err != nil {
		return nil, err
	}
	data, err := c.cgiAndroid("music.srfDissInfo.DissInfo", "CgiGetDiss", map[string]interface{}{
		"disstid": idNum, "dirid": 0, "tag": 0,
		"song_begin": begin, "song_num": 100,
		"userinfo": 1, "orderlist": true, "onlysonglist": false,
	})
	if err != nil {
		return nil, err
	}
	detail := &PlaylistDetail{Tracks: []Track{}}
	if dirinfo, ok := dig(data, "dirinfo"); ok {
		creator := ""
		if cr, ok := dig(dirinfo, "creator"); ok {
			creator = stringAt(cr, "nick")
		}
		detail.Info = PlaylistInfo{
			ID: disstid, Title: stringAt(dirinfo, "dirName"),
			Cover: stringAt(dirinfo, "picUrl"), Creator: creator,
			Desc: stringAt(dirinfo, "desc"),
		}
	}
	if list := arrayAt(data, "songlist"); list != nil {
		detail.Tracks = normalizeSongList(list)
	}
	if total, ok := numberAt(data, "total_song_num"); ok {
		detail.Total = int(total)
	} else if t, ok := numberAt(data, "total"); ok {
		detail.Total = int(t)
	}
	if hasmore, ok := numberAt(data, "hasmore"); ok {
		detail.HasMore = hasmore == 1
	}
	return detail, nil
}

// AlbumDetail 获取专辑详情（匿名 web 可用，退回 Android）。
func (c *Client) AlbumDetail(albumMid string) (*AlbumDetail, error) {
	data, err := c.cgiAnonymousWeb("music.musichallAlbum.AlbumInfoServer", "GetAlbumDetail", map[string]interface{}{
		"albumMId": albumMid,
	})
	if err != nil {
		data, err = c.cgiAndroid("music.musichallAlbum.AlbumInfoServer", "GetAlbumDetail", map[string]interface{}{
			"albumMId": albumMid,
		})
		if err != nil {
			return nil, err
		}
	}
	detail := &AlbumDetail{ID: albumMid, Cover: artworkURL(albumMid, 500), Tracks: []Track{}, Artists: []NameRef{}}
	if info, ok := dig(data, "getAlbumInfo", "albumInfo"); ok {
		detail.Title = stringAt(info, "albumName")
		detail.Company = stringAt(info, "company")
		detail.Desc = stripEm(stringAt(info, "albumDesc"))
		if release := stringAt(info, "releaseDate"); len(release) >= 4 {
			detail.ReleaseYear = release[:4]
		}
		for _, s := range arrayAt(info, "singers") {
			singer, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			detail.Artists = append(detail.Artists, NameRef{Mid: stringAt(singer, "mid"), Name: stringAt(singer, "name")})
		}
	}
	songs, err := c.AlbumSongList(albumMid)
	if err == nil {
		detail.Tracks = songs
	}
	return detail, nil
}

// AlbumSongList 获取专辑曲目。
func (c *Client) AlbumSongList(albumMid string) ([]Track, error) {
	data, err := c.cgiAnonymousWeb("music.musichallAlbum.AlbumSongList", "GetAlbumSongList", map[string]interface{}{
		"albumMid": albumMid, "begin": 0, "num": 100,
	})
	if err != nil {
		data, err = c.cgiAndroid("music.musichallAlbum.AlbumSongList", "GetAlbumSongList", map[string]interface{}{
			"albumMid": albumMid, "begin": 0, "num": 100,
		})
		if err != nil {
			return nil, err
		}
	}
	return normalizeSongList(arrayAt(data, "songList")), nil
}

// ArtistDetail 获取歌手详情 + 热门歌曲 + 专辑。
func (c *Client) ArtistDetail(singerMid string) (*ArtistDetail, error) {
	detail := &ArtistDetail{ID: singerMid, Cover: singerArtworkURL(singerMid), Songs: []Track{}, Albums: []AlbumBrief{}}

	// 歌手信息（含描述）
	data, err := c.cgiAnonymousWeb("music.musichallSinger.SingerInfoInter", "GetSingerDetail", map[string]interface{}{
		"singer_mids": []interface{}{singerMid},
	})
	if err == nil {
		if info, ok := dig(data, "singer_detail"); ok {
			detail.Name = stringAt(info, "name")
			detail.Description = stringAt(info, "desc")
		}
	}
	if detail.Name == "" {
		if data, err := c.cgiAnonymousWeb("music.UnifiedHomepage.UnifiedHomepageSrv", "GetHomepageHeader", map[string]interface{}{
			"SingerMid": singerMid,
		}); err == nil {
			if header, ok := dig(data, "homepage", "Header"); ok {
				detail.Name = stringAt(header, "singerName")
			}
		}
	}

	// 热门歌曲
	data, err = c.cgiAndroid("musichall.song_list_server", "GetSingerSongList", map[string]interface{}{
		"singerMid": singerMid, "order": 1, "number": 50, "begin": 0,
	})
	if err == nil {
		detail.Songs = normalizeSongList(arrayAt(data, "songList"))
	}

	// 专辑
	data, err = c.cgiAndroid("music.musichallAlbum.AlbumListServer", "GetAlbumList", map[string]interface{}{
		"singerMid": singerMid, "number": 30, "begin": 0,
	})
	if err == nil {
		for _, item := range arrayAt(data, "albumList") {
			album, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if brief, ok := albumBriefFrom(album); ok {
				detail.Albums = append(detail.Albums, brief)
			}
		}
	}
	return detail, nil
}
