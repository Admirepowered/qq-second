//go:build !windows

package app

// 非 Windows 平台暂不支持注册表自启（占位实现）。

func (a *App) SetAutoStart(enabled bool) error {
	return nil
}

func (a *App) GetAutoStart() (bool, error) {
	return false, nil
}
