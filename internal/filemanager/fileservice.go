package filemanager

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// FileService 是文件系统服务入口。mu/cacheOrder 供 ListDirPage 的分页枚举缓存使用。
type FileService struct {
	mu         sync.Mutex
	dirCache   map[string]*dirCacheEntry
	cacheOrder []string // LRU 序，最近使用在末尾
}

func (f *FileService) ListDir(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		result = append(result, fileEntryFromInfo(entry.Name(), fullPath, info))
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func (f *FileService) GetHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		roots := getRoots()
		if len(roots) > 0 {
			return roots[0].Path
		}
		return string(filepath.Separator)
	}
	return home
}

func (f *FileService) GetSeparator() string {
	return string(filepath.Separator)
}

func (f *FileService) GetRoots() []RootEntry {
	return getRoots()
}

func (f *FileService) GetFileInfo(path string) (FileEntry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return FileEntry{}, err
	}
	return fileEntryFromInfo(info.Name(), path, info), nil
}

func (f *FileService) IsSameDrive(path1, path2 string) bool {
	return isSameDrive(path1, path2)
}
