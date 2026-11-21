package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings 是全部持久化的用户偏好，与前端 prefsStore 一一对应（camelCase JSON）。
type Settings struct {
	ColorMode        string   `json:"colorMode"`        // system | light | dark
	Palette          string   `json:"palette"`          // 预设 id 或 custom
	PrimaryColor     string   `json:"primaryColor"`     // #RRGGBB
	BackgroundMode   string   `json:"backgroundMode"`   // default | image | artwork | color
	BackgroundColor  string   `json:"backgroundColor"`  // #RRGGBB
	BackgroundImage  string   `json:"backgroundImage"`  // backgrounds/ 目录中的文件 id
	BackgroundFit    string   `json:"backgroundFit"`    // cover | contain
	BackgroundBlur   int      `json:"backgroundBlur"`   // 0-40 px
	ArtworkInfluence int      `json:"artworkInfluence"` // 0-100 封面/图片对背景的影响度
	SurfaceOpacity   int      `json:"surfaceOpacity"`   // 40-100 界面板材不透明度
	PreferredQuality string   `json:"preferredQuality"` // standard | high | lossless
	CloseBehavior    string   `json:"closeBehavior"`    // hide | exit（点关闭按钮时隐藏到托盘或退出）
	LocalFolders     []string `json:"localFolders"`
}

func defaultSettings() Settings {
	return Settings{
		ColorMode:        "dark",
		Palette:          "default",
		PrimaryColor:     "#31c27c",
		BackgroundMode:   "default",
		BackgroundColor:  "#1d1f21",
		BackgroundFit:    "cover",
		BackgroundBlur:   24,
		ArtworkInfluence: 60,
		SurfaceOpacity:   88,
		PreferredQuality: "high",
		CloseBehavior:    "hide",
		LocalFolders:     []string{},
	}
}

// Store 是 store.json 的整体内容。
type Store struct {
	Settings  Settings    `json:"settings"`
	Credential Credential  `json:"credential"`
	Session    SessionData `json:"session"`
}

// QQ 扫码登录凭据。只保存在本机配置文件里。
type Credential struct {
	Uin        string `json:"uin"`
	QQMusicKey string `json:"qqMusicKey"`
	Nickname   string `json:"nickname"`
	LoginType  int    `json:"loginType"`
	LoggedAt   int64  `json:"loggedAt"`
}

// SessionData 是播放器状态快照（队列、进度等），启动时恢复。
type SessionData struct {
	Queue        []Track `json:"queue"`
	CurrentIndex int     `json:"currentIndex"`
	PositionMs   int     `json:"positionMs"`
	Volume       float64 `json:"volume"`
	Order        string  `json:"order"`  // sequential | shuffle
	Repeat       string  `json:"repeat"` // off | all | one
}

func clampSettings(s Settings) Settings {
	def := defaultSettings()
	if s.ColorMode != "light" && s.ColorMode != "dark" && s.ColorMode != "system" {
		s.ColorMode = def.ColorMode
	}
	if s.PrimaryColor == "" {
		s.PrimaryColor = def.PrimaryColor
	}
	switch s.BackgroundMode {
	case "default", "image", "artwork", "color":
	default:
		s.BackgroundMode = def.BackgroundMode
	}
	if s.BackgroundFit != "cover" && s.BackgroundFit != "contain" {
		s.BackgroundFit = def.BackgroundFit
	}
	s.BackgroundBlur = clampInt(s.BackgroundBlur, 0, 40)
	s.ArtworkInfluence = clampInt(s.ArtworkInfluence, 0, 100)
	s.SurfaceOpacity = clampInt(s.SurfaceOpacity, 40, 100)
	switch s.PreferredQuality {
	case "standard", "high", "lossless":
	default:
		s.PreferredQuality = def.PreferredQuality
	}
	if s.CloseBehavior != "hide" && s.CloseBehavior != "exit" {
		s.CloseBehavior = "hide"
	}
	if s.LocalFolders == nil {
		s.LocalFolders = []string{}
	}
	return s
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (a *App) initStore() error {
	path := a.storePath
	data, err := os.ReadFile(path)
	store := Store{}
	if err == nil {
		if jsonErr := json.Unmarshal(data, &store); jsonErr != nil {
			// 损坏的配置文件不致命，回退到默认值。
			println("store.json parse failed, using defaults:", jsonErr.Error())
		}
	}
	store.Settings = clampSettings(store.Settings)
	if store.Session.Queue == nil {
		store.Session.Queue = []Track{}
	}
	if store.Settings.LocalFolders == nil {
		store.Settings.LocalFolders = []string{}
	}
	a.store = store
	return a.saveStoreLocked()
}

func (a *App) saveStoreLocked() error {
	data, err := json.MarshalIndent(a.store, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.storePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.storePath)
}

// GetSettings 返回当前设置副本。
func (a *App) GetSettings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.store.Settings
}

// SaveSettings 全量保存设置；LocalFolders 由专门的绑定管理，这里保持原值。
func (a *App) SaveSettings(settings Settings) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	folders := a.store.Settings.LocalFolders
	settings.LocalFolders = folders
	a.store.Settings = clampSettings(settings)
	return a.saveStoreLocked()
}

func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "qq-second"), nil
}
