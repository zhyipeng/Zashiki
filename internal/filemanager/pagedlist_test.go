package filemanager

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// seedDirWithFiles 在 dir 下创建 n 个文件（file-0000.txt ...），返回 fileName 各命名的有序 list。
func seedDirWithFiles(t *testing.T, dir string, n int) []string {
	t.Helper()
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("file-%04d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
		names = append(names, name)
	}
	return names
}

func TestFileService_ListDirPage_Pagination(t *testing.T) {
	dir := t.TempDir()
	names := seedDirWithFiles(t, dir, 1200)

	s := &FileService{}
	page, err := s.ListDirPage(dir, 0, 500)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if page.Total != 1200 {
		t.Fatalf("Total = %d, want 1200", page.Total)
	}
	if len(page.Entries) != 500 {
		t.Fatalf("first page len = %d, want 500", len(page.Entries))
	}
	if page.Entries[0].Name != names[0] {
		t.Errorf("first entry = %q, want %q", page.Entries[0].Name, names[0])
	}

	// 第二页
	page2, err := s.ListDirPage(dir, 500, 500)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if len(page2.Entries) != 500 {
		t.Fatalf("second page len = %d, want 500", len(page2.Entries))
	}
	if page2.Entries[0].Name != names[500] {
		t.Errorf("second page first = %q, want %q", page2.Entries[0].Name, names[500])
	}

	// 余数页
	page3, err := s.ListDirPage(dir, 1000, 500)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if len(page3.Entries) != 200 {
		t.Fatalf("last page len = %d, want 200", len(page3.Entries))
	}

	// 拼接所有页应覆盖全部条目且无重复
	var joined []string
	for _, pg := range []DirPage{page, page2, page3} {
		for _, e := range pg.Entries {
			joined = append(joined, e.Name)
		}
	}
	if len(joined) != 1200 {
		t.Fatalf("joined len = %d, want 1200", len(joined))
	}
	if !reflect.DeepEqual(joined, names) {
		t.Errorf("paginated order mismatch:\n got %v\nwant %v", joined, names)
	}
}

func TestFileService_ListDirPage_OffsetOutOfRange(t *testing.T) {
	dir := t.TempDir()
	seedDirWithFiles(t, dir, 50)

	s := &FileService{}
	page, err := s.ListDirPage(dir, 500, 500)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if page.Total != 50 {
		t.Errorf("Total = %d, want 50", page.Total)
	}
	if len(page.Entries) != 0 {
		t.Errorf("Entries len = %d, want 0", len(page.Entries))
	}
}

func TestFileService_ListDirPage_ConsistentOrdering(t *testing.T) {
	dir := t.TempDir()
	seedDirWithFiles(t, dir, 300)
	if err := os.MkdirAll(filepath.Join(dir, "zdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "afile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	all, err := s.ListDir(dir)
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}

	var paged []FileEntry
	offset := 0
	for {
		pg, err := s.ListDirPage(dir, offset, 100)
		if err != nil {
			t.Fatalf("ListDirPage() error = %v", err)
		}
		paged = append(paged, pg.Entries...)
		if len(paged) >= pg.Total {
			break
		}
		offset += 100
	}

	if len(paged) != len(all) {
		t.Fatalf("paged len = %d, ListDir len = %d", len(paged), len(all))
	}
	for i := range all {
		if all[i].Name != paged[i].Name || all[i].IsDir != paged[i].IsDir {
			t.Errorf("entry %d mismatch: ListDir=%+v paged=%+v", i, all[i], paged[i])
		}
	}
}

func TestFileService_ListDirPage_CacheReuse(t *testing.T) {
	dir := t.TempDir()
	seedDirWithFiles(t, dir, 10)

	s := &FileService{}
	first, err := s.ListDirPage(dir, 0, 500)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if first.Total != 10 {
		t.Fatalf("first Total = %d, want 10", first.Total)
	}

	// 目录新增文件后，mtime 变化应触发重新枚举（缓存失效）。
	newFile := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(newFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	second, err := s.ListDirPage(dir, 0, 500)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if second.Total != 11 {
		t.Fatalf("after add Total = %d, want 11 (cache should refresh)", second.Total)
	}
}

func TestFileService_ListDirPage_NotFound(t *testing.T) {
	s := &FileService{}
	if _, err := s.ListDirPage("/nonexistent/path", 0, 10); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestFileService_ListDirPage_LimitNormalization(t *testing.T) {
	dir := t.TempDir()
	seedDirWithFiles(t, dir, 30)

	s := &FileService{}
	// limit <= 0 -> 默认 500（覆盖全部）
	p, err := s.ListDirPage(dir, 0, 0)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if len(p.Entries) != 30 {
		t.Errorf("default-limit returned %d entries, want 30", len(p.Entries))
	}

	// 超大 limit 归一化到 5000，不越界、不 panic。
	p2, err := s.ListDirPage(dir, 0, 99999)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if len(p2.Entries) != 30 {
		t.Errorf("capped-limit returned %d entries, want 30", len(p2.Entries))
	}

	// offset < 0 归一化为 0。
	p3, err := s.ListDirPage(dir, -5, 30)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	if len(p3.Entries) != 30 {
		t.Errorf("negative-offset returned %d entries, want 30", len(p3.Entries))
	}
}

func TestFileService_ListDirPage_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	s := &FileService{}
	page, err := s.ListDirPage(dir, 0, 10)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	found := false
	for _, e := range page.Entries {
		if e.Name == "link.txt" {
			found = true
			if !e.IsSymlink {
				t.Errorf("link.txt IsSymlink = false, want true")
			}
			if e.IsDir {
				t.Errorf("link.txt IsDir = true, want false")
			}
			if e.LinkTarget != target {
				t.Errorf("link.txt LinkTarget = %q, want %q", e.LinkTarget, target)
			}
		}
	}
	if !found {
		t.Error("link.txt not found in page")
	}
}

func TestFileService_ListDirPage_DotfilesHiddenOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dotfile hidden convention is unix-specific")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	page, err := s.ListDirPage(dir, 0, 10)
	if err != nil {
		t.Fatalf("ListDirPage() error = %v", err)
	}
	for _, e := range page.Entries {
		if e.Name == ".hidden" && !e.IsHidden {
			t.Errorf(".hidden IsHidden = false, want true")
		}
		if e.Name == "visible.txt" && e.IsHidden {
			t.Errorf("visible.txt IsHidden = true, want false")
		}
	}
}
