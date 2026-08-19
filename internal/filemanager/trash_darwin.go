//go:build darwin

package filemanager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getTrashInfo() TrashInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return TrashInfo{Label: "废纸篓", Available: true}
	}
	return TrashInfo{Label: "废纸篓", Path: filepath.Join(home, ".Trash"), Available: true}
}

func trashEntry(ctx context.Context, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	trashInfo := getTrashInfo()
	if !trashInfo.Available || trashInfo.Path == "" {
		return "", fmt.Errorf("trash is not available")
	}
	if err := os.MkdirAll(trashInfo.Path, 0o700); err != nil {
		return "", err
	}
	target := uniqueTrashPath(filepath.Join(trashInfo.Path, filepath.Base(abs)))
	if target == "" {
		return "", fmt.Errorf("failed to create unique trash name for %q", path)
	}
	if err := os.Rename(abs, target); err != nil {
		if err := copyEntry(ctx, abs, target, newCancellationCleaner(), noopByteCounter{}); err != nil {
			return "", err
		}
		if info.IsDir() {
			err = os.RemoveAll(abs)
		} else {
			err = os.Remove(abs)
		}
		if err != nil {
			return "", err
		}
	}
	return target, nil
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
