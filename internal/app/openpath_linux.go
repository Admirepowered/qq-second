//go:build linux

package app

import "os/exec"

func openPathOS(path string) error {
	return exec.Command("xdg-open", path).Start()
}
