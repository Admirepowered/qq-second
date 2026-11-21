//go:build windows

package app

import (
	"os"
	"os/exec"
	"syscall"
)

// relaunch 启动指定可执行文件（隐藏控制台窗口）。
func relaunch(exe string) error {
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Env = os.Environ()
	return cmd.Start()
}
