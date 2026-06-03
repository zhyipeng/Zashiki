//go:build !windows

package main

import (
	"os"
	"syscall"
)

func getRoots() []RootEntry {
	freeSpace, totalSpace := rootSpace("/")
	roots := []RootEntry{{
		Name:       "/",
		Path:       "/",
		FreeSpace:  freeSpace,
		TotalSpace: totalSpace,
	}}

	home, err := os.UserHomeDir()
	if err == nil && home != "" && home != "/" {
		freeSpace, totalSpace = rootSpace(home)
		roots = append(roots, RootEntry{
			Name:       "Home",
			Path:       home,
			FreeSpace:  freeSpace,
			TotalSpace: totalSpace,
		})
	}

	return roots
}

func rootSpace(path string) (uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	return stat.Bavail * uint64(stat.Bsize), stat.Blocks * uint64(stat.Bsize)
}
