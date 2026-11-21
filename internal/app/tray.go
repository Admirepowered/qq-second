package app

// 系统托盘：菜单（显示/播放控制/退出）+ 双击唤起主窗口。

import (
	"fmt"

	"qqsecond/internal/tray"
	"qqsecond/internal/version"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// 托盘菜单 ID。
const (
	trayMenuShow    = 1
	trayMenuToggle  = 2
	trayMenuPrev    = 3
	trayMenuNext    = 4
	trayMenuLyrics  = 5
	trayMenuRefresh = 6
	trayMenuQuit    = 9
)

var appTray *tray.Tray

// startTray 创建托盘图标（Windows 下立即可见）。
func (a *App) startTray() {
	if !tray.Supported() || a.ctx == nil {
		return
	}
	t, err := tray.New(tray.Options{
		Tooltip: tray.Tooltip("QQ Second " + version.Version, "本地 + QQ 音乐播放器"),
		OnMenu:  a.onTrayMenu,
		OnDoubleClick: func() {
			a.ShowWindow()
		},
		OnQuit: func() {
			// Shell 请求移除图标（如 explorer 重启），静默移除即可；
			// 下次启动会重建。
		},
		Menu: a.trayMenu,
	})
	if err != nil {
		fmt.Println("tray init failed:", err)
		return
	}
	appTray = t
}

func (a *App) trayMenu() []tray.MenuItem {
	items := []tray.MenuItem{
		{ID: trayMenuShow, Label: "显示主窗口"},
		{Separator: true},
		{ID: trayMenuToggle, Label: "播放 / 暂停"},
		{ID: trayMenuPrev, Label: "上一首"},
		{ID: trayMenuNext, Label: "下一首"},
		{ID: trayMenuLyrics, Label: "歌词面板"},
		{Separator: true},
		{ID: trayMenuRefresh, Label: "重新扫描本地音乐"},
		{Separator: true},
		{ID: trayMenuQuit, Label: "退出"},
	}
	return items
}

func (a *App) onTrayMenu(id int) {
	if a.ctx == nil {
		return
	}
	switch id {
	case trayMenuShow:
		a.ShowWindow()
	case trayMenuToggle:
		wailsruntime.EventsEmit(a.ctx, "tray:playback", "toggle")
	case trayMenuPrev:
		wailsruntime.EventsEmit(a.ctx, "tray:playback", "previous")
	case trayMenuNext:
		wailsruntime.EventsEmit(a.ctx, "tray:playback", "next")
	case trayMenuLyrics:
		wailsruntime.EventsEmit(a.ctx, "tray:playback", "lyrics")
	case trayMenuRefresh:
		a.RescanLocalLibrary()
		wailsruntime.EventsEmit(a.ctx, "tray:library-rescanned")
	case trayMenuQuit:
		a.QuitNow()
	}
}

// stopTray 移除托盘图标。
func (a *App) stopTray() {
	if appTray != nil {
		_ = appTray.Remove()
		appTray = nil
	}
}
