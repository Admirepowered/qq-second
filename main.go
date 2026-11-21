package main

import (
	"embed"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"qqsecond/internal/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 内存占用优化：
	// 1. GC 更积极（播放器分配率低，CPU 代价可忽略）
	debug.SetGCPercent(50)
	// 2. 软内存上限：逼近时 GC 自动加压
	debug.SetMemoryLimit(256 << 20)
	// 3. WebView2 磁盘缓存限制，避免无限膨胀；低内存设备模式压缩 GPU/瓦片缓存
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disk-cache-size=33554432 --enable-low-end-device-mode --js-flags=--max-old-space-size=192 --disable-features=msWebUILaunchFile")
	// 4. 定期把闲置堆内存还给操作系统（降低 RSS）
	go func() {
		for {
			time.Sleep(90 * time.Second)
			debug.FreeOSMemory()
		}
	}()

	// 单实例：二次启动唤起已有窗口后退出（避免双实例互踩 store.json）
	dataDir, err := app.DefaultDataDir()
	if err != nil {
		dataDir = ".qq-second-data"
	}
	release, acquired, err := app.AcquireSingleInstance(dataDir)
	if err != nil {
		println("single instance check failed:", err.Error())
	}
	if !acquired {
		_ = app.SignalRunningInstance(dataDir)
		return
	}
	defer release()

	app := app.NewApp()

	err = wails.Run(&options.App{
		Title:     "QQ Second",
		Width:     1280,
		Height:    820,
		MinWidth:  1024,
		MinHeight: 680,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if app.ServeAudio(w, r) {
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		BackgroundColour: &options.RGBA{R: 24, G: 26, B: 28, A: 255},
		OnStartup:        app.Startup,
		OnDomReady:       app.DomReady,
		OnShutdown:       app.Shutdown,
		OnBeforeClose:    app.OnBeforeClose,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
