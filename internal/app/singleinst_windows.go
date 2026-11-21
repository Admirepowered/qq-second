//go:build windows

package app

// 单实例：命名互斥量；二次启动时向已运行实例发送唤起信号后退出。

import (
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)


var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutexW = kernel32.NewProc("CreateMutexW")
	procReleaseMutex = kernel32.NewProc("ReleaseMutex")
)

// AcquireSingleInstance 尝试获取单实例锁。
// 返回 (release func, acquired bool)：acquired=false 表示已有实例在运行，
// 调用方应写唤起信号文件后直接退出。
// CI 环境（绑定生成会执行 main()）直接跳过。
func AcquireSingleInstance(dataDir string) (func(), bool, error) {
	if os.Getenv("CI") != "" {
		return func() {}, true, nil
	}
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, false, err
	}
	handle, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if callErr == syscall.ERROR_ALREADY_EXISTS {
		return nil, false, nil
	}
	if handle == 0 {
		return nil, false, callErr
	}
	release := func() {
		procReleaseMutex.Call(handle)
	}
	return release, true, nil
}

// SignalRunningInstance 向已运行实例写入唤起信号文件。
func SignalRunningInstance(dataDir string) error {
	path := filepath.Join(dataDir, signalFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString("show")
	return f.Close()
}

// WatchSignalFile 监听唤起信号文件（mtime 变化 → 回调）。
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
