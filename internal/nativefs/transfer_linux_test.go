//go:build linux || freebsd || openbsd || netbsd

package nativefs

import (
	"strings"
	"testing"
)

// 纯逻辑测试：text/uri-list 与 GNOME 移动语义编解码（不依赖真实 X 剪贴板）。

func TestParseURIText(t *testing.T) {
	text := "file:///home/user/a.txt\r\nfile:///home/user/%E4%B8%AD%E6%96%87%20%E7%9B%AE%E5%BD%95/b.txt\r\n# comment\r\n\r\n"
	paths := parseURIText(text)
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
	if paths[0] != "/home/user/a.txt" {
		t.Errorf("path[0] = %q", paths[0])
	}
	if paths[1] != "/home/user/中文 目录/b.txt" {
		t.Errorf("path[1] = %q", paths[1])
	}
}

func TestBuildAndParseURITextRoundTrip(t *testing.T) {
	paths := []string{"/tmp/a.txt", "/tmp/带空格 文件名.txt", "/tmp/中文.txt"}
	text := buildURIText(paths)
	parsed := parseURIText(text)
	if len(parsed) != len(paths) {
		t.Fatalf("round trip length mismatch: got %v want %v", parsed, paths)
	}
	for i := range paths {
		if parsed[i] != paths[i] {
			t.Errorf("round trip[%d] = %q want %q", i, parsed[i], paths[i])
		}
	}
}

func TestParseGNOMETextCut(t *testing.T) {
	text := "cut\nfile:///tmp/a.txt\nfile:///tmp/b.txt\n"
	move, paths := parseGNOMEText(text)
	if !move {
		t.Error("expected move=true for cut header")
	}
	if len(paths) != 2 || paths[0] != "/tmp/a.txt" {
		t.Errorf("unexpected paths: %v", paths)
	}
}

func TestParseGNOMETextCopy(t *testing.T) {
	text := "copy\nfile:///tmp/a.txt\n"
	move, paths := parseGNOMEText(text)
	if move {
		t.Error("expected move=false for copy header")
	}
	if len(paths) != 1 {
		t.Errorf("unexpected paths: %v", paths)
	}
}

func TestFnv64aStable(t *testing.T) {
	a := fnv64a([]byte("hello"))
	b := fnv64a([]byte("hello"))
	c := fnv64a([]byte("hello!"))
	if a != b {
		t.Error("fnv64a not deterministic")
	}
	if a == c {
		t.Error("fnv64a should differ for different inputs")
	}
}

func TestBuildURITextUsesCRLF(t *testing.T) {
	text := buildURIText([]string{"/tmp/a.txt"})
	if !strings.Contains(text, "\r\n") {
		t.Errorf("expected CRLF line endings, got %q", text)
	}
}
