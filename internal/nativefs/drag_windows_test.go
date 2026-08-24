//go:build windows

package nativefs

import "testing"

func TestStartDragRequiresWindowHandle(t *testing.T) {
	// 未注入窗口句柄时，StartDrag 应返回明确错误而不是 panic
	windowProvider = nil
	_, err := startWindowsDrag([]string{`C:\a.txt`}, DropEffectCopy|DropEffectMove)
	if err == nil {
		t.Fatal("expected error when no window handle is registered")
	}
}

func TestStartDragEmptyPaths(t *testing.T) {
	if _, err := startWindowsDrag(nil, DropEffectCopy); err == nil {
		t.Fatal("expected error for empty paths")
	}
	if _, err := startWindowsDrag([]string{}, DropEffectCopy); err == nil {
		t.Fatal("expected error for empty paths")
	}
}
