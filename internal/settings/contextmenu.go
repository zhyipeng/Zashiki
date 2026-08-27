package settings

import "fmt"

// AddToContextMenu adds a Windows Explorer entry that opens selected folders
// with the running application.
func (s *SettingsService) AddToContextMenu() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	executable, err := currentExecutablePath()
	if err != nil {
		return err
	}

	return addContextMenuForExecutable(executable)
}

func unsupportedContextMenuError() error {
	return fmt.Errorf("adding the application to the Windows context menu is not supported on this platform")
}
