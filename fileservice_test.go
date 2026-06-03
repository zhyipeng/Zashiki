package main

import (
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

func TestFileService_CopyEntriesRejectsDirectoryToItself(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	err := s.CopyEntries([]string{src}, dir, "overwrite")
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
	err := s.CopyEntries([]string{src}, dir, "rename")
	if err != nil {
		t.Fatalf("CopyEntries() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "src (1)", "file.txt")); statErr != nil {
		t.Fatalf("renamed copy should exist, stat error = %v", statErr)
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
	err := s.CopyEntries([]string{src}, destDir, "overwrite")
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
	err := s.MoveEntries([]string{src}, destDir, "overwrite")
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
