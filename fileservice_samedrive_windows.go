//go:build windows

package main

import (
	"strings"
	"syscall"
)

func isSameDrive(path1, path2 string) bool {
	vol1 := getVolume(path1)
	vol2 := getVolume(path2)
	if vol1 == "" || vol2 == "" {
		return false
	}
	return vol1 == vol2
}

func getVolume(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		return strings.ToUpper(path[:2])
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	var serial uint32
	buf := make([]uint16, syscall.MAX_PATH)
	if err := syscall.GetVolumeInformation(p, &buf[0], uint32(len(buf)), &serial, nil, nil, nil, 0); err != nil {
		return ""
	}
	return string(syscall.UTF16ToString(buf))
}
