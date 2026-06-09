//go:build !windows

package filemanager

import "testing"

func TestIsHiddenEntry(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".hidden", true},
		{".gitignore", true},
		{"normal", false},
		{"normal.txt", false},
		{".", false},
		{"..", false},
		{"", false},
	}
	for _, tc := range tests {
		got := isHiddenEntry(tc.name, "")
		if got != tc.want {
			t.Errorf("isHiddenEntry(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
