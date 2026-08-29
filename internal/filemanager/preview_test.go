package filemanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileService_GetFilePreview_Text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello\nworld"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "text" {
		t.Fatalf("Kind = %q, want text", preview.Kind)
	}
	if preview.Content != "hello\nworld" {
		t.Fatalf("Content = %q, want file content", preview.Content)
	}
	if preview.Truncated {
		t.Fatal("Truncated should be false")
	}
}

func TestFileService_GetFilePreview_TruncatedText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.log")
	content := strings.Repeat("a", maxTextPreviewBytes+8)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "text" {
		t.Fatalf("Kind = %q, want text", preview.Kind)
	}
	if len(preview.Content) != maxTextPreviewBytes {
		t.Fatalf("Content length = %d, want %d", len(preview.Content), maxTextPreviewBytes)
	}
	if !preview.Truncated {
		t.Fatal("Truncated should be true")
	}
}

func TestFileService_GetFilePreview_Image(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pixel.png")
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54,
		0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00,
		0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "image" {
		t.Fatalf("Kind = %q, want image", preview.Kind)
	}
	if preview.MimeType != "image/png" {
		t.Fatalf("MimeType = %q, want image/png", preview.MimeType)
	}
	if !strings.HasPrefix(preview.DataURL, "data:image/png;base64,") {
		t.Fatalf("DataURL = %q, want png data URL", preview.DataURL)
	}
}

func TestFileService_GetFilePreview_Binary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(path, []byte{0x00, 0xff, 0x01}, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "unsupported" {
		t.Fatalf("Kind = %q, want unsupported", preview.Kind)
	}
}

func TestFileService_GetFilePreview_Directory(t *testing.T) {
	dir := t.TempDir()

	s := &FileService{}
	preview, err := s.GetFilePreview(dir)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "unsupported" {
		t.Fatalf("Kind = %q, want unsupported", preview.Kind)
	}
	if preview.Message == "" {
		t.Fatal("Message should explain unsupported directory preview")
	}
}

func TestFileService_SaveTextPreview(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	updated, err := s.SaveTextPreview(path, "after", preview.Version)
	if err != nil {
		t.Fatalf("SaveTextPreview() error = %v", err)
	}

	if updated.Kind != "text" || updated.Content != "after" {
		t.Fatalf("SaveTextPreview() = %+v, want updated text preview", updated)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "after" {
		t.Fatalf("file content = %q, want after", string(data))
	}
}

func TestFileService_GetFilePreview_Office(t *testing.T) {
	dir := t.TempDir()
	content := []byte("fake docx content")

	tests := []struct {
		name     string
		ext      string
		wantMime string
	}{
		{"docx", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"xlsx", ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"pptx", ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, "file"+tt.ext)
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatal(err)
			}

			s := &FileService{}
			preview, err := s.GetFilePreview(path)
			if err != nil {
				t.Fatalf("GetFilePreview() error = %v", err)
			}

			if preview.Kind != "office" {
				t.Fatalf("Kind = %q, want office", preview.Kind)
			}
			if preview.MimeType != tt.wantMime {
				t.Fatalf("MimeType = %q, want %q", preview.MimeType, tt.wantMime)
			}
			// 元数据化：内容由前端经 mediastream 流式获取，不再走 base64 IPC。
			if preview.DataURL != "" {
				t.Fatal("DataURL should stay empty for office preview")
			}
			if preview.Content != "" {
				t.Fatal("Content should stay empty for office preview")
			}
			if preview.Size != int64(len(content)) {
				t.Fatalf("Size = %d, want %d", preview.Size, len(content))
			}
		})
	}
}

func TestFileService_GetFilePreview_OfficeTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.docx")

	// Create a file larger than maxOfficePreviewBytes
	data := make([]byte, maxOfficePreviewBytes+1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "unsupported" {
		t.Fatalf("Kind = %q, want unsupported (too large)", preview.Kind)
	}
	if preview.Message == "" {
		t.Fatal("Message should be non-empty for oversized file")
	}
}

func TestFileService_GetFilePreview_Pdf(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	content := []byte("%PDF-1.4 fake pdf content")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "pdf" {
		t.Fatalf("Kind = %q, want pdf", preview.Kind)
	}
	if preview.MimeType != "application/pdf" {
		t.Fatalf("MimeType = %q, want application/pdf", preview.MimeType)
	}
	if preview.DataURL != "" {
		t.Fatal("DataURL should stay empty for pdf preview")
	}
	if preview.Size != int64(len(content)) {
		t.Fatalf("Size = %d, want %d", preview.Size, len(content))
	}
}

func TestFileService_GetFilePreview_PdfTooLarge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.pdf")

	data := make([]byte, maxPdfPreviewBytes+1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "unsupported" {
		t.Fatalf("Kind = %q, want unsupported (too large)", preview.Kind)
	}
	if preview.Message == "" {
		t.Fatal("Message should be non-empty for oversized file")
	}
}

func TestFileService_GetFilePreview_Media(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		ext      string
		wantKind string
		wantMime string
	}{
		{"mp3", ".mp3", "audio", "audio/mpeg"},
		{"flac", ".flac", "audio", "audio/flac"},
		{"m4a", ".m4a", "audio", "audio/mp4"},
		{"ogg", ".ogg", "audio", "audio/ogg"},
		{"mp4", ".mp4", "video", "video/mp4"},
		{"mkv", ".mkv", "video", "video/x-matroska"},
		{"webm", ".webm", "video", "video/webm"},
		{"mov", ".mov", "video", "video/quicktime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, "file"+tt.ext)
			// 任意二进制内容：媒体 provider 不解析内容，流式交给前端播放。
			if err := os.WriteFile(path, []byte{0x00, 0x01, 0xff}, 0o644); err != nil {
				t.Fatal(err)
			}

			s := &FileService{}
			preview, err := s.GetFilePreview(path)
			if err != nil {
				t.Fatalf("GetFilePreview() error = %v", err)
			}

			if preview.Kind != tt.wantKind {
				t.Fatalf("Kind = %q, want %q", preview.Kind, tt.wantKind)
			}
			if preview.MimeType != tt.wantMime {
				t.Fatalf("MimeType = %q, want %q", preview.MimeType, tt.wantMime)
			}
			if preview.DataURL != "" {
				t.Fatal("DataURL should stay empty for streaming media preview")
			}
			if preview.Content != "" {
				t.Fatal("Content should stay empty for media preview")
			}
		})
	}
}

func TestFileService_GetFilePreview_MediaOversizedStillStreams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.mp3")

	// 媒体走流式播放，不受图片类 10MB 上限约束，也不读入内存。
	data := make([]byte, maxImagePreviewBytes+1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "audio" {
		t.Fatalf("Kind = %q, want audio", preview.Kind)
	}
	if preview.DataURL != "" {
		t.Fatal("DataURL should stay empty for streaming media preview")
	}
}

func TestFileService_GetFilePreview_TypeScriptStaysText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.ts")
	if err := os.WriteFile(path, []byte("const x = 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}

	if preview.Kind != "text" {
		t.Fatalf("Kind = %q, want text (.ts is TypeScript, not MPEG-TS)", preview.Kind)
	}
}

func TestFileService_SaveTextPreviewRejectsChangedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &FileService{}
	preview, err := s.GetFilePreview(path)
	if err != nil {
		t.Fatalf("GetFilePreview() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.SaveTextPreview(path, "after", preview.Version); err == nil {
		t.Fatal("SaveTextPreview() expected conflict error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "external" {
		t.Fatalf("file content = %q, want external", string(data))
	}
}
