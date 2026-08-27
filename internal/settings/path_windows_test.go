//go:build windows

package settings

import (
	"reflect"
	"testing"
)

func TestSplitWindowsPath(t *testing.T) {
	got := splitWindowsPath(`C:\Windows;; C:\Tools;`)
	want := []string{`C:\Windows`, ` C:\Tools`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitWindowsPath() = %#v, want %#v", got, want)
	}
}

func TestEqualWindowsPathEntry(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		want        bool
	}{
		{name: "case insensitive", left: `C:\Tools`, right: `c:\tools`, want: true},
		{name: "slash normalization", left: `C:/Tools/`, right: `c:\tools`, want: true},
		{name: "different directory", left: `C:\Tools`, right: `C:\Other`, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := equalWindowsPathEntry(tc.left, tc.right); got != tc.want {
				t.Fatalf("equalWindowsPathEntry(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
		})
	}
}
