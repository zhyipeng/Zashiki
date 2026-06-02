//go:build !windows

package main

import "syscall"

func isSameDrive(path1, path2 string) bool {
	var stat1, stat2 syscall.Stat_t
	if syscall.Stat(path1, &stat1) != nil || syscall.Stat(path2, &stat2) != nil {
		return false
	}
	return stat1.Dev == stat2.Dev
}
