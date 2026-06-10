//go:build linux || freebsd || openbsd || netbsd

package filemanager

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func getTrashInfo() TrashInfo {
	return TrashInfo{Label: "回收站", Path: trashFilesDir(), Available: true}
}

func trashEntry(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	filesDir := trashFilesDir()
	infoDir := trashInfoDir()
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return "", err
	}
	if err := os.MkdirAll(infoDir, 0o700); err != nil {
		return "", err
	}

	target := uniqueTrashPath(filepath.Join(filesDir, filepath.Base(abs)))
	if target == "" {
		return "", fmt.Errorf("failed to create unique trash name for %q", path)
	}
	if err := os.Rename(abs, target); err != nil {
		if err := copyEntry(abs, target); err != nil {
			return "", err
		}
		if err := os.RemoveAll(abs); err != nil {
			return "", err
		}
	}
	infoPath := filepath.Join(infoDir, filepath.Base(target)+".trashinfo")
	info := "[Trash Info]\nPath=" + url.PathEscape(abs) + "\nDeletionDate=" + time.Now().Format("2006-01-02T15:04:05") + "\n"
	if err := os.WriteFile(infoPath, []byte(info), 0o600); err != nil {
		return "", err
	}
	return target, nil
}

func openTrash() error {
	return exec.Command("xdg-open", trashFilesDir()).Start()
}

func trashBaseDir() string {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "Trash")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "Trash")
	}
	return filepath.Join(home, ".local", "share", "Trash")
}

func trashFilesDir() string {
	return filepath.Join(trashBaseDir(), "files")
}

func trashInfoDir() string {
	return filepath.Join(trashBaseDir(), "info")
}
