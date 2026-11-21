//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"time"
)


// AcquireSingleInstance 非 Windows：锁文件方式。
// 任何错误一律放行（fail-open）：CI 上绑定生成会执行 main()，用户目录可能不存在；
// 锁失效的代价远小于应用无法启动/构建失败。
func AcquireSingleInstance(dataDir string) (func(), bool, error) {
	if os.Getenv("CI") != "" {
		return func() {}, true, nil
	}
	lockPath := filepath.Join(dataDir, "app.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return func() {}, true, nil // fail-open
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, false, nil
		}
		return func() {}, true, nil // fail-open
	}
	_ = f.Close()
	release := func() { _ = os.Remove(lockPath) }
	return release, true, nil
}

// SignalRunningInstance 非 Windows：写唤起信号文件（目录不存在时静默忽略）。
func SignalRunningInstance(dataDir string) error {
	path := filepath.Join(dataDir, signalFile)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString("show")
	return f.Close()
}

// WatchSignalFile 监听唤起信号文件。
func (a *App) WatchSignalFile() {
	path := filepath.Join(a.dataDir, signalFile)
	var lastMod int64
	if st, err := os.Stat(path); err == nil {
		lastMod = st.ModTime().UnixNano()
	}
	go func() {
		for {
			time.Sleep(time.Second)
			st, err := os.Stat(path)
			if err != nil {
				continue
			}
			mod := st.ModTime().UnixNano()
			if mod != lastMod {
				lastMod = mod
				a.ShowWindow()
			}
		}
	}()
}
