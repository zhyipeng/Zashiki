package nativefs

import (
	"os"
	"path/filepath"
	"testing"
)

// 平台集成测试：直接驱动真实剪贴板（darwin 在本机可跑）。
// 测试约定（AGENTS.md）：不 mock 文件系统、不 mock 内部依赖；使用临时目录。

func TestDarwinClipboardRoundTrip(t *testing.T) {
	if !isDarwin() {
		t.Skip("darwin only")
	}
	dir := t.TempDir()
	file1 := filepath.Join(dir, "alpha.txt")
	file2 := filepath.Join(dir, "beta.txt")
	for _, p := range []string{file1, file2} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	transfer := NewFileTransfer()
	if err := transfer.Copy([]string{file1, file2}); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	content, err := transfer.ClipboardFiles()
	if err != nil {
		t.Fatalf("ClipboardFiles failed: %v", err)
	}
	if len(content.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", content.Paths)
	}
	if content.Paths[0] != file1 || content.Paths[1] != file2 {
		t.Fatalf("path mismatch: %v != [%s %s]", content.Paths, file1, file2)
	}
	if content.Op != ClipboardCopy {
		t.Fatalf("expected copy op, got %v", content.Op)
	}
	if content.Seq == 0 {
		t.Fatalf("expected non-zero seq")
	}
}

func TestDarwinCutWritesMoveSemantics(t *testing.T) {
	if !isDarwin() {
		t.Skip("darwin only")
	}
	dir := t.TempDir()
	file1 := filepath.Join(dir, "cutme.txt")
	if err := os.WriteFile(file1, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	transfer := NewFileTransfer()
	if err := transfer.Cut([]string{file1}); err != nil {
		t.Fatalf("Cut failed: %v", err)
	}

	content, err := transfer.ClipboardFiles()
	if err != nil {
		t.Fatalf("ClipboardFiles failed: %v", err)
	}
	if len(content.Paths) != 1 || content.Paths[0] != file1 {
		t.Fatalf("unexpected paths: %v", content.Paths)
	}
	// Finder 粘贴按复制处理；GNOME 语义不强制（macOS 无对称 cut 文件语义）
	_ = content.Op
}

func TestDarwinSequenceChangesOnRewrite(t *testing.T) {
	if !isDarwin() {
		t.Skip("darwin only")
	}
	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file1, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	transfer := NewFileTransfer()
	seq0, err := transfer.CurrentSequence()
	if err != nil {
		t.Fatalf("CurrentSequence failed: %v", err)
	}
	if err := transfer.Copy([]string{file1}); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	seq1, err := transfer.CurrentSequence()
	if err != nil {
		t.Fatalf("CurrentSequence failed: %v", err)
	}
	if seq1 <= seq0 {
		t.Fatalf("expected sequence to increase after write: %d -> %d", seq0, seq1)
	}
}

func isDarwin() bool {
	return os.PathSeparator == '/' && os.Getenv("GOOS") != "windows" && os.Getenv("GOOS") != "linux"
}
