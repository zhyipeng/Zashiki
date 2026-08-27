package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

// AddToPath adds the directory containing the running application to the
// current user's PATH and updates the process environment immediately.
func (s *SettingsService) AddToPath() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	executableDir, err := currentExecutableDir()
	if err != nil {
		return err
	}

	return addPathToUserEnvironment(executableDir)
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

func currentExecutableDir() (string, error) {
	executable, err := currentExecutablePath()
	if err != nil {
		return "", err
	}

	executableDir := filepath.Dir(executable)
	if executableDir == "." || executableDir == "" {
		return "", fmt.Errorf("resolve current executable directory")
	}
	return executableDir, nil
}
