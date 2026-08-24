//go:build darwin

package nativefs

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

// 写入剪贴板：文件引用 + 复制/移动语义
void copyFilesToPasteboard(const char **paths, int count, int move);
// 读取剪贴板文件引用；返回 JSON 数组字符串，调用方 free
char *readPasteboardFiles(void);
// 读取剪贴板移动语义（1 表示 move，0 表示 copy）
int readPasteboardMove(void);
// 读取当前 pasteboard changeCount（即变更序号）
long long pasteboardChangeCount(void);
// 清空剪贴板
void clearPasteboard(void);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// darwinTransfer 基于 NSPasteboard + NSURL（AppKit，见 transfer_darwin.m）。
// 原生拖出由 darwinDrag（drag_darwin.go + drag_darwin.m）实现。
type darwinTransfer struct {
	drag *darwinDrag
}

func newPlatformTransfer() FileTransfer {
	return &darwinTransfer{
		drag: newDarwinDrag(),
	}
}

// Copy 把 paths 作为文件引用以复制语义写入系统剪贴板。
func (t *darwinTransfer) Copy(paths []string) error {
	return writePaths(paths, false)
}

// Cut 把 paths 作为文件引用以移动语义写入系统剪贴板。
// 注意：macOS Finder 没有与 Windows 对称的「剪切文件」语义；此处写入
// 的移动语义仅供支持该约定的程序（如 GNOME 文件管理器）识别，Finder
// 粘贴时按复制处理。
func (t *darwinTransfer) Cut(paths []string) error {
	return writePaths(paths, true)
}

func writePaths(paths []string, move bool) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths to copy")
	}
	cpaths := make([]*C.char, len(paths))
	for i, p := range paths {
		cpaths[i] = C.CString(p)
	}
	defer func() {
		for _, cp := range cpaths {
			C.free(unsafe.Pointer(cp))
		}
	}()

	var moveInt C.int
	if move {
		moveInt = 1
	}
	C.copyFilesToPasteboard(&cpaths[0], C.int(len(paths)), moveInt)
	return nil
}

// ClipboardFiles 从系统剪贴板读取文件引用。
func (t *darwinTransfer) ClipboardFiles() (ClipboardContent, error) {
	content := ClipboardContent{Paths: []string{}, Op: ClipboardCopy}
	cJSON := C.readPasteboardFiles()
	if cJSON == nil {
		return content, nil
	}
	defer C.free(unsafe.Pointer(cJSON))

	jsonStr := C.GoString(cJSON)
	var paths []string
	if err := json.Unmarshal([]byte(jsonStr), &paths); err != nil {
		return content, fmt.Errorf("failed to decode pasteboard files: %w", err)
	}
	if len(paths) == 0 {
		return content, nil
	}
	content.Paths = paths
	if C.readPasteboardMove() != 0 {
		content.Op = ClipboardMove
	}
	seq, err := t.CurrentSequence()
	if err != nil {
		return content, err
	}
	content.Seq = seq
	return content, nil
}

// CurrentSequence 返回 pasteboard changeCount，作为剪贴板变更序号。
func (t *darwinTransfer) CurrentSequence() (uint64, error) {
	seq := C.pasteboardChangeCount()
	if seq < 0 {
		return 0, fmt.Errorf("invalid pasteboard change count")
	}
	return uint64(seq), nil
}

// ClearClipboard 清空系统剪贴板。
func (t *darwinTransfer) ClearClipboard() error {
	C.clearPasteboard()
	return nil
}

// StartDrag 启动原生拖出（Wails → Finder），阻塞直到拖拽结束。
func (t *darwinTransfer) StartDrag(paths []string, x, y int, effects DropEffect) (DropEffect, error) {
	return t.drag.StartDrag(paths, x, y, effects)
}
