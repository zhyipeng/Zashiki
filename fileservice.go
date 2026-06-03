package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

type FileEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"modTime"`
	IsDir    bool      `json:"isDir"`
	IsHidden bool      `json:"isHidden"`
}

type RootEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	FreeSpace  uint64 `json:"freeSpace"`
	TotalSpace uint64 `json:"totalSpace"`
}

type FileService struct{}

func (f *FileService) ListDir(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(path, entry.Name())
		result = append(result, FileEntry{
			Name:     entry.Name(),
			Path:     fullPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			IsDir:    entry.IsDir(),
			IsHidden: isHiddenEntry(entry.Name(), fullPath),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func (f *FileService) OpenFile(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "linux":
		return exec.Command("xdg-open", path).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	default:
		return exec.Command("open", path).Start()
	}
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
	info, err := os.Stat(path)
	if err != nil {
		return FileEntry{}, err
	}
	return FileEntry{
		Name:     info.Name(),
		Path:     path,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		IsDir:    info.IsDir(),
		IsHidden: isHiddenEntry(info.Name(), path),
	}, nil
}

func (f *FileService) IsSameDrive(path1, path2 string) bool {
	return isSameDrive(path1, path2)
}

func (f *FileService) CheckConflicts(paths []string, destDir string) ([]string, error) {
	var conflicts []string
	for _, src := range paths {
		dst := filepath.Join(destDir, filepath.Base(src))
		if _, err := os.Stat(dst); err == nil {
			conflicts = append(conflicts, filepath.Base(src))
		}
	}
	return conflicts, nil
}

func (f *FileService) CopyEntries(paths []string, destDir string, conflict string) error {
	log.Printf("CopyEntries to %s, conflict=%s, files: %+v", destDir, conflict, paths)
	for _, src := range paths {
		dst := resolveDst(src, destDir, conflict)
		if dst == "" {
			continue // skip
		}
		if err := copyEntry(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func (f *FileService) MoveEntries(paths []string, destDir string, conflict string) error {
	log.Printf("MoveEntries to %s, conflict=%s, files: %+v", destDir, conflict, paths)
	for _, src := range paths {
		dst := resolveDst(src, destDir, conflict)
		if dst == "" {
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			if err := copyEntry(src, dst); err != nil {
				return err
			}
			os.RemoveAll(src)
		}
	}
	return nil
}

func resolveDst(src, destDir, conflict string) string {
	dst := filepath.Join(destDir, filepath.Base(src))
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}
	switch conflict {
	case "skip":
		return ""
	case "rename":
		return uniquePath(dst)
	default: // overwrite
		return dst
	}
}

func uniquePath(path string) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return ""
}

func copyEntry(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstF.Close()

	if _, err := dstF.ReadFrom(srcF); err != nil {
		return err
	}
	return dstF.Close()
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := copyEntry(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}
