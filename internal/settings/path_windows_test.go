//go:build windows

package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestPrependWindowsPathEntry(t *testing.T) {
	got := prependWindowsPathEntry(`C:\Windows;C:\Tools;C:\Windows\`, `c:\windows`)
	want := `c:\windows;C:\Tools`
	if got != want {
		t.Fatalf("prependWindowsPathEntry() = %q, want %q", got, want)
	}
}

func TestWindowsPathLauncherContent(t *testing.T) {
	got := windowsPathLauncherContent(`C:\Program Files\Zashiki\Zashiki.exe`)
	want := "@echo off\r\nstart \"\" /B \"C:\\Program Files\\Zashiki\\Zashiki.exe\" %*\r\n"
	if got != want {
		t.Fatalf("windowsPathLauncherContent() = %q, want %q", got, want)
	}
}

func TestWindowsPathLauncherContentEscapesPercentSigns(t *testing.T) {
	got := windowsPathLauncherContent(`C:\Users\100%\Zashiki.exe`)
	if !strings.Contains(got, `C:\Users\100%%\Zashiki.exe`) {
		t.Fatalf("expected percent signs to be escaped in launcher content: %q", got)
	}
}

func TestInstalledWindowsPathLauncherDir(t *testing.T) {
	root := t.TempDir()
	launcherDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(launcherDir, 0o755); err != nil {
		t.Fatalf("create launcher directory: %v", err)
	}
	launcherPath := filepath.Join(launcherDir, windowsPathLauncherName)
	if err := os.WriteFile(launcherPath, []byte("launcher"), 0o644); err != nil {
		t.Fatalf("write launcher: %v", err)
	}

	got, ok := installedWindowsPathLauncherDir(filepath.Join(root, "Zashiki.exe"))
	if !ok {
		t.Fatal("expected installed launcher to be detected")
	}
	if got != launcherDir {
		t.Fatalf("installedWindowsPathLauncherDir() = %q, want %q", got, launcherDir)
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
