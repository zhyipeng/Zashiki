package filemanager

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func (f *FileService) CheckConflicts(paths []string, destDir string) ([]string, error) {
	var conflicts []string
	for _, src := range paths {
		if err := validateEntryDestination(src, destDir, ""); err != nil {
			return nil, err
		}
		dst := filepath.Join(destDir, filepath.Base(src))
		if _, err := os.Stat(dst); err == nil {
			conflicts = append(conflicts, filepath.Base(src))
		}
	}
	return conflicts, nil
}

func (f *FileService) CreateFolder(parentDir string, name string) (string, error) {
	parentInfo, err := os.Stat(parentDir)
	if err != nil {
		return "", err
	}
	if !parentInfo.IsDir() {
		return "", fmt.Errorf("parent path %q is not a directory", parentDir)
	}
	if name == "" {
		name = "New Folder"
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid folder name %q", name)
	}

	path := filepath.Join(parentDir, name)
	if _, err := os.Stat(path); err == nil {
		path = uniquePath(path)
	}
	if path == "" {
		return "", fmt.Errorf("failed to create unique folder name in %q", parentDir)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func (f *FileService) CreateFolderAt(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("folder path cannot be empty")
	}
	parentDir := filepath.Dir(path)
	parentInfo, err := os.Stat(parentDir)
	if err != nil {
		return "", err
	}
	if !parentInfo.IsDir() {
		return "", fmt.Errorf("parent path %q is not a directory", parentDir)
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("destination %q already exists", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if filepath.Dir(filepath.Clean(path)) == filepath.Clean(path) {
		return "", fmt.Errorf("cannot create filesystem root %q", path)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func (f *FileService) RenameEntry(path string, name string) (FileEntry, error) {
	if name == "" {
		return FileEntry{}, fmt.Errorf("new name cannot be empty")
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		return FileEntry{}, fmt.Errorf("invalid entry name %q", name)
	}

	info, err := os.Stat(path)
	if err != nil {
		return FileEntry{}, err
	}
	if filepath.Dir(filepath.Clean(path)) == filepath.Clean(path) {
		return FileEntry{}, fmt.Errorf("cannot rename filesystem root %q", path)
	}

	dst := filepath.Join(filepath.Dir(path), name)
	if filepath.Clean(dst) == filepath.Clean(path) {
		return fileEntryFromInfo(info.Name(), path, info), nil
	}
	if _, err := os.Stat(dst); err == nil {
		return FileEntry{}, fmt.Errorf("destination %q already exists", dst)
	} else if !os.IsNotExist(err) {
		return FileEntry{}, err
	}
	if err := os.Rename(path, dst); err != nil {
		return FileEntry{}, err
	}

	newInfo, err := os.Lstat(dst)
	if err != nil {
		return FileEntry{}, err
	}
	return fileEntryFromInfo(newInfo.Name(), dst, newInfo), nil
}

func (f *FileService) DeleteEntries(paths []string) ([]string, error) {
	deleted := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := validateDeletePath(path); err != nil {
			return deleted, err
		}
		if err := os.RemoveAll(path); err != nil {
			return deleted, err
		}
		deleted = append(deleted, path)
	}
	return deleted, nil
}

func (f *FileService) DeleteEmptyFolder(path string) (string, error) {
	if err := validateDeletePath(path); err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", path)
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func (f *FileService) CopyEntries(paths []string, destDir string, conflict string) ([]EntryOperationResult, error) {
	log.Printf("CopyEntries to %s, conflict=%s, files: %+v", destDir, conflict, paths)
	results := make([]EntryOperationResult, 0, len(paths))
	for _, src := range paths {
		dst, overwritten, err := resolveDst(src, destDir, conflict)
		if err != nil {
			return results, err
		}
		if dst == "" {
			results = append(results, EntryOperationResult{
				SourcePath: src,
				Skipped:    true,
			})
			continue // skip
		}
		if err := copyEntry(src, dst); err != nil {
			return results, err
		}
		results = append(results, EntryOperationResult{
			SourcePath:  src,
			TargetPath:  dst,
			Overwritten: overwritten,
		})
	}
	return results, nil
}

func (f *FileService) MoveEntries(paths []string, destDir string, conflict string) ([]EntryOperationResult, error) {
	log.Printf("MoveEntries to %s, conflict=%s, files: %+v", destDir, conflict, paths)
	results := make([]EntryOperationResult, 0, len(paths))
	for _, src := range paths {
		dst, overwritten, err := resolveDst(src, destDir, conflict)
		if err != nil {
			return results, err
		}
		if dst == "" {
			results = append(results, EntryOperationResult{
				SourcePath: src,
				Skipped:    true,
			})
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			if err := copyEntry(src, dst); err != nil {
				return results, err
			}
			if err := os.RemoveAll(src); err != nil {
				return results, err
			}
		}
		results = append(results, EntryOperationResult{
			SourcePath:  src,
			TargetPath:  dst,
			Overwritten: overwritten,
		})
	}
	return results, nil
}

func (f *FileService) CopyEntriesToTargets(pairs []EntryPathPair) ([]EntryOperationResult, error) {
	results := make([]EntryOperationResult, 0, len(pairs))
	for _, pair := range pairs {
		if err := validateExactTarget(pair.SourcePath, pair.TargetPath); err != nil {
			return results, err
		}
		if err := copyEntry(pair.SourcePath, pair.TargetPath); err != nil {
			return results, err
		}
		results = append(results, EntryOperationResult{
			SourcePath: pair.SourcePath,
			TargetPath: pair.TargetPath,
		})
	}
	return results, nil
}

func (f *FileService) MoveEntriesToTargets(pairs []EntryPathPair) ([]EntryOperationResult, error) {
	results := make([]EntryOperationResult, 0, len(pairs))
	for _, pair := range pairs {
		if filepath.Clean(pair.SourcePath) == filepath.Clean(pair.TargetPath) {
			results = append(results, EntryOperationResult{
				SourcePath: pair.SourcePath,
				TargetPath: pair.TargetPath,
			})
			continue
		}
		if err := validateExactTarget(pair.SourcePath, pair.TargetPath); err != nil {
			return results, err
		}
		if err := os.Rename(pair.SourcePath, pair.TargetPath); err != nil {
			if err := copyEntry(pair.SourcePath, pair.TargetPath); err != nil {
				return results, err
			}
			if err := os.RemoveAll(pair.SourcePath); err != nil {
				return results, err
			}
		}
		results = append(results, EntryOperationResult{
			SourcePath: pair.SourcePath,
			TargetPath: pair.TargetPath,
		})
	}
	return results, nil
}

func resolveDst(src, destDir, conflict string) (string, bool, error) {
	if err := validateEntryDestination(src, destDir, ""); err != nil {
		return "", false, err
	}

	dst := filepath.Join(destDir, filepath.Base(src))
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		if err := validateEntryDestination(src, destDir, dst); err != nil {
			return "", false, err
		}
		return dst, false, nil
	}
	overwritten := conflict != "skip" && conflict != "rename"
	switch conflict {
	case "skip":
		return "", false, nil
	case "rename":
		dst = uniquePath(dst)
	default: // overwrite
	}
	if dst == "" {
		return "", false, nil
	}
	if err := validateEntryDestination(src, destDir, dst); err != nil {
		return "", false, err
	}
	return dst, overwritten, nil
}

func validateExactTarget(src, target string) error {
	if target == "" {
		return fmt.Errorf("target path cannot be empty")
	}
	if err := validateEntryDestination(src, filepath.Dir(target), target); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("destination %q already exists", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateEntryDestination(src, destDir, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := validateSourceDestination(src, destDir, srcInfo); err != nil {
		return err
	}
	if dst == "" {
		return nil
	}
	return validateSourceDestination(src, dst, srcInfo)
}

func validateSourceDestination(src, dest string, srcInfo os.FileInfo) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	same, child := sameOrChildPath(destAbs, srcAbs)
	if same || (srcInfo.IsDir() && child) {
		return fmt.Errorf("cannot copy or move %q into itself or its subdirectory", src)
	}
	return nil
}

func sameOrChildPath(path, parent string) (bool, bool) {
	path = filepath.Clean(path)
	parent = filepath.Clean(parent)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
		parent = strings.ToLower(parent)
	}

	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false, false
	}
	if rel == "." {
		return true, false
	}
	return false, rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validateDeletePath(path string) error {
	if path == "" {
		return fmt.Errorf("cannot delete empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	clean := filepath.Clean(abs)
	if filepath.Dir(clean) == clean {
		return fmt.Errorf("cannot delete filesystem root %q", path)
	}
	return nil
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
