//go:build !windows

package filemanager

import "os"

func isHiddenEntry(name string, _ string) bool {
	if name == "." || name == ".." {
		return false
	}
	return len(name) > 0 && name[0] == '.'
}

// isHiddenInfo 在 unix 上直接用名称前缀判断，不依赖 info 的 Sys()，零 syscall。
func isHiddenInfo(_ os.FileInfo, name string, _ string) bool {
	return isHiddenEntry(name, "")
}
