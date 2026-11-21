//go:build !windows

package app

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)


// AcquireSingleInstance 非 Windows：锁文件方式。
func AcquireSingleInstance(dataDir string) (func(), bool, error) {
	lockPath := filepath.Join(dataDir, "app.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	_ = f.Close()
	release := func() { _ = os.Remove(lockPath) }
	return release, true, nil
}

// SignalRunningInstance 非 Windows：写唤起信号文件。
func SignalRunningInstance(dataDir string) error {
	path := filepath.Join(dataDir, signalFile)
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

var _ = errors.New
