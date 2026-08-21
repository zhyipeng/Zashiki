//go:build windows

package filemanager

import (
	"os"
	"syscall"
)

func isHiddenEntry(_ string, path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return false
	}
	return attrs&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}

// isHiddenInfo 读取目录缓冲区自带的 FILE_ATTRIBUTE_HIDDEN（零 syscall）；
// 退化时回退到 isHiddenEntry。
func isHiddenInfo(info os.FileInfo, _ string, path string) bool {
	if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return d.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
	}
	return isHiddenEntry("", path)
}
