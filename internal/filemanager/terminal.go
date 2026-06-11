package filemanager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (f *FileService) OpenFile(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "linux":
		return exec.Command("xdg-open", path).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	default:
		return exec.Command("open", path).Start()
	}
}

func (f *FileService) OpenInFileManager(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "linux":
		dir := path
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			dir = filepath.Dir(path)
		}
		return exec.Command("xdg-open", dir).Start()
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	default:
		return exec.Command("open", path).Start()
	}
}

func (f *FileService) OpenTerminal(path string, terminalProgram string) error {
	dir, err := terminalDir(path)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		app := "Terminal"
		if terminalProgram != "" {
			app = terminalProgram
		}
		return exec.Command("open", "-a", app, dir).Start()
	case "linux":
		if terminalProgram != "" {
			return exec.Command(terminalProgram, "--working-directory", dir).Start()
		}
		return startLinuxTerminal(dir)
	case "windows":
		if terminalProgram != "" {
			cmd := exec.Command(terminalProgram)
			cmd.Dir = dir
			return cmd.Start()
		}
		return exec.Command("cmd", "/C", "start", "", "cmd", "/K", "cd", "/d", dir).Start()
	default:
		return exec.Command("open", dir).Start()
	}
}

func terminalDir(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return path, nil
	}
	return filepath.Dir(path), nil
}

func startLinuxTerminal(dir string) error {
	commands := [][]string{
		{"x-terminal-emulator", "--working-directory", dir},
		{"gnome-terminal", "--working-directory", dir},
		{"konsole", "--workdir", dir},
		{"xfce4-terminal", "--working-directory", dir},
		{"xterm", "-e", "sh", "-c", "cd \"$1\" && exec sh", "sh", dir},
	}
	var lastErr error
	for _, command := range commands {
		if _, err := exec.LookPath(command[0]); err != nil {
			lastErr = err
			continue
		}
		if err := exec.Command(command[0], command[1:]...).Start(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no terminal application found")
}
