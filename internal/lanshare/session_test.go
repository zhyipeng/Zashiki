package lanshare

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddFilesSingleAndDir(t *testing.T) {
	sess := newSession()
	dir := t.TempDir()
	sub := filepath.Join(dir, "子目录")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	single := writeTempFile(t, dir, "说明.md", "# hi")
	writeTempFile(t, sub, "a.txt", "A")
	writeTempFile(t, sub, "photo.jpg", "JPEGDATA")

	added, err := sess.addFiles([]string{single, sub}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 3 {
		t.Fatalf("added = %d, want 3", len(added))
	}
	names := map[string]bool{}
	for _, item := range added {
		names[item.Name] = true
		if item.Kind != ShareItemKindFile {
			t.Errorf("%s: kind %v, want file", item.Name, item.Kind)
		}
		if strings.Contains(item.Name, "\\") {
			t.Errorf("%s: display name must use slashes", item.Name)
		}
	}
	for _, want := range []string{"说明.md", "子目录/a.txt", "子目录/photo.jpg"} {
		if !names[want] {
			t.Errorf("missing expanded name %q, got %v", want, names)
		}
	}
	if len(sess.list()) != 3 {
		t.Errorf("session items = %d, want 3", len(sess.list()))
	}
}

func TestAddFilesImageDetection(t *testing.T) {
	sess := newSession()
	dir := t.TempDir()
	img := writeTempFile(t, dir, "pic.PNG", "png")
	txt := writeTempFile(t, dir, "doc.txt", "text")
	added, err := sess.addFiles([]string{img, txt}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !added[0].IsImage || added[0].MIMEType != "image/png" {
		t.Errorf("png item = %+v", added[0])
	}
	if added[1].IsImage {
		t.Error("txt should not be an image")
	}
}

func TestAddFilesMissingPath(t *testing.T) {
	sess := newSession()
	added, err := sess.addFiles([]string{filepath.Join(t.TempDir(), "nope.bin")}, time.Now())
	if err == nil {
		t.Error("missing path should error")
	}
	if len(added) != 0 {
		t.Errorf("added = %d, want 0", len(added))
	}
}

func TestAddTextAndPreview(t *testing.T) {
	sess := newSession()
	item := sess.addText("第一行内容\n第二行", time.Now())
	if item.Kind != ShareItemKindText {
		t.Errorf("kind = %v", item.Kind)
	}
	if item.Name != "第一行内容" {
		t.Errorf("name = %q", item.Name)
	}
	if item.Size != int64(len("第一行内容\n第二行")) {
		t.Errorf("size = %d", item.Size)
	}
	if item.Text != "第一行内容\n第二行" {
		t.Errorf("text lost")
	}

	long := strings.Repeat("字", 40)
	preview := sess.addText(long, time.Now())
	if !strings.HasSuffix(preview.Name, "…") || len([]rune(preview.Name)) > 25 {
		t.Errorf("long preview = %q", preview.Name)
	}
}

func TestRemoveAndClear(t *testing.T) {
	sess := newSession()
	dir := t.TempDir()
	p := writeTempFile(t, dir, "x.txt", "x")
	added, err := sess.addFiles([]string{p}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	id := added[0].ID
	text := sess.addText("note", time.Now())

	if !sess.remove(id) {
		t.Error("remove should report true for existing id")
	}
	if sess.remove(id) {
		t.Error("remove should report false for unknown id")
	}
	if _, ok := sess.resolvePath(id); ok {
		t.Error("path mapping should be removed with the item")
	}
	if len(sess.list()) != 1 {
		t.Fatalf("after remove, items = %d, want 1", len(sess.list()))
	}
	sess.clear()
	if len(sess.list()) != 0 {
		t.Errorf("after clear, items = %d, want 0", len(sess.list()))
	}
	if sess.itemByID(text.ID) != nil {
		t.Error("clear should drop byID entries")
	}
}

func TestResolveFileItemsSkipsText(t *testing.T) {
	sess := newSession()
	dir := t.TempDir()
	p := writeTempFile(t, dir, "f.bin", "f")
	sess.addText("only text", time.Now())
	added, _ := sess.addFiles([]string{p}, time.Now())
	sess.addText("second text", time.Now())

	files := sess.resolveFileItems()
	if len(files) != 1 || files[0].ID != added[0].ID {
		t.Errorf("resolveFileItems = %+v, want only the file", files)
	}
}
