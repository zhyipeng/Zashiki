package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/adrg/xdg"
)

type SyncToolSettings struct {
	SourceDir      string   `json:"sourceDir"`
	TargetDir      string   `json:"targetDir"`
	Mode           string   `json:"mode"`
	CompareSize    bool     `json:"compareSize"`
	CompareModTime bool     `json:"compareModTime"`
	CompareHash    bool     `json:"compareHash"`
	IgnoreHidden   bool     `json:"ignoreHidden"`
	IgnorePatterns []string `json:"ignorePatterns"`
}

type Settings struct {
	ShowHiddenFiles        bool             `json:"showHiddenFiles"`
	ThemeMode              string           `json:"themeMode"`
	TerminalProgram        string           `json:"terminalProgram"`
	DefaultEditor          string           `json:"defaultEditor"`
	PinnedQuickAccessPaths []string         `json:"pinnedQuickAccessPaths"`
	SyncTool               SyncToolSettings `json:"syncTool"`
}

type SettingsService struct {
	mu     sync.Mutex
	cfgDir string // overrides xdg.ConfigHome in tests
}

func (s *SettingsService) baseDir() string {
	if s.cfgDir != "" {
		return s.cfgDir
	}
	return xdg.ConfigHome
}

func (s *SettingsService) configDir() (string, error) {
	dir := filepath.Join(s.baseDir(), "zashiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (s *SettingsService) configPath() (string, error) {
	dir, err := s.configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func (s *SettingsService) GetSettings() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.configPath()
	if err != nil {
		return Settings{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return Settings{}, err
	}

	settings := defaultSettings()
	if err := json.Unmarshal(data, &settings); err != nil {
		return defaultSettings(), nil
	}
	if !isValidThemeMode(settings.ThemeMode) {
		settings.ThemeMode = defaultSettings().ThemeMode
	}
	settings.SyncTool = normalizeSyncToolSettings(settings.SyncTool)
	return settings, nil
}

func (s *SettingsService) SaveSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func defaultSettings() Settings {
	return Settings{
		ThemeMode: "system",
		SyncTool: SyncToolSettings{
			Mode:           "incremental",
			CompareSize:    true,
			CompareModTime: true,
			IgnoreHidden:   true,
		},
	}
}

func isValidThemeMode(mode string) bool {
	return mode == "light" || mode == "dark" || mode == "system"
}

// normalizeSyncToolSettings repairs stored sync tool values so the frontend
// always receives a usable configuration: mode falls back to incremental and
// a config without any comparison dimension re-enables the size default.
func normalizeSyncToolSettings(syncTool SyncToolSettings) SyncToolSettings {
	if syncTool.Mode != "mirror" && syncTool.Mode != "incremental" {
		syncTool.Mode = "incremental"
	}
	if !syncTool.CompareSize && !syncTool.CompareModTime && !syncTool.CompareHash {
		syncTool.CompareSize = true
	}
	if syncTool.IgnorePatterns == nil {
		syncTool.IgnorePatterns = []string{}
	}
	return syncTool
}
