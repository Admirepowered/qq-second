package app

// 应用内自动更新（Windows）：从 Release 下载对应版本 exe 并替换重启。
// macOS：仅下载 zip 包（应用内自动替换需要签名，手动安装）。

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const updateProgressEvent = "update:progress"

// assetName 返回当前平台对应的 Release 资产文件名。
func assetName(ver string) string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("QQSecond-windows-amd64-%s.exe", ver)
	case "darwin":
		return fmt.Sprintf("QQSecond-macos-arm64-%s.zip", ver)
	}
	return ""
}

// DownloadUpdate 下载新版本到 dataDir/update/，通过事件上报进度。
func (a *App) DownloadUpdate() (*DownloadedUpdate, error) {
	a.mu.Lock()
	proxyStr := strings.TrimSpace(a.store.Settings.UpdateProxy)
	a.mu.Unlock()

	// 通过 lastGoodChannel 的通道下载
	client, err := a.githubClientForDownload(proxyStr)
	if err != nil {
		return nil, err
	}

	if a.updateInfo == nil || a.updateInfo.AssetURL == "" {
		return nil, fmt.Errorf("没有可下载的更新")
	}

	dir := filepath.Join(a.dataDir, "update")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	ext := ".exe"
	if runtime.GOOS == "darwin" {
		ext = ".zip"
	}
	target := filepath.Join(dir, "QQSecond-new"+ext)

	req, err := http.NewRequest(http.MethodGet, a.updateInfo.AssetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "QQSecond-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download http %d", resp.StatusCode)
	}

	total := resp.ContentLength
	out, err := os.Create(target + ".part")
	if err != nil {
		return nil, err
	}
	written := int64(0)
	lastPct := -1
	buf := make([]byte, 128*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return nil, werr
			}
			written += int64(n)
			if total > 0 {
				pct := int(written * 100 / total)
				if pct != lastPct {
					lastPct = pct
					if a.ctx != nil {
						wailsruntime.EventsEmit(a.ctx, updateProgressEvent, pct)
					}
				}
			}
		}
		if readErr != nil {
			break // EOF
		}
	}
	out.Close()
	_ = os.Rename(target+".part", target)

	return &DownloadedUpdate{Path: target, Size: written}, nil
}

// DownloadedUpdate 下载结果。
type DownloadedUpdate struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// ApplyUpdate Windows：替换可执行文件并重启应用。
func (a *App) ApplyUpdate() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("macOS 请下载 zip 手动安装")
	}
	ext := ".exe"
	if runtime.GOOS == "darwin" {
		ext = ".zip"
	}
	newExe := filepath.Join(a.dataDir, "update", "QQSecond-new"+ext)
	if _, err := os.Stat(newExe); err != nil {
		return fmt.Errorf("请先下载更新")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("rename current: %w", err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		_ = os.Rename(old, exe) // 回滚
		return fmt.Errorf("install new: %w", err)
	}
	// 启动新版本（隐藏控制台）
	if err := relaunch(exe); err != nil {
		return fmt.Errorf("relaunch: %w", err)
	}
	// 延迟退出，给新进程留出启动时间
	go func() {
		time.Sleep(800 * time.Millisecond)
		a.QuitNow()
	}()
	return nil
}

// githubClientForDownload 更新下载专用 client（走手动代理优先，否则直连）。
func (a *App) githubClientForDownload(manualProxy string) (*http.Client, error) {
	if manualProxy != "" {
		if c, err := a.githubClient("proxy"); err == nil {
			return c, nil
		}
	}
	if good, ok := lastGoodChannel.Load().(string); ok {
		if c, err := a.githubClient(good); err == nil {
			return c, nil
		}
	}
	return &http.Client{Timeout: 0}, nil
}
