//go:build !windows

package main

import "syscall"

func getRoots() []RootEntry {
	freeSpace, totalSpace := rootSpace("/")
	return []RootEntry{{
		Name:       "/",
		Path:       "/",
		FreeSpace:  freeSpace,
		TotalSpace: totalSpace,
	}}
}

func rootSpace(path string) (uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	return stat.Bavail * uint64(stat.Bsize), stat.Blocks * uint64(stat.Bsize)
}
