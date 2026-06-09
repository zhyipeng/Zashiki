package filemanager

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func fileEntryFromInfo(name, path string, info os.FileInfo) FileEntry {
	mode := info.Mode()
	isSymlink := mode&os.ModeSymlink != 0
	linkTarget := ""
	if isSymlink {
		if target, err := os.Readlink(path); err == nil {
			linkTarget = target
		}
	}
	return FileEntry{
		Name:         name,
		Path:         path,
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		IsDir:        info.IsDir(),
		IsHidden:     isHiddenEntry(name, path),
		IsSymlink:    isSymlink,
		LinkTarget:   linkTarget,
		IsExecutable: isExecutableEntry(path, mode, info.IsDir()),
	}
}

func isExecutableEntry(path string, mode os.FileMode, isDir bool) bool {
	if isDir {
		return false
	}
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe", ".bat", ".cmd", ".com", ".ps1":
			return true
		default:
			return false
		}
	}
	return mode&0o111 != 0
}
