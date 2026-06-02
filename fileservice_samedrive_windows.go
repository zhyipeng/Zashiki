//go:build windows

package main

import (
	"path/filepath"
	"strings"
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
	volume := filepath.VolumeName(path)
	if volume == "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return ""
		}
		volume = filepath.VolumeName(abs)
	}
	return strings.ToUpper(volume)
}
