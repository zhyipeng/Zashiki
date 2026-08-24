//go:build windows

package nativefs

import (
	"testing"
	"unsafe"
)

// 纯逻辑测试：DROPFILES 内存块构造与路径解析（不打开真实剪贴板，
// 不依赖 OLE 初始化）。

func TestBuildHDROPGlobalAndParse(t *testing.T) {
	paths := []string{
		`C:\foo\a.txt`,
		`C:\foo\images`,
		`C:\foo\中文 目录\b 文件.txt`,
	}
	h, err := buildHDROPGlobal(paths)
	if err != nil {
		t.Fatalf("buildHDROPGlobal: %v", err)
	}
	defer globalFree(h)

	parsed, err := hdropPaths(h)
	if err != nil {
		t.Fatalf("hdropPaths: %v", err)
	}
	if len(parsed) != len(paths) {
		t.Fatalf("path count mismatch: got %d want %d (%v)", len(parsed), len(paths), parsed)
	}
	for i := range paths {
		if parsed[i] != paths[i] {
			t.Errorf("path[%d] mismatch: got %q want %q", i, parsed[i], paths[i])
		}
	}
}

func TestBuildHDROPGlobalHeader(t *testing.T) {
	paths := []string{`C:\a.txt`}
	h, err := buildHDROPGlobal(paths)
	if err != nil {
		t.Fatalf("buildHDROPGlobal: %v", err)
	}
	defer globalFree(h)

	lp, err := globalLock(h)
	if err != nil {
		t.Fatalf("globalLock: %v", err)
	}
	defer globalUnlock(h)

	header := (*dropFiles)(unsafe.Pointer(lp))
	if header.fWide != 1 {
		t.Errorf("expected fWide=1 (UTF-16), got %d", header.fWide)
	}
	if header.pFiles != uint32(unsafe.Sizeof(dropFiles{})) {
		t.Errorf("expected pFiles=%d, got %d", unsafe.Sizeof(dropFiles{}), header.pFiles)
	}
}

func TestEffectGlobal(t *testing.T) {
	h, err := buildEffectGlobal(dropEffectMove)
	if err != nil {
		t.Fatalf("buildEffectGlobal: %v", err)
	}
	defer globalFree(h)

	lp, err := globalLock(h)
	if err != nil {
		t.Fatalf("globalLock: %v", err)
	}
	defer globalUnlock(h)
	effect := *(*uint32)(unsafe.Pointer(lp))
	if effect != dropEffectMove {
		t.Errorf("expected effect=%d, got %d", dropEffectMove, effect)
	}
}
