package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInitialDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no args",
			args: nil,
			want: "",
		},
		{
			name: "single dir arg",
			args: []string{dir},
			want: dir,
		},
		{
			name: "skips empty/nonexistent, uses first valid dir",
			args: []string{"", "/nonexistent/path", dir, "/another/path"},
			want: dir,
		},
		{
			name: "relative dir resolved to absolute",
			args: []string{"."},
			want: func() string {
				abs, _ := filepath.Abs(".")
				return abs
			}(),
		},
		{
			name: "file path is ignored",
			args: []string{file},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveInitialDir(tc.args)
			if got != tc.want {
				t.Errorf("resolveInitialDir(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}
