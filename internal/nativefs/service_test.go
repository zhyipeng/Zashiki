package nativefs

import (
	"errors"
	"testing"
)

// fakeTransfer 注入测试用的平台实现（验证 service 委托，不依赖真实剪贴板）。
type fakeTransfer struct {
	copiedPaths    []string
	cutPaths       []string
	content        ClipboardContent
	seq            uint64
	copyErr        error
	cutErr         error
	readErr        error
	seqErr         error
	clearErr       error
	clipboardCalls int
	cleared        bool
}

func (f *fakeTransfer) Copy(paths []string) error {
	f.copiedPaths = append([]string(nil), paths...)
	return f.copyErr
}

func (f *fakeTransfer) Cut(paths []string) error {
	f.cutPaths = append([]string(nil), paths...)
	return f.cutErr
}

func (f *fakeTransfer) ClipboardFiles() (ClipboardContent, error) {
	f.clipboardCalls++
	return f.content, f.readErr
}

func (f *fakeTransfer) CurrentSequence() (uint64, error) {
	return f.seq, f.seqErr
}

func (f *fakeTransfer) ClearClipboard() error {
	f.cleared = true
	return f.clearErr
}

func TestServiceCopyDelegates(t *testing.T) {
	fake := &fakeTransfer{}
	svc := &FileTransferService{transfer: fake}
	paths := []string{"/a.txt", "/b.txt"}
	if err := svc.Copy(paths); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if len(fake.copiedPaths) != 2 || fake.copiedPaths[0] != "/a.txt" {
		t.Errorf("unexpected copied paths: %v", fake.copiedPaths)
	}
}

func TestServiceCutDelegates(t *testing.T) {
	fake := &fakeTransfer{}
	svc := &FileTransferService{transfer: fake}
	paths := []string{"/a.txt"}
	if err := svc.Cut(paths); err != nil {
		t.Fatalf("Cut failed: %v", err)
	}
	if len(fake.cutPaths) != 1 || fake.cutPaths[0] != "/a.txt" {
		t.Errorf("unexpected cut paths: %v", fake.cutPaths)
	}
}

func TestServiceClipboardFilesReturnsContent(t *testing.T) {
	fake := &fakeTransfer{content: ClipboardContent{
		Paths: []string{"/a.txt"},
		Op:    ClipboardMove,
		Seq:   42,
	}}
	svc := &FileTransferService{transfer: fake}
	content, err := svc.ClipboardFiles()
	if err != nil {
		t.Fatalf("ClipboardFiles failed: %v", err)
	}
	if len(content.Paths) != 1 || content.Paths[0] != "/a.txt" || content.Op != ClipboardMove || content.Seq != 42 {
		t.Errorf("unexpected content: %+v", content)
	}
}

func TestServiceClipboardSequence(t *testing.T) {
	fake := &fakeTransfer{seq: 99}
	svc := &FileTransferService{transfer: fake}
	seq, err := svc.ClipboardSequence()
	if err != nil {
		t.Fatalf("ClipboardSequence failed: %v", err)
	}
	if seq != 99 {
		t.Errorf("unexpected seq: %d", seq)
	}
}

func TestServiceErrorPropagation(t *testing.T) {
	sentinel := errors.New("clipboard busy")
	fake := &fakeTransfer{copyErr: sentinel}
	svc := &FileTransferService{transfer: fake}
	err := svc.Copy([]string{"/a.txt"})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

func TestNewFileTransferServiceInjectsTransfer(t *testing.T) {
	svc := NewFileTransferService()
	if svc.transfer == nil {
		t.Fatal("NewFileTransferService did not inject transfer")
	}
}

func TestServiceClearClipboard(t *testing.T) {
	fake := &fakeTransfer{}
	svc := &FileTransferService{transfer: fake}
	if err := svc.ClearClipboard(); err != nil {
		t.Fatalf("ClearClipboard failed: %v", err)
	}
	if !fake.cleared {
		t.Error("ClearClipboard did not delegate to transfer")
	}
}
