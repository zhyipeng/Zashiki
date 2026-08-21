package filemanager

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxSyncIgnorePatterns caps ignore rule count to keep matching cheap and the
// stored settings file bounded.
const maxSyncIgnorePatterns = 256

const (
	SyncModeMirror      = "mirror"      // 全量 1:1，删除目标中源不存在的条目
	SyncModeIncremental = "incremental" // 仅增量，只补齐源多出的内容
)

const (
	SyncReasonNew         = "新增"
	SyncReasonSizeDiff    = "大小不同"
	SyncReasonModTimeDiff = "时间不同"
	SyncReasonHashDiff    = "内容不同"
	SyncReasonNewDir      = "新增目录"
)

const (
	SyncStatusDone      = "done"
	SyncStatusCancelled = "cancelled"
)

const (
	SyncOpAnalyze = "analyze"
	SyncOpCopy    = "copy"
	SyncOpDelete  = "delete"
	SyncOpCleanup = "cleanup"
)

// trashFn is a package-level indirection over trashEntry so tests can swap
// deletion for a local move instead of touching the real trash.
var trashFn = trashEntry

// AnalyzeSync compares source and target trees and returns the actions a
// subsequent ExecuteSync would perform, without touching the filesystem.
func (f *FileService) AnalyzeSync(ctx context.Context, config SyncConfig) (*SyncPlan, error) {
	if err := validateSyncConfig(config); err != nil {
		return nil, err
	}
	return buildSyncPlan(ctx, config)
}

// ExecuteSync performs the synchronization described by config. It re-analyzes
// the trees first so stale previews never cause unintended deletions. Items
// failing mid-run are collected into SyncResult.Errors without aborting the
// rest; only an invalid configuration or an unreachable source aborts early.
func (f *FileService) ExecuteSync(ctx context.Context, config SyncConfig) (*SyncResult, error) {
	if err := validateSyncConfig(config); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return &SyncResult{Status: SyncStatusCancelled}, nil
	}

	plan, err := buildSyncPlan(ctx, config)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{
		Status:       SyncStatusDone,
		SkippedCount: plan.SkippedCount,
		Errors:       append([]SyncItemError{}, plan.Errors...),
	}

	totalItems := len(plan.Copy) + len(plan.Delete)
	emitter := newProgressEmitter(OperationKindSync, totalItems, syncActionPaths(plan))
	if totalItems > 0 {
		emitter.scanTotalBytesAsync(ctx, syncCopySourcePaths(config, plan))
	}
	defer func() {
		if result.Status == SyncStatusCancelled {
			emitter.finish(ctx.Err(), true)
		} else {
			var finishErr error
			if len(result.Errors) > 0 {
				finishErr = fmt.Errorf("%d 项同步失败", len(result.Errors))
			}
			emitter.finish(finishErr, false)
		}
	}()

	if err := f.executeSyncCopy(ctx, config, plan, result, emitter); err != nil {
		result.Status = SyncStatusCancelled
		return result, nil
	}
	if err := f.executeSyncDelete(ctx, config, plan, result, emitter); err != nil {
		result.Status = SyncStatusCancelled
		return result, nil
	}
	f.cleanupSyncEmptyDirs(ctx, config, plan, result, emitter)
	return result, nil
}

func (f *FileService) executeSyncCopy(ctx context.Context, config SyncConfig, plan *SyncPlan, result *SyncResult, emitter *progressEmitter) error {
	if len(plan.Copy) == 0 {
		return nil
	}
	created := newCancellationCleaner()
	for _, action := range plan.Copy {
		if ctx.Err() != nil {
			created.cleanup()
			return ctx.Err()
		}
		src := filepath.Join(config.SourceDir, action.RelPath)
		dst := filepath.Join(config.TargetDir, action.RelPath)
		emitter.setCurrentName(action.RelPath)
		if action.Reason == SyncReasonNewDir {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				result.Errors = append(result.Errors, SyncItemError{RelPath: action.RelPath, Op: SyncOpCopy, Error: err.Error()})
				emitter.itemDone()
				continue
			}
			created.track(dst)
			result.Copied++
			emitter.itemDone()
			continue
		}
		if err := copySyncFile(ctx, src, dst, created, emitter); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				created.cleanup()
				return err
			}
			result.Errors = append(result.Errors, SyncItemError{RelPath: action.RelPath, Op: SyncOpCopy, Error: err.Error()})
			emitter.itemDone()
			continue
		}
		result.Copied++
		emitter.itemDone()
	}
	return nil
}

func (f *FileService) executeSyncDelete(ctx context.Context, config SyncConfig, plan *SyncPlan, result *SyncResult, emitter *progressEmitter) error {
	if len(plan.Delete) == 0 {
		return nil
	}
	for _, action := range plan.Delete {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		path := filepath.Join(config.TargetDir, action.RelPath)
		emitter.setCurrentName(action.RelPath)
		if _, err := trashFn(ctx, path); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			result.Errors = append(result.Errors, SyncItemError{RelPath: action.RelPath, Op: SyncOpDelete, Error: err.Error()})
			emitter.itemDone()
			continue
		}
		result.Deleted++
		emitter.itemDone()
	}
	return nil
}

// cleanupSyncEmptyDirs removes target directories that the plan emptied by
// deleting their contents. Only directories below the target root are ever
// removed, and only when they no longer hold any entry.
func (f *FileService) cleanupSyncEmptyDirs(ctx context.Context, config SyncConfig, plan *SyncPlan, result *SyncResult, emitter *progressEmitter) {
	var dirs []string
	for _, action := range plan.Delete {
		if action.IsDir {
			dirs = append(dirs, filepath.Join(config.TargetDir, action.RelPath))
		}
	}
	// Deepest first so children are checked before their parents.
	sortDescByDepth(dirs)
	for _, dir := range dirs {
		if ctx.Err() != nil {
			return
		}
		if err := os.Remove(dir); err == nil {
			result.RemovedDirs++
		}
		// ENOTEMPTY and friends mean the directory still holds entries
		// (e.g. protected by ignore rules); leaving it in place is correct.
	}
}

func sortDescByDepth(paths []string) {
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0; j-- {
			if depth(paths[j]) <= depth(paths[j-1]) {
				break
			}
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
}

func depth(path string) int {
	return strings.Count(filepath.ToSlash(filepath.Clean(path)), "/")
}

// copySyncFile copies a single file and preserves mode and modification time
// so size/mtime based re-analysis after the sync stays stable.
func copySyncFile(ctx context.Context, src, dst string, created *cancellationCleaner, emitter *progressEmitter) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source %q is a directory", src)
	}
	if err := copyFile(ctx, src, dst, created, emitter); err != nil {
		return err
	}
	if err := os.Chmod(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

// buildSyncPlan walks both trees and derives copy/delete actions.
func buildSyncPlan(ctx context.Context, config SyncConfig) (*SyncPlan, error) {
	plan := &SyncPlan{}
	sourceEntries := scanSyncTree(ctx, config, config.SourceDir, "source", plan)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	targetEntries := scanSyncTree(ctx, config, config.TargetDir, "target", plan)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	plan.Copy = make([]SyncCopyAction, 0)
	plan.Delete = make([]SyncDeleteAction, 0)

	for _, src := range sourceEntries {
		dst, exists := targetEntries[src.relPath]
		if !exists {
			if src.isDir {
				plan.Copy = append(plan.Copy, SyncCopyAction{RelPath: src.relPath, Reason: SyncReasonNewDir})
			} else {
				plan.Copy = append(plan.Copy, SyncCopyAction{RelPath: src.relPath, Reason: SyncReasonNew})
			}
			continue
		}
		if src.isDir != dst.isDir {
			// Type conflict (file vs directory at the same relative path):
			// report it as an error; a destructive resolve is unsafe.
			plan.Errors = append(plan.Errors, SyncItemError{
				RelPath: src.relPath,
				Op:      SyncOpAnalyze,
				Error:   "源与目标同一路径类型不一致（文件/目录冲突）",
			})
			continue
		}
		if src.isDir {
			continue
		}
		reason, same := compareSyncFiles(ctx, config, src, dst)
		if same {
			plan.SkippedCount++
			continue
		}
		plan.Copy = append(plan.Copy, SyncCopyAction{RelPath: src.relPath, Reason: reason})
	}

	if config.Mode == SyncModeMirror {
		for _, dst := range targetEntries {
			if _, exists := sourceEntries[dst.relPath]; exists {
				continue
			}
			if !topmostSyncDelete(targetEntries, dst.relPath) {
				continue
			}
			plan.Delete = append(plan.Delete, SyncDeleteAction{RelPath: dst.relPath, IsDir: dst.isDir})
		}
	}

	return plan, nil
}

// topmostSyncDelete reports whether relPath is the highest ancestor (including
// itself) missing from the source, so a whole orphaned subtree is deleted as a
// single trash action instead of one per nested entry.
func topmostSyncDelete(entries map[string]syncTreeEntry, relPath string) bool {
	for parent := filepath.Dir(relPath); parent != "."; parent = filepath.Dir(parent) {
		if _, exists := entries[parent]; exists {
			return false
		}
	}
	return true
}

type syncTreeEntry struct {
	relPath string
	absPath string
	size    int64
	modTime time.Time
	isDir   bool
}

// scanSyncTree walks one side of the sync pair. It records visible entries
// keyed by slash-separated path relative to root, and appends traversal
// problems to plan.Errors. A missing root yields an empty map, which makes a
// not-yet-existing target behave like an empty tree.
func scanSyncTree(ctx context.Context, config SyncConfig, root, side string, plan *SyncPlan) map[string]syncTreeEntry {
	entries := make(map[string]syncTreeEntry)
	rootInfo, err := os.Lstat(root)
	if err != nil {
		if !os.IsNotExist(err) {
			plan.Errors = append(plan.Errors, SyncItemError{
				RelPath: "",
				Op:      SyncOpAnalyze,
				Error:   fmt.Sprintf("无法读取%s目录 %q: %v", side, root, err),
			})
		}
		return entries
	}
	if !rootInfo.IsDir() {
		plan.Errors = append(plan.Errors, SyncItemError{
			RelPath: "",
			Op:      SyncOpAnalyze,
			Error:   fmt.Sprintf("%s路径 %q 不是目录", side, root),
		})
		return entries
	}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if path == root {
			return nil
		}
		if err != nil {
			plan.Errors = append(plan.Errors, SyncItemError{
				RelPath: syncRelPath(root, path),
				Op:      SyncOpAnalyze,
				Error:   fmt.Sprintf("遍历失败: %v", err),
			})
			return nil // unreadable entries are skipped, siblings keep walking
		}
		relPath := syncRelPath(root, path)
		if relPath == "" {
			return nil
		}
		if syncIgnored(config, d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // symlinks do not participate in v1
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			plan.Errors = append(plan.Errors, SyncItemError{
				RelPath: relPath,
				Op:      SyncOpAnalyze,
				Error:   fmt.Sprintf("读取信息失败: %v", infoErr),
			})
			return nil
		}
		entries[relPath] = syncTreeEntry{
			relPath: relPath,
			absPath: path,
			size:    info.Size(),
			modTime: info.ModTime(),
			isDir:   info.IsDir(),
		}
		return nil
	})
	return entries
}

// syncRelPath converts an absolute path under root into a slash-separated
// relative path usable as a cross-platform map key.
func syncRelPath(root, path string) string {
	relPath, err := filepath.Rel(root, path)
	if err != nil || relPath == "." {
		return ""
	}
	return filepath.ToSlash(relPath)
}

// syncIgnored reports whether an entry is excluded by the config's hidden-file
// switch or ignore patterns. Patterns match the entry name at any depth.
func syncIgnored(config SyncConfig, name string) bool {
	if config.IgnoreHidden && isHiddenEntry(name, "") {
		return true
	}
	for _, pattern := range config.IgnorePatterns {
		if matched, err := filepath.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

// compareSyncFiles decides whether source and target files are identical
// according to the enabled dimensions, cheapest first. It returns a human
// readable reason for the difference when they are not.
func compareSyncFiles(ctx context.Context, config SyncConfig, src, dst syncTreeEntry) (string, bool) {
	if config.CompareSize {
		if src.size != dst.size {
			return SyncReasonSizeDiff, false
		}
	}
	if config.CompareModTime {
		if !src.modTime.Equal(dst.modTime) {
			return SyncReasonModTimeDiff, false
		}
	}
	if config.CompareHash {
		if config.CompareSize && src.size != dst.size {
			return SyncReasonSizeDiff, false // already detected above; unreachable guard
		}
		same, err := filesEqualByHash(ctx, src.absPath, dst.absPath)
		if err == nil && !same {
			return SyncReasonHashDiff, false
		}
	}
	return "", true
}

// filesEqualByHash streams both files through SHA-256 and compares digests,
// so arbitrary file sizes compare in constant memory.
func filesEqualByHash(ctx context.Context, a, b string) (bool, error) {
	hashA, err := fileSHA256(ctx, a)
	if err != nil {
		return false, err
	}
	hashB, err := fileSHA256(ctx, b)
	if err != nil {
		return false, err
	}
	return string(hashA) == string(hashB), nil
}

func fileSHA256(ctx context.Context, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	hasher := sha256.New()
	buf := make([]byte, copyBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, writeErr := hasher.Write(buf[:n]); writeErr != nil {
				return nil, writeErr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	return hasher.Sum(nil), nil
}

func validateSyncConfig(config SyncConfig) error {
	if config.SourceDir == "" {
		return fmt.Errorf("source directory cannot be empty")
	}
	if config.TargetDir == "" {
		return fmt.Errorf("target directory cannot be empty")
	}
	if config.Mode != SyncModeMirror && config.Mode != SyncModeIncremental {
		return fmt.Errorf("invalid sync mode %q", config.Mode)
	}
	if !config.CompareSize && !config.CompareModTime && !config.CompareHash {
		return fmt.Errorf("at least one comparison dimension (size, modTime, hash) must be enabled")
	}
	if len(config.IgnorePatterns) > maxSyncIgnorePatterns {
		return fmt.Errorf("too many ignore patterns (max %d)", maxSyncIgnorePatterns)
	}

	srcAbs, err := filepath.Abs(config.SourceDir)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(config.TargetDir)
	if err != nil {
		return err
	}
	srcAbs = filepath.Clean(srcAbs)
	dstAbs = filepath.Clean(dstAbs)

	same, child := sameOrChildPath(dstAbs, srcAbs)
	if same || child {
		return fmt.Errorf("target directory %q must not be the source or inside it", config.TargetDir)
	}
	same, child = sameOrChildPath(srcAbs, dstAbs)
	if same || child {
		return fmt.Errorf("source directory %q must not be the target or inside it", config.SourceDir)
	}
	if filepath.Dir(dstAbs) == dstAbs {
		return fmt.Errorf("target directory cannot be the filesystem root")
	}

	srcInfo, err := os.Stat(config.SourceDir)
	if err != nil {
		return fmt.Errorf("source directory is not accessible: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source path %q is not a directory", config.SourceDir)
	}
	if dstInfo, err := os.Lstat(config.TargetDir); err == nil && !dstInfo.IsDir() {
		return fmt.Errorf("target path %q exists but is not a directory", config.TargetDir)
	}
	return nil
}

// syncActionPaths lists action relative paths for the initial progress event.
func syncActionPaths(plan *SyncPlan) []string {
	paths := make([]string, 0, len(plan.Copy)+len(plan.Delete))
	for _, action := range plan.Copy {
		paths = append(paths, action.RelPath)
	}
	for _, action := range plan.Delete {
		paths = append(paths, action.RelPath)
	}
	return paths
}

// syncCopySourcePaths materializes copy action source paths for the byte scan.
func syncCopySourcePaths(config SyncConfig, plan *SyncPlan) []string {
	paths := make([]string, 0, len(plan.Copy))
	for _, action := range plan.Copy {
		if action.Reason == SyncReasonNewDir {
			continue
		}
		paths = append(paths, filepath.Join(config.SourceDir, action.RelPath))
	}
	return paths
}
