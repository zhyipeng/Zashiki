package filemanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
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

func (f *FileService) DeleteEntries(ctx context.Context, paths []string) ([]string, error) {
	emitter := newProgressEmitter(OperationKindDelete, len(paths), paths)
	deleted := make([]string, 0, len(paths))
	for _, path := range paths {
		if ctx.Err() != nil {
			emitter.finish(ctx.Err(), true)
			return deleted, ctx.Err()
		}
		if err := validateDeletePath(path); err != nil {
			emitter.finish(err, false)
			return deleted, err
		}
		emitter.setCurrentName(filepath.Base(path))
		if err := os.RemoveAll(path); err != nil {
			emitter.finish(err, false)
			return deleted, err
		}
		deleted = append(deleted, path)
		emitter.itemDone()
	}
	emitter.finish(nil, false)
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

func (f *FileService) CopyEntries(ctx context.Context, paths []string, destDir string, conflict string) ([]EntryOperationResult, error) {
	log.Printf("CopyEntries to %s, conflict=%s, files: %+v", destDir, conflict, paths)
	emitter := newProgressEmitter(OperationKindCopy, len(paths), paths)
	emitter.scanTotalBytesAsync(ctx, paths)
	created := newCancellationCleaner()
	results := make([]EntryOperationResult, 0, len(paths))
	for _, src := range paths {
		if ctx.Err() != nil {
			created.cleanup()
			emitter.finish(ctx.Err(), true)
			return results, ctx.Err()
		}
		dst, overwritten, err := resolveDst(src, destDir, conflict)
		if err != nil {
			created.cleanup()
			emitter.finish(err, false)
			return results, err
		}
		if dst == "" {
			results = append(results, EntryOperationResult{
				SourcePath: src,
				Skipped:    true,
			})
			emitter.itemDone()
			continue // skip
		}
		emitter.setCurrentName(filepath.Base(src))
		if err := copyEntry(ctx, src, dst, created, emitter); err != nil {
			created.cleanup()
			emitter.finish(err, errors.Is(err, context.Canceled))
			return results, err
		}
		results = append(results, EntryOperationResult{
			SourcePath:  src,
			TargetPath:  dst,
			Overwritten: overwritten,
		})
		emitter.itemDone()
	}
	emitter.finish(nil, false)
	return results, nil
}

func (f *FileService) MoveEntries(ctx context.Context, paths []string, destDir string, conflict string) ([]EntryOperationResult, error) {
	log.Printf("MoveEntries to %s, conflict=%s, files: %+v", destDir, conflict, paths)
	emitter := newProgressEmitter(OperationKindMove, len(paths), paths)
	emitter.scanTotalBytesAsync(ctx, paths)
	created := newCancellationCleaner()
	results := make([]EntryOperationResult, 0, len(paths))
	for _, src := range paths {
		if ctx.Err() != nil {
			created.cleanup()
			emitter.finish(ctx.Err(), true)
			return results, ctx.Err()
		}
		dst, overwritten, err := resolveDst(src, destDir, conflict)
		if err != nil {
			created.cleanup()
			emitter.finish(err, false)
			return results, err
		}
		if dst == "" {
			results = append(results, EntryOperationResult{
				SourcePath: src,
				Skipped:    true,
			})
			emitter.itemDone()
			continue
		}
		emitter.setCurrentName(filepath.Base(src))
		if err := os.Rename(src, dst); err != nil {
			// Cross-device fallback: copy then remove the source.
			if err := copyEntry(ctx, src, dst, created, emitter); err != nil {
				created.cleanup()
				emitter.finish(err, errors.Is(err, context.Canceled))
				return results, err
			}
			if err := os.RemoveAll(src); err != nil {
				created.cleanup()
				emitter.finish(err, false)
				return results, err
			}
		}
		results = append(results, EntryOperationResult{
			SourcePath:  src,
			TargetPath:  dst,
			Overwritten: overwritten,
		})
		emitter.itemDone()
	}
	emitter.finish(nil, false)
	return results, nil
}

func (f *FileService) CopyEntriesToTargets(ctx context.Context, pairs []EntryPathPair) ([]EntryOperationResult, error) {
	emitter := newProgressEmitter(OperationKindCopy, len(pairs), pairSources(pairs))
	created := newCancellationCleaner()
	results := make([]EntryOperationResult, 0, len(pairs))
	for _, pair := range pairs {
		if ctx.Err() != nil {
			created.cleanup()
			emitter.finish(ctx.Err(), true)
			return results, ctx.Err()
		}
		if err := validateExactTarget(pair.SourcePath, pair.TargetPath); err != nil {
			created.cleanup()
			emitter.finish(err, false)
			return results, err
		}
		emitter.setCurrentName(filepath.Base(pair.SourcePath))
		if err := copyEntry(ctx, pair.SourcePath, pair.TargetPath, created, emitter); err != nil {
			created.cleanup()
			emitter.finish(err, errors.Is(err, context.Canceled))
			return results, err
		}
		results = append(results, EntryOperationResult{
			SourcePath: pair.SourcePath,
			TargetPath: pair.TargetPath,
		})
		emitter.itemDone()
	}
	emitter.finish(nil, false)
	return results, nil
}

func (f *FileService) MoveEntriesToTargets(ctx context.Context, pairs []EntryPathPair) ([]EntryOperationResult, error) {
	emitter := newProgressEmitter(OperationKindMove, len(pairs), pairSources(pairs))
	created := newCancellationCleaner()
	results := make([]EntryOperationResult, 0, len(pairs))
	for _, pair := range pairs {
		if ctx.Err() != nil {
			created.cleanup()
			emitter.finish(ctx.Err(), true)
			return results, ctx.Err()
		}
		if filepath.Clean(pair.SourcePath) == filepath.Clean(pair.TargetPath) {
			results = append(results, EntryOperationResult{
				SourcePath: pair.SourcePath,
				TargetPath: pair.TargetPath,
			})
			emitter.itemDone()
			continue
		}
		if err := validateExactTarget(pair.SourcePath, pair.TargetPath); err != nil {
			created.cleanup()
			emitter.finish(err, false)
			return results, err
		}
		emitter.setCurrentName(filepath.Base(pair.SourcePath))
		if err := os.Rename(pair.SourcePath, pair.TargetPath); err != nil {
			// Cross-device fallback: copy then remove the source.
			if err := copyEntry(ctx, pair.SourcePath, pair.TargetPath, created, emitter); err != nil {
				created.cleanup()
				emitter.finish(err, errors.Is(err, context.Canceled))
				return results, err
			}
			if err := os.RemoveAll(pair.SourcePath); err != nil {
				created.cleanup()
				emitter.finish(err, false)
				return results, err
			}
		}
		results = append(results, EntryOperationResult{
			SourcePath: pair.SourcePath,
			TargetPath: pair.TargetPath,
		})
		emitter.itemDone()
	}
	emitter.finish(nil, false)
	return results, nil
}

func pairSources(pairs []EntryPathPair) []string {
	sources := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		sources = append(sources, pair.SourcePath)
	}
	return sources
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

// cancellationCleaner remembers destinations created by the current batch so
// a cancelled operation can remove them, restoring the "nothing happened"
// semantics. Overwritten destinations cannot be restored and are not tracked.
type cancellationCleaner struct {
	mu    sync.Mutex
	paths []string
}

func newCancellationCleaner() *cancellationCleaner {
	return &cancellationCleaner{}
}

func (c *cancellationCleaner) track(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths = append(c.paths, path)
}

func (c *cancellationCleaner) cleanup() {
	c.mu.Lock()
	paths := c.paths
	c.paths = nil
	c.mu.Unlock()
	// Remove deepest paths first so tracked parents are emptied before removal.
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, path := range paths {
		_ = os.RemoveAll(path)
	}
}

// byteCounter receives copied-byte ticks for progress reporting.
type byteCounter interface {
	bytesCopied(n int64)
}

// noopByteCounter discards byte ticks for call sites without progress needs.
type noopByteCounter struct{}

func (noopByteCounter) bytesCopied(int64) {}

func copyEntry(ctx context.Context, src, dst string, created *cancellationCleaner, counter byteCounter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return copyDir(ctx, src, dst, created, counter)
	}
	return copyFile(ctx, src, dst, created, counter)
}

const copyBufferSize = 256 * 1024

// cancelCheckWriter wraps the destination file so each Write first checks ctx
// cancellation and reports the copied byte count to the progress counter. It is
// used with io.CopyBuffer to keep cancellation and byte-accounting semantics
// identical to the previous read/write loop while delegating chunking to io.CopyBuffer.
type cancelCheckWriter struct {
	ctx     context.Context
	counter byteCounter
	dst     io.Writer
}

// Write reports copied bytes to the counter and returns a context error as soon
// as the caller's context is cancelled, preserving cancellable copy semantics.
func (w *cancelCheckWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.dst.Write(p)
	if n > 0 {
		w.counter.bytesCopied(int64(n))
	}
	return n, err
}

func copyFile(ctx context.Context, src, dst string, created *cancellationCleaner, counter byteCounter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	// Only track the destination for cleanup when we are about to create it;
	// overwriting an existing file must not be rolled back.
	dstExisted := false
	if _, err := os.Lstat(dst); err == nil {
		dstExisted = true
	} else if !os.IsNotExist(err) {
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
	if !dstExisted {
		created.track(dst)
	}

	// io.CopyBuffer reads from srcF and writes through cancelCheckWriter, which
	// checks ctx cancellation before each write and accumulates real copied bytes
	// into the counter. Partial writes are handled internally; io.EOF ends the loop.
	buf := make([]byte, copyBufferSize)
	_, copyErr := io.CopyBuffer(&cancelCheckWriter{ctx: ctx, counter: counter, dst: dstF}, srcF, buf)
	if copyErr != nil {
		dstF.Close()
		return copyErr
	}
	return dstF.Close()
}

func copyDir(ctx context.Context, src, dst string, created *cancellationCleaner, counter byteCounter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	dstExisted := false
	if _, err := os.Lstat(dst); err == nil {
		dstExisted = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	if !dstExisted {
		created.track(dst)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := copyEntry(ctx, srcPath, dstPath, created, counter); err != nil {
			return err
		}
	}
	return nil
}
