package app

// Track 是本地音乐与 QQ 音乐统一的曲目模型（前端队列/列表都用它）。
type Track struct {
	Kind      string    `json:"kind"` // "qq" | "local"
	Id        string    `json:"id"`   // qq: songMid；local: 路径哈希
	Mid       string    `json:"mid,omitempty"`
	SongId    string    `json:"songId,omitempty"`
	MediaMid  string    `json:"mediaMid,omitempty"`
	AlbumMid  string    `json:"albumMid,omitempty"`
	Title     string    `json:"title"`
	Artists   []NameRef `json:"artists"`
	Album     string    `json:"album,omitempty"`
	DurationMs int     `json:"durationMs"`
	Artwork   string    `json:"artwork,omitempty"`
	Path      string    `json:"path,omitempty"` // 本地文件绝对路径
	Ext       string    `json:"ext,omitempty"`
	TrackNum  int       `json:"trackNum,omitempty"`
	// Quality 由解析播放源时填充：standard/high/lossless
	Quality string `json:"quality,omitempty"`
	// Capability: full | preview | unavailable（仅 QQ 曲目）
	Capability string `json:"capability,omitempty"`
}

// NameRef 是歌手引用。
type NameRef struct {
	Mid  string `json:"mid,omitempty"`
	Name string `json:"name"`
}
