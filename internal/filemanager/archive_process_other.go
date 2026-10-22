//go:build !windows

package filemanager

import "os/exec"

func configureArchiveCommand(cmd *exec.Cmd) {}
