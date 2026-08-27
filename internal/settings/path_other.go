//go:build !darwin && !linux && !windows

package settings

import "fmt"

func addPathToUserEnvironment(executablePath string) error {
	return fmt.Errorf("adding the application to PATH is not supported on this platform")
}

func addContextMenuForExecutable(executablePath string) error {
	return unsupportedContextMenuError()
}
