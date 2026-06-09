//go:build !windows

package filemanager

func isHiddenEntry(name string, _ string) bool {
	if name == "." || name == ".." {
		return false
	}
	return len(name) > 0 && name[0] == '.'
}
