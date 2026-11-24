package qqmusic

// 原始响应 → 统一 Track 模型的归一化。

import (
	"fmt"
	"strconv"
)

// Track 与 app 包的 Track 结构对齐（JSON 形状一致）。
type Track struct {
	Kind       string    `json:"kind"` // "qq"
	Id         string    `json:"id"`
	Mid        string    `json:"mid,omitempty"`
	SongId     string    `json:"songId,omitempty"` // 数字歌曲 ID（收藏接口需要）
	MediaMid   string    `json:"mediaMid,omitempty"`
	AlbumMid   string    `json:"albumMid,omitempty"`
	Title      string    `json:"title"`
	Artists    []NameRef `json:"artists"`
	Album      string    `json:"album,omitempty"`
	DurationMs int       `json:"durationMs"`
	Artwork    string    `json:"artwork,omitempty"`
	AlbumMidForCover string `json:"-"`
	Quality    string    `json:"quality,omitempty"`
	Capability string    `json:"capability,omitempty"` // full | preview | unavailable
	TrackNumber int      `json:"trackNumber,omitempty"`
}

// NameRef 歌手引用。
type NameRef struct {
	Mid  string `json:"mid,omitempty"`
	Name string `json:"name"`
}

// AlbumRef 专辑引用。
type AlbumRef struct {
	Mid   string `json:"mid,omitempty"`
	Title string `json:"title,omitempty"`
}

// artworkURL 按专辑 mid 生成封面地址。
func artworkURL(albumMid string, size int) string {
	if albumMid == "" {
		return ""
	}
	return fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R%dx%dM000%s.jpg", size, size, albumMid)
}

func singerArtworkURL(singerMid string) string {
	if singerMid == "" {
		return ""
	}
	return fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T001R500x500M000%s.jpg", singerMid)
}

// normalizeSong 把 musicu.fcg 的 song 对象归一化为 Track。
func normalizeSong(raw map[string]interface{}) (Track, bool) {
	mid := stringAt(raw, "mid")
	if mid == "" {
		mid = stringAt(raw, "songmid")
	}
	if mid == "" {
		return Track{}, false
	}
	title := stringAt(raw, "title")
	if title == "" {
		title = stringAt(raw, "name")
	}
	if title == "" {
		title = stringAt(raw, "songname")
	}
	title = stripEm(title)

	track := Track{Kind: "qq", Id: mid, Mid: mid, Title: title, Capability: "full"}
	// 数字歌曲 ID（收藏/举报等写接口需要）
	if n, ok := numberAt(raw, "id"); ok && n > 0 {
		track.SongId = strconv.FormatInt(n, 10)
	}

	// 歌手
	for _, s := range arrayAt(raw, "singer") {
		singer, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		track.Artists = append(track.Artists, NameRef{
			Mid:  stringAt(singer, "mid"),
			Name: stripEm(stringAt(singer, "name")),
		})
	}
	// 兼容旧字段 singerName (client_search_cp)
	if len(track.Artists) == 0 {
		if n := stringAt(raw, "singername"); n != "" {
			track.Artists = append(track.Artists, NameRef{Name: n})
		}
	}

	// 专辑
	album, hasAlbum := dig(raw, "album")
	if hasAlbum {
		track.Album = stripEm(stringAt(album, "title"))
		if track.Album == "" {
			track.Album = stripEm(stringAt(album, "name"))
		}
		if track.Album == "" {
			track.Album = stripEm(stringAt(album, "albumname"))
		}
		track.AlbumMid = stringAt(album, "mid")
	} else if am := stringAt(raw, "albummid"); am != "" {
		track.AlbumMid = am
		track.Album = stripEm(stringAt(raw, "albumname"))
	}

	// 时长（秒 → 毫秒）
	if interval, ok := numberAt(raw, "interval"); ok && interval > 0 {
		track.DurationMs = int(interval) * 1000
	} else if interval, ok := numberAt(raw, "song_duration"); ok {
		track.DurationMs = int(interval)
	}

	// 文件信息
	if file, ok := dig(raw, "file"); ok {
		track.MediaMid = stringAt(file, "media_mid")
		if track.MediaMid == "" {
			track.MediaMid = mid
		}
		hasFull := false
		for _, key := range []string{"size_320mp3", "size_128mp3", "size_192aac", "size_flac", "size_hires"} {
			if size, ok := numberAt(file, key); ok && size > 0 {
				hasFull = true
				break
			}
		}
		if trySize, ok := numberAt(file, "size_try"); ok && trySize > 0 && !hasFull {
			track.Capability = "preview"
		}
		if !hasFull && track.Capability != "preview" {
			// 仍然尝试解析（可能只是字段缺失），交给 vkey 决定
			track.Capability = "full"
		}
	} else {
		track.MediaMid = mid
	}
	if track.MediaMid == "" {
		track.MediaMid = mid
	}

	// 音质估计
	if sizeFlac, ok := numberAt(raw, "size_flac"); ok && sizeFlac > 0 {
		track.Quality = "lossless"
	} else if file, ok := dig(raw, "file"); ok {
		if size, ok := numberAt(file, "size_flac"); ok && size > 0 {
			track.Quality = "lossless"
		} else if size, ok := numberAt(file, "size_320mp3"); ok && size > 0 {
			track.Quality = "high"
		} else {
			track.Quality = "standard"
		}
	} else {
		track.Quality = "standard"
	}

	if track.AlbumMid != "" {
		track.Artwork = artworkURL(track.AlbumMid, 300)
	}

	// 曲序
	if n, ok := numberAt(raw, "index"); ok {
		track.TrackNumber = int(n)
	}

	return track, true
}

// normalizeSongList 归一化歌曲数组。
func normalizeSongList(list []interface{}) []Track {
	tracks := make([]Track, 0, len(list))
	for _, item := range list {
		song, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if t, ok := normalizeSong(song); ok {
			tracks = append(tracks, t)
		}
	}
	return tracks
}

// PlaylistInfo 歌单/榜单元信息。
type PlaylistInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Cover     string `json:"cover"`
	Creator   string `json:"creator,omitempty"`
	SongCount int    `json:"songCount,omitempty"`
	Desc      string `json:"desc,omitempty"`
	UpdateTime string `json:"updateTime,omitempty"`
	DirID     int    `json:"dirId,omitempty"` // 歌单目录 ID（"我喜欢"=201；写入歌单需要）
}

// PlaylistDetail 歌单/榜单详情。
type PlaylistDetail struct {
	Info   PlaylistInfo `json:"info"`
	Tracks []Track      `json:"tracks"`
	Total  int          `json:"total"`
	HasMore bool        `json:"hasMore"`
}

// AlbumDetail 专辑详情。
type AlbumDetail struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Artists     []NameRef `json:"artists"`
	Cover       string    `json:"cover"`
	ReleaseYear string    `json:"releaseYear,omitempty"`
	Company     string    `json:"company,omitempty"`
	Desc        string    `json:"desc,omitempty"`
	Tracks      []Track   `json:"tracks"`
}

// ArtistDetail 歌手详情。
type ArtistDetail struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Cover       string       `json:"cover"`
	Description string       `json:"description,omitempty"`
	Songs       []Track      `json:"songs"`
	Albums      []AlbumBrief `json:"albums"`
}

// AlbumBrief 歌手页的专辑卡片。
type AlbumBrief struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Cover       string `json:"cover"`
	ReleaseYear string `json:"releaseYear,omitempty"`
}

// SearchResult 分类搜索结果。
type SearchResult struct {
	Songs     []Track        `json:"songs"`
	Artists   []ArtistBrief  `json:"artists"`
	Albums    []AlbumBrief   `json:"albums"`
	Playlists []PlaylistInfo `json:"playlists"`
	SongTotal int64          `json:"songTotal"`
	More      bool           `json:"more"`
}

// ArtistBrief 歌手搜索卡片。
type ArtistBrief struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cover string `json:"cover,omitempty"`
}

// ChartInfo 榜单入口卡片。
type ChartInfo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Cover   string `json:"cover,omitempty"`
	UpdateTime string `json:"updateTime,omitempty"`
}

func albumBriefFrom(raw map[string]interface{}) (AlbumBrief, bool) {
	id := stringAt(raw, "mid")
	if id == "" {
		id = stringAt(raw, "albumMid")
	}
	if id == "" {
		return AlbumBrief{}, false
	}
	title := stringAt(raw, "name")
	if title == "" {
		title = stringAt(raw, "title")
		if title == "" {
			title = stringAt(raw, "albumName")
		}
	}
	brief := AlbumBrief{ID: id, Title: title, Cover: artworkURL(id, 300)}
	if t, ok := numberAt(raw, "aDate"); ok && t > 0 {
		brief.ReleaseYear = strconv.FormatInt(t, 10)
	} else if y := stringAt(raw, "timePublic"); y != "" {
		brief.ReleaseYear = y
	}
	return brief, true
}
