//go:build darwin

package app

import "os/exec"

func openPathOS(path string) error {
	return exec.Command("open", path).Start()
}
