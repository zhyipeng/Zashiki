package filemanager

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func writePNGFile(path string, width, height int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := writePNGFile(path, width, height); err != nil {
		t.Fatal(err)
	}
}

func thumbnailCacheKey(path string, maxSize int) string {
	return filepath.Clean(path) + "\x00" + strconv.Itoa(maxSize)
}

func TestGenerateThumbnail_ScalesToFit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	writeTestPNG(t, path, 200, 100)

	result, err := GenerateThumbnail(path, 50)
	if err != nil {
		t.Fatalf("GenerateThumbnail() error = %v", err)
	}
	if result.MimeType != "image/png" {
		t.Fatalf("MimeType = %q, want image/png", result.MimeType)
	}

	decoded, err := png.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("output is not a valid png: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 25 {
		t.Fatalf("thumbnail size = %dx%d, want 50x25", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateThumbnail_SmallImagePassthrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.png")
	writeTestPNG(t, path, 16, 16)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	result, err := GenerateThumbnail(path, 256)
	if err != nil {
		t.Fatalf("GenerateThumbnail() error = %v", err)
	}
	if string(result.Data) != string(original) {
		t.Fatal("small image should be returned as-is without re-encoding")
	}
	if result.MimeType != "image/png" {
		t.Fatalf("MimeType = %q, want image/png", result.MimeType)
	}
}

func TestGenerateThumbnail_NonImageUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := GenerateThumbnail(path, 256); err != ErrThumbnailUnsupported {
		t.Fatalf("error = %v, want ErrThumbnailUnsupported", err)
	}
}

func TestGenerateThumbnail_UndecodableImageFallsBackWithMime(t *testing.T) {
	dir := t.TempDir()
	svg := filepath.Join(dir, "logo.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := GenerateThumbnail(svg, 256)
	if err != ErrThumbnailUnsupported {
		t.Fatalf("error = %v, want ErrThumbnailUnsupported", err)
	}
	if result.MimeType != "image/svg+xml" {
		t.Fatalf("MimeType = %q, want image/svg+xml for fallback", result.MimeType)
	}

	heic := filepath.Join(dir, "photo.heic")
	if err := os.WriteFile(heic, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateThumbnail(heic, 256); err != ErrThumbnailUnsupported {
		t.Fatalf("heic error = %v, want ErrThumbnailUnsupported", err)
	}
}

func TestGenerateThumbnail_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := GenerateThumbnail(filepath.Join(dir, "nope.png"), 256); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGenerateThumbnail_CacheHitAndInvalidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cached.png")
	writeTestPNG(t, path, 300, 300)

	if _, err := GenerateThumbnail(path, 64); err != nil {
		t.Fatalf("first generate: %v", err)
	}
	if _, err := GenerateThumbnail(path, 64); err != nil {
		t.Fatalf("second generate (cache hit): %v", err)
	}

	key := thumbnailCacheKey(path, 64)
	thumbnailCache.mu.Lock()
	_, ok := thumbnailCache.entries[key]
	thumbnailCache.mu.Unlock()
	if !ok {
		t.Fatalf("thumbnail not cached under key %q", key)
	}

	// 修改文件后缓存应失效并重新生成成功。
	if err := writePNGFile(path, 400, 400); err != nil {
		t.Fatal(err)
	}
	result, err := GenerateThumbnail(path, 64)
	if err != nil {
		t.Fatalf("generate after modify: %v", err)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected non-empty thumbnail after invalidation")
	}
}

func TestNormalizeThumbnailSize(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: DefaultThumbnailSize},
		{in: -5, want: DefaultThumbnailSize},
		{in: 10, want: minThumbnailSize},
		{in: 256, want: 256},
		{in: 4096, want: maxThumbnailSize},
	}
	for _, tc := range tests {
		if got := NormalizeThumbnailSize(tc.in); got != tc.want {
			t.Errorf("NormalizeThumbnailSize(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
