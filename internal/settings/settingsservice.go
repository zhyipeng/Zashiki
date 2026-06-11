package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/adrg/xdg"
)

type Settings struct {
	ShowHiddenFiles bool   `json:"showHiddenFiles"`
	ThemeMode       string `json:"themeMode"`
	TerminalProgram string `json:"terminalProgram"`
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
	return Settings{ThemeMode: "system"}
}

func isValidThemeMode(mode string) bool {
	return mode == "light" || mode == "dark" || mode == "system"
}
