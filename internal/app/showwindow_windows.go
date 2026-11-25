//go:build windows

package app

// 兜底显示主窗口（Win32 直调）。
//
// 背景：个别机器上 WebView2 的首帧就绪回调会丢失，Wails 自己不会执行「显示窗口」，
// 结果进程和前端日志都正常但看不到窗口。`wailsruntime.WindowShow` 依赖 Wails 内部
// 的 hasStarted 状态，这种情况下可能无效；这里直接枚举本进程的 wailsWindow 窗口并
// ShowWindow，确定性强，也不反复抢焦点。

import (
	"syscall"
	"unsafe"
)

var (
	modUser32Show           = syscall.NewLazyDLL("user32.dll")
	procEnumWindows         = modUser32Show.NewProc("EnumWindows")
	procGetWindowPID        = modUser32Show.NewProc("GetWindowThreadProcessId")
	procGetClassName        = modUser32Show.NewProc("GetClassNameW")
	procIsVisible           = modUser32Show.NewProc("IsWindowVisible")
	procShowWindow          = modUser32Show.NewProc("ShowWindow")
	procSetForegroundWindow = modUser32Show.NewProc("SetForegroundWindow")
	procGetCurrentPID       = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentProcessId")
)

const (
	swShow    = 5
	swRestore = 9
)

// ShowMainWindowNative 找到本进程的 wailsWindow 并显示；返回是否成功。
func ShowMainWindowNative() bool {
	pid, _, _ := procGetCurrentPID.Call()
	target := uint32(pid)
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var winPID uint32
		procGetWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&winPID)))
		if winPID != target {
			return 1 // continue
		}
		var buf [64]uint16
		n, _, _ := procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 {
			return 1
		}
		name := syscall.UTF16ToString(buf[:n])
		// Wails 主窗口类名固定为 wailsWindow（托盘窗口是 QQSecondTrayWindow）
		if name == "wailsWindow" {
			found = hwnd
			return 0 // stop
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	if found == 0 {
		return false
	}
	visible, _, _ := procIsVisible.Call(found)
	if visible == 0 {
		procShowWindow.Call(found, swRestore)
		procShowWindow.Call(found, swShow)
	}
	procSetForegroundWindow.Call(found)
	return true
}
