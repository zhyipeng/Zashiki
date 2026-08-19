package filemanager

import (
	"context"
	"os"
	"path/filepath"
)

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

func (f *FileService) TrashEntries(ctx context.Context, paths []string) ([]EntryOperationResult, error) {
	emitter := newProgressEmitter(OperationKindTrash, len(paths), paths)
	trashed := make([]EntryOperationResult, 0, len(paths))
	for _, path := range paths {
		if ctx.Err() != nil {
			emitter.finish(ctx.Err(), true)
			return trashed, ctx.Err()
		}
		if err := validateDeletePath(path); err != nil {
			emitter.finish(err, false)
			return trashed, err
		}
		emitter.setCurrentName(filepath.Base(path))
		targetPath, err := trashEntry(ctx, path)
		if err != nil {
			emitter.finish(err, false)
			return trashed, err
		}
		trashed = append(trashed, EntryOperationResult{
			SourcePath: path,
			TargetPath: targetPath,
		})
		emitter.itemDone()
	}
	emitter.finish(nil, false)
	return trashed, nil
}

func uniqueTrashPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	return uniquePath(path)
}
