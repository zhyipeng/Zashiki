package filemanager

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestEditorCommand(t *testing.T) {
	cmd := editorCommand("/tmp/example.txt", " code ")
	if runtime.GOOS == "darwin" {
		if filepath.Base(cmd.Path) != "open" {
			t.Fatalf("expected darwin app command path open, got %q", cmd.Path)
		}
		wantArgs := []string{"open", "-a", "code", "/tmp/example.txt"}
		assertArgs(t, cmd.Args, wantArgs)
		return
	}

	if cmd.Path != "code" {
		t.Fatalf("expected command path code, got %q", cmd.Path)
	}
	assertArgs(t, cmd.Args, []string{"code", "/tmp/example.txt"})
}

func TestIsMacApplication(t *testing.T) {
	tests := []struct {
		name   string
		editor string
		want   bool
	}{
		{name: "app bundle", editor: "/Applications/Visual Studio Code.app", want: true},
		{name: "app name", editor: "Visual Studio Code", want: true},
		{name: "executable path", editor: "/usr/local/bin/code", want: false},
		{name: "empty", editor: " ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMacApplication(tt.editor); got != tt.want {
				t.Fatalf("isMacApplication(%q) = %v, want %v", tt.editor, got, tt.want)
			}
		})
	}
}

func assertArgs(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q; full args: %v", i, got[i], want[i], got)
		}
	}
}
