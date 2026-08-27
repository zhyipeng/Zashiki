package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

// AddToPath adds the running application to the current user's PATH and
// updates the process environment immediately.
func (s *SettingsService) AddToPath() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	executable, err := currentExecutablePath()
	if err != nil {
		return err
	}

	return addPathToUserEnvironment(executable)
}

func currentExecutablePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	return executable, nil
}
