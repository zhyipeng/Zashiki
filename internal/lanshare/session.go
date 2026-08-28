package lanshare

import (
	"errors"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxShareFiles caps the expanded file count of one share to keep a runaway
// folder selection from flooding the session and the receiver page.
const maxShareFiles = 10000

// ErrTooManyFiles is returned when expanding a selection exceeds maxShareFiles.
var ErrTooManyFiles = errors.New("分享文件数超出上限")

// imageExt set for receiver-side inline preview. SVG is deliberately excluded:
// it is the only raster alternative that can carry scripts.
var imageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".avif": true,
}

// session owns the share item list: items in insertion order plus the opaque
// file-ID → on-disk path mapping that download/zip handlers resolve against.
type session struct {
	mu    sync.RWMutex
	items []ShareItem
	byID  map[string]ShareItem
	paths map[string]string
}

func newSession() *session {
	return &session{
		byID:  make(map[string]ShareItem),
		paths: make(map[string]string),
	}
}

// addFiles adds one entry per path; directories are expanded to all regular
// files beneath them with folder-prefixed display names.
func (s *session) addFiles(paths []string, now time.Time) ([]ShareItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var added []ShareItem
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return added, err
		}
		if info.IsDir() {
			items, err := s.expandDirLocked(path, info.Name(), now)
			if err != nil {
				return added, err
			}
			added = append(added, items...)
			continue
		}
		item, err := s.addFileLocked(path, info.Name(), info.Size(), now)
		if err != nil {
			return added, err
		}
		added = append(added, item)
	}
	return added, nil
}

// expandDirLocked walks dir and adds every regular file (symlinks excluded,
// matching WalkDir behaviour) with display names prefixed by the folder name.
func (s *session) expandDirLocked(dir, displayName string, now time.Time) ([]ShareItem, error) {
	var added []ShareItem
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Unreadable entries are skipped; siblings keep sharing.
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		if s.lenLocked()+len(added) >= maxShareFiles {
			return ErrTooManyFiles
		}
		item, addErr := s.addFileLocked(path, filepath.ToSlash(filepath.Join(displayName, rel)), info.Size(), now)
		if addErr != nil {
			return addErr
		}
		added = append(added, item)
		return nil
	})
	if errors.Is(err, ErrTooManyFiles) {
		return added, ErrTooManyFiles
	}
	// Other walk errors are already swallowed per entry.
	return added, nil
}

func (s *session) addFileLocked(path, name string, size int64, now time.Time) (ShareItem, error) {
	id := newRandomID()
	ext := strings.ToLower(filepath.Ext(name))
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	item := ShareItem{
		ID:       id,
		Kind:     ShareItemKindFile,
		Name:     name,
		Size:     size,
		IsImage:  imageExt[ext],
		MIMEType: mimeType,
		AddedAt:  now,
	}
	s.items = append(s.items, item)
	s.byID[id] = item
	s.paths[id] = path
	return item, nil
}

// addText shares a text snippet; the display name is a single-line preview.
func (s *session) addText(text string, now time.Time) ShareItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := ShareItem{
		ID:      newRandomID(),
		Kind:    ShareItemKindText,
		Name:    textPreview(text),
		Size:    int64(len(text)),
		Text:    text,
		AddedAt: now,
	}
	s.items = append(s.items, item)
	s.byID[item.ID] = item
	return item
}

// textPreview builds the display name of a text item: its first non-empty
// line, truncated to roughly one visual row.
func textPreview(text string) string {
	firstLine := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			firstLine = strings.TrimSpace(line)
			break
		}
	}
	if firstLine == "" {
		firstLine = "(空白文本)"
	}
	runes := []rune(firstLine)
	if len(runes) > 24 {
		return string(runes[:24]) + "…"
	}
	return firstLine
}

func (s *session) remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return false
	}
	delete(s.byID, id)
	delete(s.paths, id)
	for i, item := range s.items {
		if item.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			break
		}
	}
	return true
}

func (s *session) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = nil
	s.byID = make(map[string]ShareItem)
	s.paths = make(map[string]string)
}

func (s *session) lenLocked() int {
	return len(s.items)
}

func (s *session) list() []ShareItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.items) == 0 {
		return []ShareItem{}
	}
	return append([]ShareItem(nil), s.items...)
}

func (s *session) resolvePath(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, ok := s.paths[id]
	return path, ok
}

func (s *session) itemByID(id string) *ShareItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if item, ok := s.byID[id]; ok {
		return &item
	}
	return nil
}

// resolveFileItems returns file items in insertion order (for the zip listing).
func (s *session) resolveFileItems() []ShareItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var files []ShareItem
	for _, item := range s.items {
		if item.Kind == ShareItemKindFile {
			files = append(files, item)
		}
	}
	return files
}
