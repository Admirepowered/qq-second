//go:build windows

package app

import "os/exec"

// openPathOS Windows: explorer 打开目录。
// 注意：绝不能加 SysProcAttr HideWindow——它会把弹出的文件夹窗口也隐藏掉
// （历史坑：按钮"无效"的根因）。
func openPathOS(path string) error {
	return exec.Command("explorer", path).Start()
}
