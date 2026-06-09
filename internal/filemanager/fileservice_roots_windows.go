//go:build windows

package filemanager

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func getRoots() []RootEntry {
	roots := make([]RootEntry, 0, 4)
	for drive := 'A'; drive <= 'Z'; drive++ {
		path := fmt.Sprintf("%c:\\", drive)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			freeSpace, totalSpace := rootSpace(path)
			roots = append(roots, RootEntry{
				Name:       rootName(path, drive),
				Path:       path,
				FreeSpace:  freeSpace,
				TotalSpace: totalSpace,
			})
		}
	}
	return roots
}

func rootName(path string, drive rune) string {
	driveName := fmt.Sprintf("%c:", drive)
	volumeName, err := volumeLabel(path)
	if err != nil || volumeName == "" {
		return driveName
	}
	return fmt.Sprintf("%s (%s)", volumeName, driveName)
}

func volumeLabel(path string) (string, error) {
	rootPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	var volumeName [windows.MAX_PATH + 1]uint16
	err = windows.GetVolumeInformation(
		rootPath,
		&volumeName[0],
		uint32(len(volumeName)),
		nil,
		nil,
		nil,
		nil,
		0,
	)
	if err != nil {
		return "", err
	}
	return windows.UTF16ToString(volumeName[:]), nil
}

func rootSpace(path string) (uint64, uint64) {
	rootPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}

	var freeBytesAvailable uint64
	var totalBytes uint64
	if err := windows.GetDiskFreeSpaceEx(rootPath, &freeBytesAvailable, &totalBytes, nil); err != nil {
		return 0, 0
	}
	return freeBytesAvailable, totalBytes
}
