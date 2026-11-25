//go:build !windows

package app

// ShowMainWindowNative 非 Windows 平台无此兜底。
func ShowMainWindowNative() bool { return false }