package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newTestService(t *testing.T) *SettingsService {
	t.Helper()
	return &SettingsService{cfgDir: t.TempDir()}
}

func TestSettingsService_GetSettings_Default(t *testing.T) {
	svc := newTestService(t)
	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	if settings.ShowHiddenFiles {
		t.Error("expected ShowHiddenFiles to default to false")
	}
	if settings.ThemeMode != "system" {
		t.Errorf("expected ThemeMode=system by default, got %q", settings.ThemeMode)
	}
	if settings.DefaultEditor != "" {
		t.Errorf("expected DefaultEditor to default to empty, got %q", settings.DefaultEditor)
	}
	if len(settings.PinnedQuickAccessPaths) != 0 {
		t.Errorf("expected PinnedQuickAccessPaths to default to empty, got %v", settings.PinnedQuickAccessPaths)
	}
	if settings.SyncTool.Mode != "incremental" {
		t.Errorf("expected SyncTool.Mode=incremental by default, got %q", settings.SyncTool.Mode)
	}
	if !settings.SyncTool.CompareSize || !settings.SyncTool.CompareModTime {
		t.Errorf("expected SyncTool size+modTime comparison by default, got %+v", settings.SyncTool)
	}
	if !settings.SyncTool.IgnoreHidden {
		t.Error("expected SyncTool.IgnoreHidden=true by default")
	}
	if settings.SyncTool.CompareHash {
		t.Error("expected SyncTool.CompareHash=false by default")
	}
}

func TestSettingsService_SaveAndGet(t *testing.T) {
	svc := newTestService(t)
	original := Settings{
		ShowHiddenFiles:        true,
		ThemeMode:              "dark",
		TerminalProgram:        "Terminal",
		DefaultEditor:          "code",
		PinnedQuickAccessPaths: []string{"/tmp/project", "/tmp/archive"},
	}
	if err := svc.SaveSettings(original); err != nil {
		t.Fatalf("SaveSettings() failed: %v", err)
	}

	loaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	if loaded.ShowHiddenFiles != true {
		t.Errorf("expected ShowHiddenFiles=true, got %v", loaded.ShowHiddenFiles)
	}
	if loaded.ThemeMode != "dark" {
		t.Errorf("expected ThemeMode=dark, got %q", loaded.ThemeMode)
	}
	if loaded.TerminalProgram != "Terminal" {
		t.Errorf("expected TerminalProgram=Terminal, got %q", loaded.TerminalProgram)
	}
	if loaded.DefaultEditor != "code" {
		t.Errorf("expected DefaultEditor=code, got %q", loaded.DefaultEditor)
	}
	if len(loaded.PinnedQuickAccessPaths) != 2 || loaded.PinnedQuickAccessPaths[0] != "/tmp/project" || loaded.PinnedQuickAccessPaths[1] != "/tmp/archive" {
		t.Errorf("expected pinned quick access paths to round trip, got %v", loaded.PinnedQuickAccessPaths)
	}
}

func TestSettingsService_SaveAndGetSyncTool(t *testing.T) {
	svc := newTestService(t)
	original := Settings{
		SyncTool: SyncToolSettings{
			SourceDir:      "/tmp/source",
			TargetDir:      "/tmp/target",
			Mode:           "mirror",
			CompareHash:    true,
			IgnoreHidden:   false,
			IgnorePatterns: []string{"node_modules", "*.tmp"},
		},
	}
	if err := svc.SaveSettings(original); err != nil {
		t.Fatalf("SaveSettings() failed: %v", err)
	}

	loaded, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	syncTool := loaded.SyncTool
	if syncTool.SourceDir != "/tmp/source" || syncTool.TargetDir != "/tmp/target" {
		t.Errorf("expected sync dirs to round trip, got %+v", syncTool)
	}
	if syncTool.Mode != "mirror" {
		t.Errorf("expected Mode=mirror, got %q", syncTool.Mode)
	}
	if syncTool.CompareSize || syncTool.CompareModTime || !syncTool.CompareHash {
		t.Errorf("expected only hash comparison, got %+v", syncTool)
	}
	if syncTool.IgnoreHidden {
		t.Error("expected IgnoreHidden=false to round trip")
	}
	if len(syncTool.IgnorePatterns) != 2 || syncTool.IgnorePatterns[0] != "node_modules" || syncTool.IgnorePatterns[1] != "*.tmp" {
		t.Errorf("expected ignore patterns to round trip, got %v", syncTool.IgnorePatterns)
	}
}

func TestSettingsService_LegacyConfigWithoutSyncTool(t *testing.T) {
	svc := newTestService(t)
	path, err := svc.configPath()
	if err != nil {
		t.Fatalf("configPath() failed: %v", err)
	}
	// A config written before the sync tool existed.
	if err := os.WriteFile(path, []byte(`{"showHiddenFiles":true,"themeMode":"dark"}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	if !settings.ShowHiddenFiles || settings.ThemeMode != "dark" {
		t.Errorf("expected legacy fields preserved, got %+v", settings)
	}
	if settings.SyncTool.Mode != "incremental" || !settings.SyncTool.CompareSize {
		t.Errorf("expected sync defaults for legacy config, got %+v", settings.SyncTool)
	}
}

func TestSettingsService_InvalidSyncToolNormalized(t *testing.T) {
	svc := newTestService(t)
	path, err := svc.configPath()
	if err != nil {
		t.Fatalf("configPath() failed: %v", err)
	}
	config := `{"syncTool":{"mode":"bogus","compareSize":false,"compareModTime":false,"compareHash":false}}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	syncTool := settings.SyncTool
	if syncTool.Mode != "incremental" {
		t.Errorf("expected invalid mode normalized to incremental, got %q", syncTool.Mode)
	}
	if !syncTool.CompareSize {
		t.Error("expected CompareSize re-enabled when no dimension is configured")
	}
}

func TestSettingsService_ConfigFileCreated(t *testing.T) {
	svc := newTestService(t)
	if err := svc.SaveSettings(Settings{ShowHiddenFiles: false}); err != nil {
		t.Fatalf("SaveSettings() failed: %v", err)
	}

	path, err := svc.configPath()
	if err != nil {
		t.Fatalf("configPath() failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile config failed: %v", err)
	}

	var parsed Settings
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}
}

func TestSettingsService_ConfigDirCreated(t *testing.T) {
	svc := newTestService(t)
	dir, err := svc.configDir()
	if err != nil {
		t.Fatalf("configDir() failed: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat config dir failed: %v", err)
	}
	if !info.IsDir() {
		t.Error("config dir is not a directory")
	}
}

func TestSettingsService_CorruptedFile(t *testing.T) {
	svc := newTestService(t)
	path, err := svc.configPath()
	if err != nil {
		t.Fatalf("configPath() failed: %v", err)
	}

	if err := os.WriteFile(path, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() should return defaults on corrupted file, got error: %v", err)
	}
	if settings.ShowHiddenFiles {
		t.Error("expected default ShowHiddenFiles=false for corrupted config file")
	}
	if settings.ThemeMode != "system" {
		t.Errorf("expected default ThemeMode=system for corrupted config file, got %q", settings.ThemeMode)
	}
}

func TestSettingsService_InvalidThemeMode(t *testing.T) {
	svc := newTestService(t)
	path, err := svc.configPath()
	if err != nil {
		t.Fatalf("configPath() failed: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"showHiddenFiles":true,"themeMode":"unknown"}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	if settings.ThemeMode != "system" {
		t.Errorf("expected invalid ThemeMode to fallback to system, got %q", settings.ThemeMode)
	}
	if !settings.ShowHiddenFiles {
		t.Error("expected ShowHiddenFiles to be preserved")
	}
}

func TestConfigDirDefaultPath(t *testing.T) {
	svc := newTestService(t)
	dir, err := svc.configDir()
	if err != nil {
		t.Fatalf("configDir() failed: %v", err)
	}

	base := filepath.Base(dir)
	if base != "zashiki" {
		t.Errorf("expected config dir to end with 'zashiki', got %s", base)
	}
}
