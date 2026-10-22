package filemanager

import (
	"os/exec"
	"syscall"
)

func configureArchiveCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
