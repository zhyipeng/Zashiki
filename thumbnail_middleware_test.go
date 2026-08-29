package main

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeMiddlewarePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 64, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func thumbnailRequest(t *testing.T, localPath string, sizeQuery string) *httptest.ResponseRecorder {
	t.Helper()
	target := thumbnailPrefix + base64.URLEncoding.EncodeToString([]byte(localPath))
	if sizeQuery != "" {
		target += "?" + sizeQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	handler := thumbnailMiddleware(http.NotFoundHandler())
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestThumbnailMiddleware_ServesScaledPNG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "photo.png")
	writeMiddlewarePNG(t, path, 200, 100)

	recorder := thumbnailRequest(t, path, "s=50")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", contentType)
	}
	decoded, err := png.Decode(recorder.Body)
	if err != nil {
		t.Fatalf("response body is not a valid png: %v", err)
	}
	if bounds := decoded.Bounds(); bounds.Dx() != 50 || bounds.Dy() != 25 {
		t.Fatalf("thumbnail size = %dx%d, want 50x25", bounds.Dx(), bounds.Dy())
	}
}

func TestThumbnailMiddleware_ServesRawFallbackForSVG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "logo.svg")
	body := `<svg xmlns="http://www.w3.org/2000/svg"><rect width="4" height="4"/></svg>`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := thumbnailRequest(t, path, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/svg+xml" {
		t.Fatalf("Content-Type = %q, want image/svg+xml", contentType)
	}
	if recorder.Body.String() != body {
		t.Fatal("svg should be served as-is")
	}
}

func TestThumbnailMiddleware_NonImageNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := thumbnailRequest(t, path, "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestThumbnailMiddleware_ExeWithoutIconNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.exe")
	if err := os.WriteFile(path, []byte("not a pe file"), 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := thumbnailRequest(t, path, "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (exe bytes must never be served to <img>)", recorder.Code)
	}
	if recorder.Body.String() == "not a pe file" {
		t.Fatal("raw exe bytes must not be served as thumbnail fallback")
	}
}

func TestThumbnailMiddleware_RejectsBadRequests(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name   string
		target string
	}{
		{"relative path", thumbnailPrefix + base64.URLEncoding.EncodeToString([]byte("relative.png"))},
		{"traversal path", thumbnailPrefix + base64.URLEncoding.EncodeToString([]byte(filepath.Join(dir, "..", "escape.png")))},
		{"missing file", thumbnailPrefix + base64.URLEncoding.EncodeToString([]byte(filepath.Join(dir, "nope.png")))},
		{"directory", thumbnailPrefix + base64.URLEncoding.EncodeToString([]byte(dir))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			recorder := httptest.NewRecorder()
			handler := thumbnailMiddleware(http.NotFoundHandler())
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusBadRequest && recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 400 or 404", recorder.Code)
			}
		})
	}
}

func TestThumbnailMiddleware_ETagNotModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "etag.png")
	writeMiddlewarePNG(t, path, 16, 16)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	etag := fmt.Sprintf(`"%x-%d-%d"`, info.ModTime().UnixNano(), info.Size(), 256)

	req := httptest.NewRequest(
		http.MethodGet,
		thumbnailPrefix+base64.URLEncoding.EncodeToString([]byte(path)),
		nil,
	)
	req.Header.Set("If-None-Match", etag)
	recorder := httptest.NewRecorder()
	handler := thumbnailMiddleware(http.NotFoundHandler())
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 for matching If-None-Match", recorder.Code)
	}
}

func TestThumbnailMiddleware_PassesOtherPathsThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	recorder := httptest.NewRecorder()
	handler := thumbnailMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusTeapot)
	}))
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418 from next handler", recorder.Code)
	}
}
