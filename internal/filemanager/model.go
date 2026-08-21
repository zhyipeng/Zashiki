package filemanager

import "time"

// DirPage 是 ListDirPage 的分页返回模型：entries 为单个分页切片，
// total 为目录条目总数（分页期间条目消失仍计入，与 ListDir 的 continue 语义一致）。
type DirPage struct {
	Entries []FileEntry `json:"entries"`
	Total   int         `json:"total"`
}

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
	Version   string `json:"version"`
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

type SyncConfig struct {
	SourceDir      string   `json:"sourceDir"`
	TargetDir      string   `json:"targetDir"`
	Mode           string   `json:"mode"`           // "mirror" | "incremental"
	CompareSize    bool     `json:"compareSize"`    // 判重维度：文件大小
	CompareModTime bool     `json:"compareModTime"` // 判重维度：修改时间
	CompareHash    bool     `json:"compareHash"`    // 判重维度：SHA-256 内容哈希
	IgnoreHidden   bool     `json:"ignoreHidden"`   // 忽略隐藏文件
	IgnorePatterns []string `json:"ignorePatterns"` // 名字精确匹配 + filepath.Match 通配符
}

type SyncCopyAction struct {
	RelPath string `json:"relPath"`
	Reason  string `json:"reason"` // 新增 | 大小不同 | 时间不同 | 内容不同 | 新增目录
}

type SyncDeleteAction struct {
	RelPath string `json:"relPath"`
	IsDir   bool   `json:"isDir"`
}

type SyncItemError struct {
	RelPath string `json:"relPath"`
	Op      string `json:"op"` // analyze | copy | delete | cleanup
	Error   string `json:"error"`
}

type SyncPlan struct {
	Copy         []SyncCopyAction   `json:"copy"`
	Delete       []SyncDeleteAction `json:"delete"`
	SkippedCount int                `json:"skippedCount"`
	Errors       []SyncItemError    `json:"errors"`
}

type SyncResult struct {
	Status       string          `json:"status"` // "done" | "cancelled"
	Copied       int             `json:"copied"`
	Deleted      int             `json:"deleted"`
	RemovedDirs  int             `json:"removedDirs"`
	SkippedCount int             `json:"skippedCount"`
	Errors       []SyncItemError `json:"errors"`
}
