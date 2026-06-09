//go:build darwin

package filemanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getTrashInfo() TrashInfo {
	return TrashInfo{Label: "废纸篓", Available: true}
}

func trashEntry(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	script := `tell application "Finder" to delete POSIX file "` + escapeAppleScriptString(abs) + `"`
	return exec.Command("osascript", "-e", script).Run()
}

func openTrash() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	trashPath := filepath.Join(home, ".Trash")
	if err := exec.Command("open", trashPath).Run(); err == nil {
		return nil
	}
	script := `tell application "Finder" to open POSIX file "` + escapeAppleScriptString(trashPath) + `"`
	if err := exec.Command("osascript", "-e", script).Run(); err == nil {
		return nil
	}
	return fmt.Errorf("failed to open Trash in Finder")
}

func escapeAppleScriptString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
