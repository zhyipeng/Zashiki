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

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}

	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	executableDir := filepath.Dir(executable)
	if executableDir == "." || executableDir == "" {
		return fmt.Errorf("resolve current executable directory")
	}

	return addPathToUserEnvironment(executableDir)
}
