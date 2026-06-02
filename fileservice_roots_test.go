package main

import (
	"os"
	"testing"
)

func TestFileService_GetRoots(t *testing.T) {
	s := &FileService{}
	roots := s.GetRoots()
	if len(roots) == 0 {
		t.Fatal("GetRoots() returned no roots")
	}
	for _, root := range roots {
		if root.Name == "" {
			t.Errorf("root name is empty for path %q", root.Path)
		}
		info, err := os.Stat(root.Path)
		if err != nil {
			t.Errorf("root path %q is not statable: %v", root.Path, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("root path %q is not a directory", root.Path)
		}
		if root.TotalSpace > 0 && root.FreeSpace > root.TotalSpace {
			t.Errorf("root path %q has free space greater than total space: free=%d total=%d", root.Path, root.FreeSpace, root.TotalSpace)
		}
	}
}
