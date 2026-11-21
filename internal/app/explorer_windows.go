//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

// explorerOpen 用资源管理器打开目录。
func explorerOpen(dir string) *exec.Cmd {
	cmd := exec.Command("explorer", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
