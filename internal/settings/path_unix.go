//go:build darwin || linux

package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const zashikiPathMarker = "# Added by Zashiki"

func addPathToUserEnvironment(pathDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home directory: %w", err)
	}

	shell := os.Getenv("SHELL")
	configPath := shellConfigPath(home, shell)
	if filepath.Base(configPath) == "config.fish" {
		if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
			return fmt.Errorf("create fish config directory: %w", err)
		}
	}

	if err := addPathEntryToShellConfig(configPath, pathDir, shell); err != nil {
		return err
	}

	return addPathToProcessEnvironment(pathDir)
}

func addPathToProcessEnvironment(pathDir string) error {
	current := os.Getenv("PATH")
	for _, entry := range strings.Split(current, string(os.PathListSeparator)) {
		if equalPathEntry(entry, pathDir) {
			return nil
		}
	}

	if current == "" {
		return os.Setenv("PATH", pathDir)
	}
	return os.Setenv("PATH", current+string(os.PathListSeparator)+pathDir)
}

func equalPathEntry(left, right string) bool {
	return filepath.Clean(strings.TrimSpace(left)) == filepath.Clean(strings.TrimSpace(right))
}

func shellConfigPath(home, shell string) string {
	switch filepath.Base(shell) {
	case "zsh":
		return filepath.Join(home, ".zprofile")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	case "bash":
		for _, name := range []string{".bash_profile", ".bash_login", ".bashrc"} {
			candidate := filepath.Join(home, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		return filepath.Join(home, ".profile")
	default:
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".zprofile")
		}
		return filepath.Join(home, ".profile")
	}
}

func addPathEntryToShellConfig(configPath, pathDir, shell string) error {
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read shell config: %w", err)
	}

	content := string(data)
	pathLine := shellPathLine(pathDir, shell)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != zashikiPathMarker {
			continue
		}

		if i+1 == len(lines) {
			lines = append(lines, pathLine)
		} else {
			lines[i+1] = pathLine
		}
		return writeShellConfig(configPath, strings.Join(lines, "\n"))
	}

	if strings.Contains(content, pathDir) {
		return nil
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	content += zashikiPathMarker + "\n" + pathLine + "\n"
	return writeShellConfig(configPath, content)
}

func writeShellConfig(configPath, content string) error {
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write shell config: %w", err)
	}
	return nil
}

func shellPathLine(pathDir, shell string) string {
	quotedPath := shellSingleQuote(pathDir)
	if filepath.Base(shell) == "fish" {
		return "set -gx PATH " + quotedPath + " $PATH"
	}
	return "export PATH=" + quotedPath + ":$PATH"
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
