package filemanager

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

func (f *FileService) TrashEntries(paths []string) ([]string, error) {
	trashed := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := validateDeletePath(path); err != nil {
			return trashed, err
		}
		if err := trashEntry(path); err != nil {
			return trashed, err
		}
		trashed = append(trashed, path)
	}
	return trashed, nil
}
