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

type FilePreview struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	Content   string `json:"content"`
	DataURL   string `json:"dataUrl"`
	Truncated bool   `json:"truncated"`
	Message   string `json:"message"`
}

type RootEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FreeSpace  uint64 `json:"freeSpace"`
	TotalSpace uint64 `json:"totalSpace"`
}

type EntryOperationResult struct {
	SourcePath  string `json:"sourcePath"`
	TargetPath  string `json:"targetPath"`
	Skipped     bool   `json:"skipped"`
	Overwritten bool   `json:"overwritten"`
}

type EntryPathPair struct {
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
}
