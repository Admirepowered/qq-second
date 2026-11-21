package app

// 播放会话快照：独立文件 session.json（不与 settings 共写 store.json，
// 避免播放中每 2 秒的快照把并发修改的设置覆盖掉——多实例/高频写的历史坑）。

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// sessionPath 返回 session.json 路径。
func (a *App) sessionPath() string {
	return filepath.Join(a.dataDir, "session.json")
}

// saveSessionFile 写入会话快照（原子替换）。
func (a *App) saveSessionFile(s SessionData) {
	if s.Queue == nil {
		s.Queue = []Track{}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	tmp := a.sessionPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, a.sessionPath())
}

// loadSessionFile 读取会话快照；不存在或损坏返回零值。
func (a *App) loadSessionFile() SessionData {
	data, err := os.ReadFile(a.sessionPath())
	if err != nil {
		return SessionData{Volume: 0.85, Order: "sequential", Repeat: "off"}
	}
	var s SessionData
	if err := json.Unmarshal(data, &s); err != nil {
		return SessionData{Volume: 0.85, Order: "sequential", Repeat: "off"}
	}
	if s.Queue == nil {
		s.Queue = []Track{}
	}
	if s.Volume <= 0 {
		s.Volume = 0.85
	}
	return s
}
