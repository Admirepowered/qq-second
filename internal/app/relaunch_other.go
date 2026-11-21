//go:build !windows

package app

import "os/exec"

// relaunch 启动指定可执行文件。
func relaunch(exe string) error {
	return exec.Command(exe).Start()
}
