//go:build windows

package app

// 开机自启：HKCU\...\Run 注册表键（无需管理员权限）。

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValueName = "QQSecond"

// SetAutoStart 设置或取消开机自启。
func (a *App) SetAutoStart(enabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()

	if !enabled {
		return key.DeleteValue(runValueName)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	return key.SetStringValue(runValueName, `"`+exe+`"`)
}

// GetAutoStart 查询当前是否开机自启。
func (a *App) GetAutoStart() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer key.Close()
	val, _, err := key.GetStringValue(runValueName)
	if err != nil {
		return false, nil // 键不存在 = 未启用
	}
	return val != "", nil
}
