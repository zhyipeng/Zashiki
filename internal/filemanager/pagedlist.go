package filemanager

import (
	"os"
	"path/filepath"
	"sort"
)

// 分页上限与默认值：limit 归一化为默认 500，上限 5000。
const (
	dirDefaultLimit = 500
	dirMaxLimit     = 5000
)

// dirCacheLimit 是同时缓存的目录枚举数上限。每条 dirent 仅保留
// 名称与类型（~100B/条），缓存 16 个超大目录仍远小于一次全量 JSON。
const dirCacheLimit = 16

// dirCacheEntry 缓存一次 os.ReadDir 的按名排序结果。dirents 只携带
// 名称与类型信息（不带 size/modTime），因此即使 mtime 校验偶尔错过
// 目录变更，也只会短暂影响名称集合，条目的 size/modTime 仍每页现查。
type dirCacheEntry struct {
	path    string        // filepath.Clean 后的缓存键
	dirents []os.DirEntry // 与 os.ReadDir 相同的名称/类型集合
	mtime   int64         // 目录 mtime（UnixNano），用于新鲜度校验
	size    int64         // 目录 size，防止 mtime 粒度差异导致的漏检
}

// FileService 持有分页枚举缓存。指针方法注册到 Wails，实例作为单例，带 mu 保证并发安全。
// 结构体字段定义在 fileservice.go。

// ListDirPage 返回 path 目录的分页切片与总条数。
//   - offset<0 与 limit<=0 归一化；limit 上限 5000（超界按上限截断）
//   - offset 越界时返回空 entries 与正确 total
//   - 条目在读取中途消失（Info() 失败）会被跳过但仍计入 total（与 ListDir 的 continue 语义一致）
func (f *FileService) ListDirPage(path string, offset, limit int) (DirPage, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = dirDefaultLimit
	}
	if limit > dirMaxLimit {
		limit = dirMaxLimit
	}

	dirents, err := f.loadSortedDirents(path)
	if err != nil {
		return DirPage{}, err
	}
	total := len(dirents)
	if offset >= total {
		return DirPage{Entries: make([]FileEntry, 0), Total: total}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	slice := dirents[offset:end]
	entries := make([]FileEntry, 0, len(slice))
	for _, entry := range slice {
		fe, ok := f.fileEntryFromDirEntry(path, entry)
		if ok {
			entries = append(entries, fe)
		}
	}
	return DirPage{Entries: entries, Total: total}, nil
}

// fileEntryFromDirEntry 基于单条 dirent 构建 FileEntry。
// unix 上 entry.Info() 走 fstatat(fd相对)，免全路径解析；
// Windows 上 FileInfo 直接来自读取目录时的缓冲区，零额外 syscall。
func (f *FileService) fileEntryFromDirEntry(dir string, entry os.DirEntry) (FileEntry, bool) {
	fullPath := filepath.Join(dir, entry.Name())
	info, err := entry.Info()
	if err != nil {
		return FileEntry{}, false
	}
	return fileEntryFromDirInfo(entry.Name(), fullPath, info), true
}

// loadSortedDirents 返回 path 的按名排序 dirents，并复用目录级缓存。
// 校验目录 mtime+size（1 次 os.Stat）；未命中或已变化则重新 os.ReadDir 并更新 LRU。
func (f *FileService) loadSortedDirents(path string) ([]os.DirEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	key := filepath.Clean(path)
	mtimeNs := info.ModTime().UnixNano()
	size := info.Size()

	f.mu.Lock()
	if f.dirCache != nil {
		if ce, ok := f.dirCache[key]; ok && ce.mtime == mtimeNs && ce.size == size {
			f.touchLocked(key)
			dirs := ce.dirents
			f.mu.Unlock()
			return dirs, nil
		}
	}
	f.mu.Unlock()

	dirents, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	// 与 ListDir 相同的排序语义（目录在前、其余按名），保证分页连续切片顺序一致。
	sort.Slice(dirents, func(i, j int) bool {
		if dirents[i].IsDir() != dirents[j].IsDir() {
			return dirents[i].IsDir()
		}
		return dirents[i].Name() < dirents[j].Name()
	})
	ce := &dirCacheEntry{path: key, dirents: dirents, mtime: mtimeNs, size: size}
	f.mu.Lock()
	f.putLocked(key, ce)
	f.mu.Unlock()
	return dirents, nil
}

// touchLocked 把 key 移到 LRU 末尾（最近使用）。调用方需持有 mu。
func (f *FileService) touchLocked(key string) {
	for i, k := range f.cacheOrder {
		if k == key {
			f.cacheOrder = append(f.cacheOrder[:i], f.cacheOrder[i+1:]...)
			f.cacheOrder = append(f.cacheOrder, key)
			return
		}
	}
}

// putLocked 写入（或刷新）缓存条目并维护 LRU，超出上限时淘汰最久未用。调用方需持有 mu。
func (f *FileService) putLocked(key string, ce *dirCacheEntry) {
	if f.dirCache == nil {
		f.dirCache = make(map[string]*dirCacheEntry)
	}
	if _, exists := f.dirCache[key]; exists {
		f.touchLocked(key)
	} else {
		f.cacheOrder = append(f.cacheOrder, key)
	}
	f.dirCache[key] = ce
	for len(f.cacheOrder) > dirCacheLimit {
		evict := f.cacheOrder[0]
		f.cacheOrder = f.cacheOrder[1:]
		delete(f.dirCache, evict)
	}
}
