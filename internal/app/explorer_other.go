//go:build !windows

package app

import "os/exec"

func explorerOpen(dir string) *exec.Cmd {
	return exec.Command("xdg-open", dir)
}
