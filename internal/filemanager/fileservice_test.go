package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileService_ListDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644)

	s := &FileService{}
	entries, err := s.ListDir(dir)
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// 目录在前
	if !entries[0].IsDir || entries[0].Name != "subdir" {
		t.Errorf("first entry should be subdir (dir), got %v", entries[0])
	}
	// 文件按名称排序在后
	if entries[1].Name != "a.txt" || entries[2].Name != "b.txt" {
		t.Errorf("files not sorted by name, got %v and %v", entries[1].Name, entries[2].Name)
	}
}

func TestFileService_ListDir_NotFound(t *testing.T) {
	s := &FileService{}
	_, err := s.ListDir("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestFileService_GetFileInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("content"), 0o644)

	s := &FileService{}
	info, err := s.GetFileInfo(path)
	if err != nil {
		t.Fatalf("GetFileInfo() error = %v", err)
	}
	if info.Name != "test.txt" {
		t.Errorf("Name = %q, want %q", info.Name, "test.txt")
	}
	if info.Size != 7 {
		t.Errorf("Size = %d, want 7", info.Size)
	}
	if info.IsDir {
		t.Error("IsDir should be false")
	}
}

func TestFileService_CreateFolder(t *testing.T) {
	dir := t.TempDir()
	s := &FileService{}

	first, err := s.CreateFolder(dir, "New Folder")
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("created folder should exist: %v", err)
	}

	second, err := s.CreateFolder(dir, "New Folder")
	if err != nil {
		t.Fatalf("CreateFolder() duplicate error = %v", err)
	}
	if second == first {
		t.Fatal("CreateFolder() duplicate should use a unique path")
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("second folder should exist: %v", err)
	}
}

func TestFileService_CreateFolderAt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "exact")
	s := &FileService{}

	created, err := s.CreateFolderAt(target)
	if err != nil {
		t.Fatalf("CreateFolderAt() error = %v", err)
	}
	if created != target {
		t.Fatalf("CreateFolderAt() = %q, want %q", created, target)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("created folder stat = %v, err = %v", info, err)
	}
	if _, err := s.CreateFolderAt(target); err == nil {
		t.Fatal("CreateFolderAt() expected error for existing target")
	}
}

func TestFileService_RenameEntry(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	entry, err := s.RenameEntry(oldPath, "new.txt")
	if err != nil {
		t.Fatalf("RenameEntry() error = %v", err)
	}
	if entry.Name != "new.txt" {
		t.Fatalf("Name = %q, want new.txt", entry.Name)
	}
	if entry.Path != filepath.Join(dir, "new.txt") {
		t.Fatalf("Path = %q, want renamed path", entry.Path)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old path should not exist, stat error = %v", err)
	}
}

func TestFileService_RenameEntryRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	if _, err := s.RenameEntry(path, filepath.Join("nested", "file.txt")); err == nil {
		t.Fatal("RenameEntry() expected error for path-like name")
	}
}

func TestFileService_RenameEntryRejectsExistingDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	if _, err := s.RenameEntry(path, "existing.txt"); err == nil {
		t.Fatal("RenameEntry() expected error for existing destination")
	}
}

func TestFileService_DeleteEntries(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	subdir := filepath.Join(dir, "subdir")
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	deleted, err := s.DeleteEntries(context.Background(), []string{file, subdir})
	if err != nil {
		t.Fatalf("DeleteEntries() error = %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("DeleteEntries() deleted %d paths, want 2", len(deleted))
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("deleted file should not exist, stat error = %v", err)
	}
	if _, err := os.Stat(subdir); !os.IsNotExist(err) {
		t.Fatalf("deleted directory should not exist, stat error = %v", err)
	}
}

func TestFileService_DeleteEmptyFolderRejectsNonEmpty(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(child, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	if _, err := s.DeleteEmptyFolder(dir); err == nil {
		t.Fatal("DeleteEmptyFolder() expected error for non-empty directory")
	}
	if _, err := os.Stat(child); err != nil {
		t.Fatalf("child should remain after rejected delete, stat error = %v", err)
	}
}

func TestFileService_DeleteEntriesRejectsRoot(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	s := &FileService{}
	if _, err := s.DeleteEntries(context.Background(), []string{root}); err == nil {
		t.Fatal("DeleteEntries() expected error for filesystem root")
	}
}

func TestFileService_CopyEntriesRejectsDirectoryToItself(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	_, err := s.CopyEntries(context.Background(), []string{src}, dir, "overwrite")
	if err == nil {
		t.Fatal("CopyEntries() expected error when copying directory to itself")
	}
}

func TestFileService_CopyEntriesAllowsRenameConflictInSameParent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	results, err := s.CopyEntries(context.Background(), []string{src}, dir, "rename")
	if err != nil {
		t.Fatalf("CopyEntries() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("CopyEntries() returned %d results, want 1", len(results))
	}
	if results[0].SourcePath != src || results[0].TargetPath != filepath.Join(dir, "src (1)") || results[0].Skipped || results[0].Overwritten {
		t.Fatalf("CopyEntries() result = %+v, want renamed copy result", results[0])
	}
	if _, statErr := os.Stat(filepath.Join(dir, "src (1)", "file.txt")); statErr != nil {
		t.Fatalf("renamed copy should exist, stat error = %v", statErr)
	}
}

func TestFileService_CopyEntriesToTargets(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	target := filepath.Join(dir, "nested", "target.txt")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	results, err := s.CopyEntriesToTargets(context.Background(), []EntryPathPair{{SourcePath: src, TargetPath: target}})
	if err != nil {
		t.Fatalf("CopyEntriesToTargets() error = %v", err)
	}
	if len(results) != 1 || results[0].SourcePath != src || results[0].TargetPath != target {
		t.Fatalf("CopyEntriesToTargets() result = %+v, want exact target result", results)
	}
	if content, err := os.ReadFile(target); err != nil || string(content) != "content" {
		t.Fatalf("target content = %q, err = %v", string(content), err)
	}
}

func TestFileService_CopyEntriesToTargetsRejectsExistingTarget(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	if _, err := s.CopyEntriesToTargets(context.Background(), []EntryPathPair{{SourcePath: src, TargetPath: target}}); err == nil {
		t.Fatal("CopyEntriesToTargets() expected error for existing target")
	}
}

func TestFileService_MoveEntriesToTargets(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	target := filepath.Join(dir, "nested", "target.txt")
	if err := os.WriteFile(src, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	results, err := s.MoveEntriesToTargets(context.Background(), []EntryPathPair{{SourcePath: src, TargetPath: target}})
	if err != nil {
		t.Fatalf("MoveEntriesToTargets() error = %v", err)
	}
	if len(results) != 1 || results[0].SourcePath != src || results[0].TargetPath != target {
		t.Fatalf("MoveEntriesToTargets() result = %+v, want exact target result", results)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should not exist after move, stat error = %v", err)
	}
	if content, err := os.ReadFile(target); err != nil || string(content) != "content" {
		t.Fatalf("target content = %q, err = %v", string(content), err)
	}
}

func TestFileService_CheckConflictsRejectsDirectoryAsDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	_, err := s.CheckConflicts([]string{src}, src)
	if err == nil {
		t.Fatal("CheckConflicts() expected error when destination is source")
	}
}

func TestFileService_CopyEntriesRejectsDirectoryToChild(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	destDir := filepath.Join(src, "child")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	_, err := s.CopyEntries(context.Background(), []string{src}, destDir, "overwrite")
	if err == nil {
		t.Fatal("CopyEntries() expected error when copying directory to child")
	}
	if _, statErr := os.Stat(destDir); !os.IsNotExist(statErr) {
		t.Fatalf("destination child should not be created, stat error = %v", statErr)
	}
}

func TestFileService_MoveEntriesRejectsDirectoryToChildWithoutDeletingSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	destDir := filepath.Join(src, "child")
	file := filepath.Join(src, "file.txt")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	_, err := s.MoveEntries(context.Background(), []string{src}, destDir, "overwrite")
	if err == nil {
		t.Fatal("MoveEntries() expected error when moving directory to child")
	}
	if _, statErr := os.Stat(file); statErr != nil {
		t.Fatalf("source file should remain after rejected move, stat error = %v", statErr)
	}
	if _, statErr := os.Stat(destDir); !os.IsNotExist(statErr) {
		t.Fatalf("destination child should not be created, stat error = %v", statErr)
	}
}
