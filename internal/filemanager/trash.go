package filemanager

import "os"

type TrashInfo struct {
	Label     string `json:"label"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

func (f *FileService) GetTrashInfo() TrashInfo {
	return getTrashInfo()
}

func (f *FileService) OpenTrash() error {
	return openTrash()
}

func (f *FileService) TrashEntries(paths []string) ([]EntryOperationResult, error) {
	trashed := make([]EntryOperationResult, 0, len(paths))
	for _, path := range paths {
		if err := validateDeletePath(path); err != nil {
			return trashed, err
		}
		targetPath, err := trashEntry(path)
		if err != nil {
			return trashed, err
		}
		trashed = append(trashed, EntryOperationResult{
			SourcePath: path,
			TargetPath: targetPath,
		})
	}
	return trashed, nil
}

func uniqueTrashPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	return uniquePath(path)
}
