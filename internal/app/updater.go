package app

// 应用内自动更新（Windows）：从 Release 下载对应版本 exe 并替换重启。
// macOS：仅下载 zip 包（应用内自动替换需要签名，手动安装）。

import (
	"fmt"
	"io"
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
//
// 关键：更新包下载在 %APPDATA%（通常是 C:），而程序可能装在别的盘（如 E:\Tools\...）。
// 跨盘 os.Rename 会直接失败（"cannot move the file to a different disk drive"），
// 所以先把新文件复制到**程序所在目录**（同盘），再把当前 exe 改名成 .old，
// 最后把同盘的暂存文件改名为主程序并重启。
func (a *App) ApplyUpdate() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("macOS 请下载 zip 手动安装")
	}
	ext := ".exe"
	newExe := filepath.Join(a.dataDir, "update", "QQSecond-new"+ext)
	if _, err := os.Stat(newExe); err != nil {
		return fmt.Errorf("请先下载更新")
	}
	if err := sanityCheckExe(newExe); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := installUpdate(exe, newExe); err != nil {
		return err
	}

	// 启动新版本（隐藏控制台）
	if err := relaunch(exe); err != nil {
		return fmt.Errorf("重启失败: %w", err)
	}
	// 延迟退出，给新进程留出启动时间
	go func() {
		time.Sleep(800 * time.Millisecond)
		a.QuitNow()
	}()
	return nil
}

// installUpdate 把下载好的新程序安装到 exePath（可单测，不含重启逻辑）。
func installUpdate(exePath, downloadedPath string) error {
	if err := sanityCheckExe(downloadedPath); err != nil {
		return err
	}
	exeDir := filepath.Dir(exePath)

	// 1) 复制到与目标同盘的暂存文件（跨盘时这一步就是关键）
	staged := filepath.Join(exeDir, ".qqsecond-update.tmp")
	if err := copyFile(downloadedPath, staged); err != nil {
		return fmt.Errorf("写入程序目录失败（可能需要管理员权限，可手动下载安装）: %w", err)
	}

	// 2) 旧程序改名（Windows 允许重命名正在运行的 exe；用时间戳避免残留冲突）
	oldExe := fmt.Sprintf("%s.old.%d", exePath, time.Now().Unix())
	if err := os.Rename(exePath, oldExe); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("重命名当前程序失败（可能需要管理员权限，可手动下载安装）: %w", err)
	}

	// 3) 同盘改名成主程序；失败则回滚
	if err := os.Rename(staged, exePath); err != nil {
		_ = os.Rename(oldExe, exePath)
		_ = os.Remove(staged)
		return fmt.Errorf("安装新版本失败: %w", err)
	}
	return nil
}

// sanityCheckExe 确认下载的是可执行文件而不是错误页/半截文件。
func sanityCheckExe(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Size() < 1<<20 {
		return fmt.Errorf("更新包异常（只有 %d 字节），请重新下载", st.Size())
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var head [2]byte
	if _, err := f.Read(head[:]); err != nil {
		return err
	}
	if head[0] != 'M' || head[1] != 'Z' {
		return fmt.Errorf("更新包不是有效的 Windows 可执行文件，请重新下载")
	}
	return nil
}

// copyFile 复制文件（先写 .part 再改名，避免半截文件）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
}

// cleanupUpdateLeftovers 清理上一次更新留下的 .old / 暂存文件。
// 旧进程若还在运行会锁住 .old，忽略错误即可（下次启动再清）。
func cleanupUpdateLeftovers() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, base+".old") ||
			name == ".qqsecond-update.tmp" ||
			name == ".qqsecond-update.tmp.part" {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
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
