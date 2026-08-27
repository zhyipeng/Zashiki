//go:build darwin || linux

package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddPathEntryToShellConfigIsIdempotent(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".zprofile")
	initial := "export PATH='/usr/local/bin':$PATH\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	if err := addPathEntryToShellConfig(configPath, "/Applications/Zashiki.app/Contents/MacOS", "/bin/zsh"); err != nil {
		t.Fatalf("first addPathEntryToShellConfig() failed: %v", err)
	}
	first, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after first add: %v", err)
	}

	if err := addPathEntryToShellConfig(configPath, "/Applications/Zashiki.app/Contents/MacOS", "/bin/zsh"); err != nil {
		t.Fatalf("second addPathEntryToShellConfig() failed: %v", err)
	}
	second, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after second add: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("expected repeated additions to be unchanged, first=%q second=%q", first, second)
	}
	if count := strings.Count(string(second), zashikiPathMarker); count != 1 {
		t.Fatalf("expected one PATH marker, got %d in %q", count, second)
	}
}

func TestAddPathEntryToShellConfigUpdatesManagedEntry(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".zprofile")
	initial := zashikiPathMarker + "\nexport PATH='/old/location':$PATH\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	if err := addPathEntryToShellConfig(configPath, "/new/location", "/bin/zsh"); err != nil {
		t.Fatalf("addPathEntryToShellConfig() failed: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "export PATH='/new/location':$PATH") {
		t.Fatalf("expected managed entry to be updated, got %q", content)
	}
	if strings.Contains(content, "/old/location") {
		t.Fatalf("expected old managed entry to be removed, got %q", content)
	}
}

func TestShellConfigPathUsesExistingShellConfiguration(t *testing.T) {
	home := t.TempDir()
	bashRC := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashRC, []byte("# shell\n"), 0o644); err != nil {
		t.Fatalf("write bash config: %v", err)
	}

	if got := shellConfigPath(home, "/bin/bash"); got != bashRC {
		t.Fatalf("shellConfigPath() = %q, want %q", got, bashRC)
	}
	if got := shellConfigPath(home, "/bin/fish"); got != filepath.Join(home, ".config", "fish", "config.fish") {
		t.Fatalf("shellConfigPath() for fish = %q", got)
	}
}

func TestAddPathToProcessEnvironmentIsIdempotent(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")

	if err := addPathToProcessEnvironment("/opt/zashiki/bin"); err != nil {
		t.Fatalf("first addPathToProcessEnvironment() failed: %v", err)
	}
	first := os.Getenv("PATH")
	if err := addPathToProcessEnvironment("/opt/zashiki/bin"); err != nil {
		t.Fatalf("second addPathToProcessEnvironment() failed: %v", err)
	}

	if got := os.Getenv("PATH"); got != first {
		t.Fatalf("expected repeated additions to be unchanged, first=%q got=%q", first, got)
	}
	if got := strings.Count(first, "/opt/zashiki/bin"); got != 1 {
		t.Fatalf("expected one process PATH entry, got %d in %q", got, first)
	}
}

func TestUnixPathLauncherContent(t *testing.T) {
	got := unixPathLauncherContent("/Applications/Zashiki.app/Contents/MacOS/Zashiki")
	want := "#!/bin/sh\nnohup '/Applications/Zashiki.app/Contents/MacOS/Zashiki' \"$@\" </dev/null >/dev/null 2>&1 &\n"
	if got != want {
		t.Fatalf("unixPathLauncherContent() = %q, want %q", got, want)
	}
}

func TestUnixPathLauncherContentQuotesSingleQuotes(t *testing.T) {
	got := unixPathLauncherContent("/Applications/O'Reilly/Zashiki")
	if !strings.Contains(got, `'/Applications/O'"'"'Reilly/Zashiki'`) {
		t.Fatalf("expected single quotes to be shell-escaped: %q", got)
	}
}
