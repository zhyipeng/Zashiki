package main

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
}

func TestSettingsService_SaveAndGet(t *testing.T) {
	svc := newTestService(t)
	original := Settings{ShowHiddenFiles: true}
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
