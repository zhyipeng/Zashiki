package filemanager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mholt/archives"
)

type archiveTestFile struct {
	name, content string
	mode          os.FileMode
}

func writeArchiveZIP(t *testing.T, filename string, files ...archiveTestFile) {
	t.Helper()
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
		if file.mode != 0 {
			header.SetMode(file.mode)
		}
		entry, err := w.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	for _, format := range []string{"zip", "tar", "tar.gz"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "资料")
			if err := os.MkdirAll(filepath.Join(source, "空目录"), 0o755); err != nil {
				t.Fatal(err)
			}
			content := "你好，归档文件\n"
			if err := os.WriteFile(filepath.Join(source, "说明.txt"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			service := &FileService{}
			filename := filepath.Join(dir, "output."+format)
			if _, err := service.CreateArchive(context.Background(), []string{source}, filename, format); err != nil {
				t.Fatal(err)
			}
			page, err := service.ListArchive(context.Background(), filename, "", 0, 200)
			if err != nil || page.Total != 1 || page.Entries[0].Path != "资料" || !page.Entries[0].IsDir {
				t.Fatalf("root = %+v, %v", page, err)
			}
			page, err = service.ListArchive(context.Background(), filename, "资料", 0, 200)
			if err != nil || page.Total != 2 || !page.Entries[0].IsDir {
				t.Fatalf("children = %+v, %v", page, err)
			}
			preview, err := service.GetArchiveEntryPreview(context.Background(), filename, "资料/说明.txt")
			if err != nil || preview.Kind != "text" || preview.Content != content {
				t.Fatalf("preview = %+v, %v", preview, err)
			}
			output := filepath.Join(dir, "extract")
			if _, err := service.ExtractArchive(context.Background(), filename, output, nil); err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(filepath.Join(output, "资料", "说明.txt"))
			if err != nil || string(actual) != content {
				t.Fatalf("extracted = %q, %v", actual, err)
			}
			if info, err := os.Stat(filepath.Join(output, "资料", "空目录")); err != nil || !info.IsDir() {
				t.Fatalf("empty directory missing: %v", err)
			}
		})
	}
}

func TestArchiveDirectoryPagingAndCacheInvalidation(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "archive.zip")
	writeArchiveZIP(t, filename, archiveTestFile{name: "sub/b.txt", content: "b"}, archiveTestFile{name: "sub/a.txt", content: "a"}, archiveTestFile{name: "root.txt", content: "root"})
	service := &FileService{}
	ctx := context.Background()
	page, err := service.ListArchive(ctx, filename, "", 0, 1)
	if err != nil || page.Total != 2 || len(page.Entries) != 1 || page.Entries[0].Path != "sub" {
		t.Fatalf("page = %+v, %v", page, err)
	}
	page, err = service.ListArchive(ctx, filename, "sub", 1, 1)
	if err != nil || page.Total != 2 || page.Entries[0].Name != "b.txt" {
		t.Fatalf("page = %+v, %v", page, err)
	}
	page, err = service.ListArchive(ctx, filename, "sub", 999, 1)
	if err != nil || len(page.Entries) != 0 {
		t.Fatalf("past end = %+v, %v", page, err)
	}
	writeArchiveZIP(t, filename, archiveTestFile{name: "new.txt", content: "new content changes size"})
	page, err = service.ListArchive(ctx, filename, "", 0, 200)
	if err != nil || page.Total != 1 || page.Entries[0].Name != "new.txt" {
		t.Fatalf("stale cache = %+v, %v", page, err)
	}
	if _, err := service.ListArchive(ctx, filename, "missing", 0, 200); err == nil {
		t.Fatal("accepted missing directory")
	}
}

func TestArchiveExtractSelectionAndExistingDestination(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "archive.zip")
	writeArchiveZIP(t, filename, archiveTestFile{name: "sub/a.txt", content: "a"}, archiveTestFile{name: "submarine.txt", content: "b"})
	service := &FileService{}
	output := filepath.Join(dir, "extract")
	if _, err := service.ExtractArchive(context.Background(), filename, output, []string{"sub"}); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(output, "sub", "a.txt")); err != nil || string(content) != "a" {
		t.Fatalf("selected file: %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(output, "submarine.txt")); !os.IsNotExist(err) {
		t.Fatal("extracted unselected file")
	}
	if _, err := service.ExtractArchive(context.Background(), filename, output, nil); err == nil {
		t.Fatal("accepted existing destination")
	}
	if content, err := os.ReadFile(filepath.Join(output, "sub", "a.txt")); err != nil || string(content) != "a" {
		t.Fatal("modified existing destination")
	}
	if _, err := service.ExtractArchive(context.Background(), filename, filepath.Join(dir, "bad"), []string{"missing"}); err == nil {
		t.Fatal("accepted missing selection")
	}
}

func TestArchiveRejectsUnsafeAndDuplicatePaths(t *testing.T) {
	for _, name := range []string{"../escape.txt", "/absolute.txt", `C:\escape.txt`, `..\escape.txt`, "a/../../escape.txt", `\\server\share\file`} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			filename := filepath.Join(dir, "unsafe.zip")
			writeArchiveZIP(t, filename, archiveTestFile{name: name, content: "escape"})
			output := filepath.Join(dir, "extract")
			if _, err := (&FileService{}).ExtractArchive(context.Background(), filename, output, nil); err == nil {
				t.Fatal("accepted unsafe archive")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("left output on failure")
			}
		})
	}
	for _, files := range [][]archiveTestFile{
		{{name: "a.txt"}, {name: "a.txt"}},
		{{name: "a/b.txt"}, {name: "a"}},
		{{name: "a"}, {name: "a/b.txt"}},
	} {
		filename := filepath.Join(t.TempDir(), "conflict.zip")
		writeArchiveZIP(t, filename, files...)
		if _, err := (&FileService{}).ListArchive(context.Background(), filename, "", 0, 200); err == nil {
			t.Fatal("accepted conflicting paths")
		}
	}
}

func TestArchiveRejectsLinksAndCleansPartialOutput(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "links.zip")
	writeArchiveZIP(t, filename, archiveTestFile{name: "first.txt", content: "first"}, archiveTestFile{name: "link", content: "../outside", mode: os.ModeSymlink | 0o777})
	service := &FileService{}
	page, err := service.ListArchive(context.Background(), filename, "", 0, 200)
	if err != nil || !page.Entries[1].IsLink {
		t.Fatalf("link entry = %+v, %v", page, err)
	}
	output := filepath.Join(dir, "extract")
	if _, err := service.ExtractArchive(context.Background(), filename, output, nil); err == nil {
		t.Fatal("extracted symlink")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("partial output not cleaned")
	}
	// Hard links can have a regular file mode: LinkTarget must also be checked.
	tarname := filepath.Join(dir, "hardlink.tar")
	f, err := os.Create(tarname)
	if err != nil {
		t.Fatal(err)
	}
	w := tar.NewWriter(f)
	if err := w.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "../outside", Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExtractArchive(context.Background(), tarname, output, nil); err == nil {
		t.Fatal("extracted hard link")
	}
}

func TestArchivePreviewLimitsAndImage(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "preview.zip")
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	writeArchiveZIP(t, filename,
		archiveTestFile{name: "text.txt", content: strings.Repeat("中", maxTextPreviewBytes)},
		archiveTestFile{name: "image.png", content: imageData.String()},
		archiveTestFile{name: "binary.bin", content: "\x00\x01\x02"},
		archiveTestFile{name: "document.pdf", content: "%PDF-1.4\nASCII-only PDF header"},
		archiveTestFile{name: "huge.png", content: strings.Repeat("a", maxImagePreviewBytes+1)},
	)
	service := &FileService{}
	preview, err := service.GetArchiveEntryPreview(context.Background(), filename, "text.txt")
	if err != nil || !preview.Truncated || !utf8.ValidString(preview.Content) || len(preview.Content) > maxTextPreviewBytes {
		t.Fatalf("text preview size=%d truncated=%v err=%v", len(preview.Content), preview.Truncated, err)
	}
	preview, err = service.GetArchiveEntryPreview(context.Background(), filename, "image.png")
	if err != nil || preview.Kind != "image" || preview.DataURL != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(imageData.Bytes()) {
		t.Fatalf("image preview kind=%s err=%v", preview.Kind, err)
	}
	for _, name := range []string{"binary.bin", "huge.png", "document.pdf"} {
		preview, err = service.GetArchiveEntryPreview(context.Background(), filename, name)
		if err != nil || preview.Kind != "unsupported" {
			t.Fatalf("%s preview kind=%s err=%v", name, preview.Kind, err)
		}
	}
	if _, err := service.GetArchiveEntryPreview(context.Background(), filename, "missing"); err == nil {
		t.Fatal("accepted missing preview entry")
	}
}

func TestArchiveCancellationAndCreateValidation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := &FileService{}
	filename := filepath.Join(dir, "archive.zip")
	if _, err := service.CreateArchive(context.Background(), []string{source}, filename, "zip"); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateArchive(context.Background(), []string{source}, filename, "zip"); err == nil {
		t.Fatal("overwrote archive")
	}
	current, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(original, current) {
		t.Fatal("existing archive changed")
	}
	if _, err := service.CreateArchive(context.Background(), []string{dir}, filepath.Join(dir, "recursive.zip"), "zip"); err == nil {
		t.Fatal("allowed output in source directory")
	}
	if _, err := service.CreateArchive(context.Background(), nil, filepath.Join(dir, "empty.zip"), "zip"); err == nil {
		t.Fatal("allowed empty sources")
	}
	if _, err := service.CreateArchive(context.Background(), []string{source}, filepath.Join(dir, "bad.rar"), "rar"); err == nil {
		t.Fatal("allowed unsupported writer")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ListArchive(ctx, filename, "", 0, 200); !errors.Is(err, context.Canceled) {
		t.Fatalf("list cancellation: %v", err)
	}
	if _, err := service.GetArchiveEntryPreview(ctx, filename, "source.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("preview cancellation: %v", err)
	}
	output := filepath.Join(dir, "cancelled")
	if _, err := service.ExtractArchive(ctx, filename, output, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("extract cancellation: %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("created output after cancellation")
	}
	if _, err := service.CreateArchive(ctx, []string{source}, output, "zip"); !errors.Is(err, context.Canceled) {
		t.Fatalf("create cancellation: %v", err)
	}
}

func TestArchiveCorruptInput(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "broken.zip")
	if err := os.WriteFile(filename, []byte("PK\x03\x04broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (&FileService{}).ListArchive(context.Background(), filename, "", 0, 200); err == nil {
		t.Fatal("accepted corrupt archive")
	}
}

func TestArchiveRead7zAndRAR(t *testing.T) {
	dir := t.TempDir()
	// A minimal RAR4 archive using the Store method, with real header/data CRCs.
	var rar bytes.Buffer
	rar.WriteString("Rar!\x1a\x07\x00")
	block := func(kind byte, flags uint16, data []byte) {
		header := make([]byte, 7+len(data))
		header[2] = kind
		binary.LittleEndian.PutUint16(header[3:5], flags)
		binary.LittleEndian.PutUint16(header[5:7], uint16(len(header)))
		copy(header[7:], data)
		binary.LittleEndian.PutUint16(header[:2], uint16(crc32.ChecksumIEEE(header[2:])))
		rar.Write(header)
	}
	block(0x73, 0, make([]byte, 6))
	content, name := []byte("hello from rar\n"), "hello.txt"
	header := make([]byte, 25+len(name))
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(content)))
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(content)))
	header[8] = 2
	binary.LittleEndian.PutUint32(header[9:13], crc32.ChecksumIEEE(content))
	binary.LittleEndian.PutUint32(header[13:17], 0x50210000)
	header[17], header[18] = 20, 0x30
	binary.LittleEndian.PutUint16(header[19:21], uint16(len(name)))
	binary.LittleEndian.PutUint32(header[21:25], 0x20)
	copy(header[25:], name)
	block(0x74, 0x8000, header)
	rar.Write(content)
	block(0x7b, 0, nil)
	rarname := filepath.Join(dir, "plain.rar")
	if err := os.WriteFile(rarname, rar.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{filepath.Join("testdata", "archives", "plain.7z"), rarname} {
		t.Run(filepath.Ext(filename), func(t *testing.T) {
			service := &FileService{}
			page, err := service.ListArchive(context.Background(), filename, "", 0, 200)
			if err != nil || page.Total == 0 {
				t.Fatalf("list = %+v, %v", page, err)
			}
			output := filepath.Join(t.TempDir(), "extract")
			if _, err := service.ExtractArchive(context.Background(), filename, output, nil); err != nil {
				t.Fatal(err)
			}
			for _, entry := range page.Entries {
				if entry.IsDir {
					continue
				}
				preview, err := service.GetArchiveEntryPreview(context.Background(), filename, entry.Path)
				if err != nil || preview.Kind != "text" {
					t.Fatalf("preview kind=%s, %v", preview.Kind, err)
				}
				actual, err := os.ReadFile(filepath.Join(output, entry.Name))
				if err != nil || string(actual) != preview.Content || len(actual) == 0 {
					t.Fatalf("extracted differs from preview, %v", err)
				}
			}
		})
	}
}

func TestArchiveOutputCapturesFinalFlushFailure(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "closed.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	writer := &archiveOutputWriter{writer: file}
	// With no input files the ZIP central directory is written only during Close.
	_ = (archives.Zip{}).Archive(context.Background(), writer, nil)
	if !errors.Is(writer.err, os.ErrClosed) {
		t.Fatalf("final flush error was lost: %v", writer.err)
	}
}
