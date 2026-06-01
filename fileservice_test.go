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
