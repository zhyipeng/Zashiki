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

func TestContextMenuRegistrations(t *testing.T) {
	registrations := contextMenuRegistrations()
	if len(registrations) != 3 {
		t.Fatalf("expected three context menu registrations, got %d", len(registrations))
	}
	if registrations[0].keyPath != `Software\Classes\Directory\shell\Zashiki` || registrations[0].placeholder != "%1" {
		t.Fatalf("unexpected directory registration: %#v", registrations[0])
	}
	if registrations[1].keyPath != `Software\Classes\Directory\Background\shell\Zashiki` || registrations[1].placeholder != "%V" {
		t.Fatalf("unexpected background registration: %#v", registrations[1])
	}
	if registrations[2].keyPath != `Software\Classes\Drive\shell\Zashiki` || registrations[2].placeholder != "%1" {
		t.Fatalf("unexpected drive registration: %#v", registrations[2])
	}
}

func TestContextMenuCommandQuotesExecutableAndArgument(t *testing.T) {
	got := contextMenuCommand(`C:\Program Files\Zashiki\Zashiki.exe`, "%1")
	want := `"C:\Program Files\Zashiki\Zashiki.exe" "%1"`
	if got != want {
		t.Fatalf("contextMenuCommand() = %q, want %q", got, want)
	}
}
