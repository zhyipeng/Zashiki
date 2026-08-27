//go:build windows

package settings

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var sendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

const (
	hwndBroadcast     = 0xffff
	wmSettingChange   = 0x001a
	smtoAbortIfHung   = 0x0002
	pathChangeWaitMs  = 5000
	shcneAssocChanged = 0x08000000
)

var shellChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

func addPathToUserEnvironment(pathDir string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open user environment registry key: %w", err)
	}
	defer key.Close()

	current, valueType, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("read user PATH: %w", err)
	}
	if err == registry.ErrNotExist {
		current = ""
		valueType = registry.SZ
	}

	entries := splitWindowsPath(current)
	for _, entry := range entries {
		if equalWindowsPathEntry(entry, pathDir) {
			notifyEnvironmentChange()
			return addPathToProcessEnvironment(pathDir)
		}
	}
	entries = append(entries, pathDir)
	updated := strings.Join(entries, ";")

	if valueType == registry.EXPAND_SZ {
		if err := key.SetExpandStringValue("Path", updated); err != nil {
			return fmt.Errorf("write user PATH: %w", err)
		}
	} else if err := key.SetStringValue("Path", updated); err != nil {
		return fmt.Errorf("write user PATH: %w", err)
	}

	notifyEnvironmentChange()
	return addPathToProcessEnvironment(pathDir)
}

func notifyEnvironmentChange() {
	environment, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}

	_, _, _ = sendMessageTimeout.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(environment)),
		uintptr(smtoAbortIfHung),
		uintptr(pathChangeWaitMs),
		0,
	)
}

type contextMenuRegistration struct {
	keyPath     string
	placeholder string
}

func addContextMenuForExecutable(executablePath string) error {
	for _, registration := range contextMenuRegistrations() {
		if err := writeContextMenuRegistration(registration, executablePath); err != nil {
			return err
		}
	}

	notifyShellAssociationChange()
	return nil
}

func contextMenuRegistrations() []contextMenuRegistration {
	return []contextMenuRegistration{
		{keyPath: `Software\Classes\Directory\shell\Zashiki`, placeholder: "%1"},
		{keyPath: `Software\Classes\Directory\Background\shell\Zashiki`, placeholder: "%V"},
		{keyPath: `Software\Classes\Drive\shell\Zashiki`, placeholder: "%1"},
	}
}

func writeContextMenuRegistration(registration contextMenuRegistration, executablePath string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registration.keyPath, registry.READ|registry.WRITE)
	if err != nil {
		return fmt.Errorf("open context menu registry key: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue("MUIVerb", "用 Zashiki 打开"); err != nil {
		return fmt.Errorf("write context menu label: %w", err)
	}
	if err := key.SetStringValue("Icon", executablePath); err != nil {
		return fmt.Errorf("write context menu icon: %w", err)
	}

	commandKey, _, err := registry.CreateKey(key, "command", registry.READ|registry.WRITE)
	if err != nil {
		return fmt.Errorf("open context menu command key: %w", err)
	}
	defer commandKey.Close()

	if err := commandKey.SetStringValue("", contextMenuCommand(executablePath, registration.placeholder)); err != nil {
		return fmt.Errorf("write context menu command: %w", err)
	}
	return nil
}

func contextMenuCommand(executablePath, placeholder string) string {
	return fmt.Sprintf(`"%s" "%s"`, executablePath, placeholder)
}

func notifyShellAssociationChange() {
	_, _, _ = shellChangeNotify.Call(uintptr(shcneAssocChanged), 0, 0, 0)
}

func addPathToProcessEnvironment(pathDir string) error {
	current := os.Getenv("PATH")
	for _, entry := range splitWindowsPath(current) {
		if equalWindowsPathEntry(entry, pathDir) {
			return nil
		}
	}

	if current == "" {
		return os.Setenv("PATH", pathDir)
	}
	return os.Setenv("PATH", current+";"+pathDir)
}

func splitWindowsPath(value string) []string {
	parts := strings.Split(value, ";")
	entries := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			entries = append(entries, part)
		}
	}
	return entries
}

func equalWindowsPathEntry(left, right string) bool {
	normalize := func(value string) string {
		value = strings.TrimSpace(strings.Trim(value, `"`))
		return strings.TrimRight(strings.ReplaceAll(value, "/", `\`), `\`)
	}
	return strings.EqualFold(normalize(left), normalize(right))
}
