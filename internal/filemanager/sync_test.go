package filemanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// syncTestEnv prepares isolated source/target directories and a trash stub
// that moves deleted paths into a fake trash dir instead of the real one.
type syncTestEnv struct {
	source  string
	target  string
	trash   string
	trashed []string
}

func newSyncTestEnv(t *testing.T) *syncTestEnv {
	t.Helper()
	base := t.TempDir()
	env := &syncTestEnv{
		source: filepath.Join(base, "source"),
		target: filepath.Join(base, "target"),
		trash:  filepath.Join(base, "fake-trash"),
	}
	if err := os.MkdirAll(env.source, 0o755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.MkdirAll(env.trash, 0o755); err != nil {
		t.Fatalf("MkdirAll(trash) error = %v", err)
	}

	originalTrashFn := trashFn
	trashFn = func(ctx context.Context, path string) (string, error) {
		env.trashed = append(env.trashed, path)
		target := filepath.Join(env.trash, filepath.Base(path))
		if err := os.Rename(path, target); err != nil {
			return "", err
		}
		return target, nil
	}
	t.Cleanup(func() { trashFn = originalTrashFn })
	return env
}

func (e *syncTestEnv) writeFile(t *testing.T, relPath, content string) {
	t.Helper()
	path := filepath.Join(e.source, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", relPath, err)
	}
}

func (e *syncTestEnv) writeTargetFile(t *testing.T, relPath, content string) {
	t.Helper()
	path := filepath.Join(e.target, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", relPath, err)
	}
}

func (e *syncTestEnv) setSourceModTime(t *testing.T, relPath string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(e.source, filepath.FromSlash(relPath))
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%s) error = %v", relPath, err)
	}
}

func baseSyncConfig(env *syncTestEnv) SyncConfig {
	return SyncConfig{
		SourceDir:      env.source,
		TargetDir:      env.target,
		Mode:           SyncModeIncremental,
		CompareSize:    true,
		CompareModTime: true,
	}
}

func copyReasons(plan *SyncPlan) []string {
	reasons := make([]string, 0, len(plan.Copy))
	for _, action := range plan.Copy {
		reasons = append(reasons, fmt.Sprintf("%s:%s", action.RelPath, action.Reason))
	}
	sort.Strings(reasons)
	return reasons
}

func deletePaths(plan *SyncPlan) []string {
	paths := make([]string, 0, len(plan.Delete))
	for _, action := range plan.Delete {
		paths = append(paths, action.RelPath)
	}
	sort.Strings(paths)
	return paths
}

func TestAnalyzeSyncEmptyTarget(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "hello")
	env.writeFile(t, "dir/b.txt", "world")

	plan, err := (&FileService{}).AnalyzeSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}

	got := copyReasons(plan)
	want := []string{"a.txt:新增", "dir:新增目录", "dir/b.txt:新增"}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("Copy actions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Copy actions = %v, want %v", got, want)
		}
	}
	if len(plan.Delete) != 0 {
		t.Fatalf("Delete actions = %v, want none", plan.Delete)
	}
	if plan.SkippedCount != 0 {
		t.Fatalf("SkippedCount = %d, want 0", plan.SkippedCount)
	}
}

func TestAnalyzeSyncMissingTargetBehavesEmpty(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "hello")
	config := baseSyncConfig(env)
	config.TargetDir = filepath.Join(env.source, "..", "not-created-target")

	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Copy) != 1 || plan.Copy[0].RelPath != "a.txt" {
		t.Fatalf("Copy actions = %v, want a.txt only", plan.Copy)
	}
}

func TestAnalyzeSyncComparisonDimensions(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "same.txt", "same")
	env.writeFile(t, "size.txt", "12345")
	env.writeFile(t, "time.txt", "content")
	env.writeTargetFile(t, "same.txt", "same")
	env.writeTargetFile(t, "size.txt", "1234")
	env.writeTargetFile(t, "time.txt", "content")

	// Align mtimes for same.txt and size.txt so only the intended dimension differs.
	alignedTime := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	env.setSourceModTime(t, "same.txt", alignedTime)
	if err := os.Chtimes(filepath.Join(env.target, "same.txt"), alignedTime, alignedTime); err != nil {
		t.Fatalf("Chtimes error = %v", err)
	}
	env.setSourceModTime(t, "size.txt", alignedTime)
	if err := os.Chtimes(filepath.Join(env.target, "size.txt"), alignedTime, alignedTime); err != nil {
		t.Fatalf("Chtimes error = %v", err)
	}
	// time.txt keeps identical content/size but a different mtime.
	env.setSourceModTime(t, "time.txt", time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := os.Chtimes(filepath.Join(env.target, "time.txt"), time.Date(2032, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2032, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Chtimes error = %v", err)
	}

	// size + mtime dimensions: same.txt skipped, others copied.
	config := baseSyncConfig(env)
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if plan.SkippedCount != 1 {
		t.Fatalf("SkippedCount = %d, want 1", plan.SkippedCount)
	}
	if len(plan.Copy) != 2 {
		t.Fatalf("Copy actions = %v, want size.txt + time.txt", plan.Copy)
	}
	reasons := map[string]string{}
	for _, action := range plan.Copy {
		reasons[action.RelPath] = action.Reason
	}
	if reasons["size.txt"] != SyncReasonSizeDiff {
		t.Fatalf("size.txt reason = %q, want %q", reasons["size.txt"], SyncReasonSizeDiff)
	}
	if reasons["time.txt"] != SyncReasonModTimeDiff {
		t.Fatalf("time.txt reason = %q, want %q", reasons["time.txt"], SyncReasonModTimeDiff)
	}

	// Only size dimension: same.txt and time.txt skipped, size.txt copied.
	config = SyncConfig{SourceDir: env.source, TargetDir: env.target, Mode: SyncModeIncremental, CompareSize: true}
	plan, err = (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if plan.SkippedCount != 2 {
		t.Fatalf("SkippedCount = %d, want 2", plan.SkippedCount)
	}
	if len(plan.Copy) != 1 || plan.Copy[0].RelPath != "size.txt" || plan.Copy[0].Reason != SyncReasonSizeDiff {
		t.Fatalf("Copy actions = %v, want size.txt:大小不同", plan.Copy)
	}
}

func TestAnalyzeSyncHashDimension(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "same-length-different-content")
	env.writeTargetFile(t, "a.txt", "same-length-different-XXXXXXXX")
	env.setSourceModTime(t, "a.txt", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := os.Chtimes(filepath.Join(env.target, "a.txt"), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Chtimes error = %v", err)
	}

	config := SyncConfig{SourceDir: env.source, TargetDir: env.target, Mode: SyncModeIncremental, CompareHash: true}
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Copy) != 1 || plan.Copy[0].Reason != SyncReasonHashDiff {
		t.Fatalf("Copy actions = %v, want a.txt:内容不同", plan.Copy)
	}

	// Same content but only hash enabled: skipped.
	env.writeTargetFile(t, "a.txt", "same-length-different-content")
	if err := os.Chtimes(filepath.Join(env.target, "a.txt"), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Chtimes error = %v", err)
	}
	plan, err = (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Copy) != 0 || plan.SkippedCount != 1 {
		t.Fatalf("Copy = %v skipped = %d, want skip only", plan.Copy, plan.SkippedCount)
	}
}

func TestAnalyzeSyncIgnoreRules(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "keep.txt", "a")
	env.writeFile(t, "node_modules/pkg/index.js", "x")
	env.writeFile(t, "temp.log", "x")
	env.writeFile(t, "temp.tmp", "x")
	env.writeFile(t, "normal.d", "x")

	config := baseSyncConfig(env)
	config.IgnorePatterns = []string{"node_modules", "*.log", "*.tmp"}
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	got := copyReasons(plan)
	want := []string{"keep.txt:新增", "normal.d:新增"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Copy actions = %v, want %v", got, want)
	}
}

func TestAnalyzeSyncIgnoreHidden(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, ".hidden.txt", "x")
	env.writeFile(t, "visible.txt", "x")
	env.writeFile(t, ".config/nested.txt", "x")

	config := baseSyncConfig(env)
	config.IgnoreHidden = true
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	got := copyReasons(plan)
	if len(got) != 1 || got[0] != "visible.txt:新增" {
		t.Fatalf("Copy actions = %v, want visible.txt only (hidden pruned)", got)
	}
}

func TestAnalyzeSyncIgnoreProtectsTarget(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "keep.txt", "a")
	env.writeTargetFile(t, "keep.txt", "a")
	env.writeTargetFile(t, "node_modules/pkg/index.js", "x")
	// Align mtime so keep.txt counts as skipped.
	targetStat, err := os.Stat(filepath.Join(env.target, "keep.txt"))
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	env.setSourceModTime(t, "keep.txt", targetStat.ModTime())

	config := baseSyncConfig(env)
	config.Mode = SyncModeMirror
	config.IgnorePatterns = []string{"node_modules"}
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Delete) != 0 {
		t.Fatalf("Delete actions = %v, want none (ignored target entries protected)", plan.Delete)
	}
}

func TestAnalyzeSyncMirrorDeletesOrphans(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "keep.txt", "a")
	env.writeTargetFile(t, "keep.txt", "a")
	env.writeTargetFile(t, "orphan.txt", "x")
	env.writeTargetFile(t, "olddir/nested/deep.txt", "x")
	env.writeTargetFile(t, "olddir/root.txt", "x")
	targetStat, err := os.Stat(filepath.Join(env.target, "keep.txt"))
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	env.setSourceModTime(t, "keep.txt", targetStat.ModTime())

	config := baseSyncConfig(env)
	config.Mode = SyncModeMirror
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}

	got := deletePaths(plan)
	want := []string{"olddir", "orphan.txt"}
	if len(got) != len(want) {
		t.Fatalf("Delete actions = %v, want %v (subtree collapsed to topmost dir)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Delete actions = %v, want %v", got, want)
		}
	}

	// Incremental mode must not produce deletes.
	config.Mode = SyncModeIncremental
	plan, err = (&FileService{}).AnalyzeSync(context.Background(), config)
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Delete) != 0 {
		t.Fatalf("Delete actions = %v, want none in incremental mode", plan.Delete)
	}
}

func TestAnalyzeSyncTypeConflictReported(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "conflict", "file content")
	if err := os.MkdirAll(filepath.Join(env.target, "conflict"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	plan, err := (&FileService{}).AnalyzeSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Errors) == 0 {
		t.Fatalf("plan.Errors empty, want type conflict entry")
	}
	if len(plan.Copy) != 0 || len(plan.Delete) != 0 {
		t.Fatalf("Copy = %v Delete = %v, want none on conflict", plan.Copy, plan.Delete)
	}
}

func TestAnalyzeSyncSkipsSymlinks(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "real.txt", "a")
	if err := os.Symlink("real.txt", filepath.Join(env.source, "link.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	plan, err := (&FileService{}).AnalyzeSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	for _, action := range plan.Copy {
		if action.RelPath == "link.txt" {
			t.Fatalf("symlink leaked into copy plan: %v", plan.Copy)
		}
	}
}

func TestAnalyzeSyncInvalidConfigs(t *testing.T) {
	env := newSyncTestEnv(t)
	service := &FileService{}
	ctx := context.Background()

	cases := []struct {
		name   string
		config SyncConfig
	}{
		{"empty source", SyncConfig{TargetDir: env.target, Mode: SyncModeIncremental, CompareSize: true}},
		{"empty target", SyncConfig{SourceDir: env.source, Mode: SyncModeIncremental, CompareSize: true}},
		{"bad mode", SyncConfig{SourceDir: env.source, TargetDir: env.target, Mode: "bogus", CompareSize: true}},
		{"no dimension", SyncConfig{SourceDir: env.source, TargetDir: env.target, Mode: SyncModeIncremental}},
		{"same dir", SyncConfig{SourceDir: env.source, TargetDir: env.source, Mode: SyncModeIncremental, CompareSize: true}},
		{"target inside source", SyncConfig{SourceDir: env.source, TargetDir: filepath.Join(env.source, "sub"), Mode: SyncModeIncremental, CompareSize: true}},
		{"source inside target", SyncConfig{SourceDir: filepath.Join(env.target, "sub"), TargetDir: env.target, Mode: SyncModeIncremental, CompareSize: true}},
		{"target is root", SyncConfig{SourceDir: env.source, TargetDir: filepath.VolumeName(env.source) + string(filepath.Separator), Mode: SyncModeIncremental, CompareSize: true}},
		{"source missing", SyncConfig{SourceDir: filepath.Join(env.source, "missing"), TargetDir: env.target, Mode: SyncModeIncremental, CompareSize: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := service.AnalyzeSync(ctx, tc.config); err == nil {
				t.Fatalf("AnalyzeSync(%s) expected error, got nil", tc.name)
			}
		})
	}
}

func TestExecuteSyncIncrementalCreatesTarget(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "hello")
	env.writeFile(t, "dir/b.txt", "world")
	config := baseSyncConfig(env)

	result, err := (&FileService{}).ExecuteSync(context.Background(), config)
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}
	if result.Status != SyncStatusDone {
		t.Fatalf("Status = %q, want done", result.Status)
	}
	if result.Copied != 3 { // a.txt + dir + dir/b.txt
		t.Fatalf("Copied = %d, want 3", result.Copied)
	}
	for _, rel := range []string{"a.txt", "dir/b.txt"} {
		data, err := os.ReadFile(filepath.Join(env.target, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", rel, err)
		}
		expected := "hello"
		if rel == "dir/b.txt" {
			expected = "world"
		}
		if string(data) != expected {
			t.Fatalf("%s content = %q, want %q", rel, data, expected)
		}
	}
}

func TestExecuteSyncPreservesModTime(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "hello")
	modTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.Local)
	env.setSourceModTime(t, "a.txt", modTime)

	result, err := (&FileService{}).ExecuteSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}
	if result.Status != SyncStatusDone {
		t.Fatalf("Status = %q, want done", result.Status)
	}

	info, err := os.Stat(filepath.Join(env.target, "a.txt"))
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	if !info.ModTime().Equal(modTime) {
		t.Fatalf("target mtime = %v, want %v", info.ModTime(), modTime)
	}

	// Idempotence: re-analysis with mtime+size dimensions must be a no-op.
	plan, err := (&FileService{}).AnalyzeSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("AnalyzeSync() error = %v", err)
	}
	if len(plan.Copy) != 0 || len(plan.Delete) != 0 {
		t.Fatalf("second analysis Copy = %v Delete = %v, want empty (idempotent)", plan.Copy, plan.Delete)
	}
}

func TestExecuteSyncMirrorTrashesOrphans(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "keep.txt", "same")
	env.writeTargetFile(t, "keep.txt", "same")
	env.writeTargetFile(t, "orphan.txt", "x")
	env.writeTargetFile(t, "olddir/nested.txt", "x")
	targetStat, err := os.Stat(filepath.Join(env.target, "keep.txt"))
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	env.setSourceModTime(t, "keep.txt", targetStat.ModTime())

	config := baseSyncConfig(env)
	config.Mode = SyncModeMirror
	result, err := (&FileService{}).ExecuteSync(context.Background(), config)
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}
	if result.Status != SyncStatusDone {
		t.Fatalf("Status = %q, want done", result.Status)
	}
	if result.Deleted != 2 {
		t.Fatalf("Deleted = %d, want 2 (orphan.txt + olddir subtree)", result.Deleted)
	}
	if result.RemovedDirs != 0 {
		t.Fatalf("RemovedDirs = %d, want 0 (olddir removed as subtree by trash)", result.RemovedDirs)
	}

	if _, err := os.Stat(filepath.Join(env.target, "orphan.txt")); !os.IsNotExist(err) {
		t.Fatalf("orphan.txt still exists in target")
	}
	if _, err := os.Stat(filepath.Join(env.target, "olddir")); !os.IsNotExist(err) {
		t.Fatalf("olddir still exists in target")
	}
	// keep.txt survives.
	if _, err := os.Stat(filepath.Join(env.target, "keep.txt")); err != nil {
		t.Fatalf("keep.txt missing: %v", err)
	}
	// Deleted entries moved to fake trash.
	if len(env.trashed) != 2 {
		t.Fatalf("trashed = %v, want 2 entries", env.trashed)
	}
	if _, err := os.Stat(filepath.Join(env.trash, "orphan.txt")); err != nil {
		t.Fatalf("orphan.txt not in trash: %v", err)
	}
}

func TestExecuteSyncMirrorEmptiedDirRemoved(t *testing.T) {
	// Source no longer has dir/file.txt but target does; dir itself also
	// exists in target only. The whole subtree collapses into one delete.
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "a")
	env.writeTargetFile(t, "a.txt", "a")
	env.writeTargetFile(t, "emptydir/soon.txt", "x")
	targetStat, err := os.Stat(filepath.Join(env.target, "a.txt"))
	if err != nil {
		t.Fatalf("Stat error = %v", err)
	}
	env.setSourceModTime(t, "a.txt", targetStat.ModTime())

	config := baseSyncConfig(env)
	config.Mode = SyncModeMirror
	result, err := (&FileService{}).ExecuteSync(context.Background(), config)
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}
	if result.Deleted != 1 {
		t.Fatalf("Deleted = %d, want 1", result.Deleted)
	}
	if _, err := os.Stat(filepath.Join(env.target, "emptydir")); !os.IsNotExist(err) {
		t.Fatalf("emptydir still exists")
	}
}

func TestExecuteSyncOverwritesChangedFile(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "new content")
	env.writeTargetFile(t, "a.txt", "old content")

	result, err := (&FileService{}).ExecuteSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}
	if result.Copied != 1 {
		t.Fatalf("Copied = %d, want 1", result.Copied)
	}
	data, err := os.ReadFile(filepath.Join(env.target, "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(data) != "new content" {
		t.Fatalf("target content = %q, want %q", data, "new content")
	}
}

func TestExecuteSyncCancellation(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "hello")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := (&FileService{}).ExecuteSync(ctx, baseSyncConfig(env))
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v (cancelled runs return result, not error)", err)
	}
	if result.Status != SyncStatusCancelled {
		t.Fatalf("Status = %q, want cancelled", result.Status)
	}
	if result.Copied != 0 {
		t.Fatalf("Copied = %d, want 0", result.Copied)
	}
	// Rollback removes partially created target content.
	if _, err := os.Stat(env.target); !os.IsNotExist(err) {
		t.Fatalf("target should not remain after cancelled run")
	}
}

func TestExecuteSyncSingleItemErrorContinues(t *testing.T) {
	env := newSyncTestEnv(t)
	env.writeFile(t, "a.txt", "aaa")
	env.writeFile(t, "dir/b.txt", "bbb")

	// Create a directory where a.txt's parent chain expects a file in the
	// target to force a per-item copy error mid-run.
	env.writeTargetFile(t, "a.txt", "old")

	// Make target a.txt read-only directory content conflicts are hard to
	// force portably; instead remove source's dir/b.txt parent mid-run is
	// racy. Simplest portable failure: point one action at a source file
	// deleted between analysis and copy — simulate by pre-deleting nothing
	// and trusting analysis+copy consistency. Instead, force error via
	// unwritable target subdirectory name collision: target "dir" exists
	// as a FILE.
	if err := os.WriteFile(filepath.Join(env.target, "dir"), []byte("blocked"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	result, err := (&FileService{}).ExecuteSync(context.Background(), baseSyncConfig(env))
	if err != nil {
		t.Fatalf("ExecuteSync() error = %v", err)
	}
	if result.Status != SyncStatusDone {
		t.Fatalf("Status = %q, want done (item errors don't cancel)", result.Status)
	}
	if len(result.Errors) == 0 {
		t.Fatalf("Errors empty, want dir/b.txt copy failure recorded")
	}
	// a.txt still copied despite dir/b.txt failing.
	if result.Copied < 1 {
		t.Fatalf("Copied = %d, want >= 1 (other items continue)", result.Copied)
	}
	data, err := os.ReadFile(filepath.Join(env.target, "a.txt"))
	if err != nil || string(data) != "aaa" {
		t.Fatalf("a.txt should be synced, got %q err=%v", data, err)
	}
}

func TestExecuteSyncInvalidConfigReturnsError(t *testing.T) {
	env := newSyncTestEnv(t)
	config := baseSyncConfig(env)
	config.Mode = "bogus"
	if _, err := (&FileService{}).ExecuteSync(context.Background(), config); err == nil {
		t.Fatalf("ExecuteSync() expected error for invalid mode")
	}
}
