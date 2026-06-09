package filemanager

import "time"

type FileEntry struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"modTime"`
	IsDir        bool      `json:"isDir"`
	IsHidden     bool      `json:"isHidden"`
	IsSymlink    bool      `json:"isSymlink"`
	LinkTarget   string    `json:"linkTarget"`
	IsExecutable bool      `json:"isExecutable"`
}

type RootEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FreeSpace  uint64 `json:"freeSpace"`
	TotalSpace uint64 `json:"totalSpace"`
}
