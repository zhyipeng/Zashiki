package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func mediaRequest(t *testing.T, localPath string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, mediaPrefix+base64.URLEncoding.EncodeToString([]byte(localPath)), nil)
	for key, value := range header {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler := mediaMiddleware(http.NotFoundHandler())
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestMediaMiddleware_ServesFullContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "song.mp3")
	content := []byte("fake mp3 bytes")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := mediaRequest(t, path, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "audio/mpeg" {
		t.Fatalf("Content-Type = %q, want audio/mpeg", contentType)
	}
	if acceptRanges := recorder.Header().Get("Accept-Ranges"); acceptRanges != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", acceptRanges)
	}
	if recorder.Body.String() != string(content) {
		t.Fatalf("body = %q, want file content", recorder.Body.String())
	}
}

func TestMediaMiddleware_SupportsRangeRequests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.mp4")
	content := []byte("0123456789abcdef")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := mediaRequest(t, path, map[string]string{"Range": "bytes=2-5"})
	if recorder.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", recorder.Code)
	}
	if body := recorder.Body.String(); body != "2345" {
		t.Fatalf("body = %q, want 2345", body)
	}
	if contentRange := recorder.Header().Get("Content-Range"); contentRange != "bytes 2-5/16" {
		t.Fatalf("Content-Range = %q, want bytes 2-5/16", contentRange)
	}
}

func TestMediaMiddleware_NoSizeCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.mkv")

	// 超过 htmlAssetMiddleware 的 10MB 上限也应可流式播放。
	data := make([]byte, 10*1024*1024+1)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	recorder := mediaRequest(t, path, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "video/x-matroska" {
		t.Fatalf("Content-Type = %q, want video/x-matroska", contentType)
	}
	if length := recorder.Header().Get("Content-Length"); length == "" {
		t.Fatal("Content-Length should be set for full responses")
	}
}

func TestMediaMiddleware_RejectsBadRequests(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name   string
		target string
	}{
		{"invalid encoding", mediaPrefix + "!!not-base64!!"},
		{"relative path", mediaPrefix + base64.URLEncoding.EncodeToString([]byte("song.mp3"))},
		{"traversal path", mediaPrefix + base64.URLEncoding.EncodeToString([]byte(filepath.Join(dir, "..", "escape.mp3")))},
		{"missing file", mediaPrefix + base64.URLEncoding.EncodeToString([]byte(filepath.Join(dir, "nope.mp3")))},
		{"directory", mediaPrefix + base64.URLEncoding.EncodeToString([]byte(dir))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			recorder := httptest.NewRecorder()
			handler := mediaMiddleware(http.NotFoundHandler())
			handler.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusBadRequest && recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 400 or 404", recorder.Code)
			}
		})
	}
}

func TestMediaMiddleware_PassesOtherPathsThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	recorder := httptest.NewRecorder()
	handler := mediaMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusTeapot)
	}))
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418 from next handler", recorder.Code)
	}
}
