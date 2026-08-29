package mediastream

import (
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	s := NewService()
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	base, err := s.GetBaseURL()
	if err != nil || base == "" {
		t.Fatalf("GetBaseURL() = %q, %v", base, err)
	}
	return s, base
}

func resourceURL(base, localPath string) string {
	return base + "/" + base64.URLEncoding.EncodeToString([]byte(localPath))
}

func TestService_StreamsFullContent(t *testing.T) {
	_, base := newTestService(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "song.mp3")
	content := []byte("fake mp3 bytes")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(resourceURL(base, path))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "audio/mpeg" {
		t.Fatalf("Content-Type = %q, want audio/mpeg", contentType)
	}
	if acceptRanges := resp.Header.Get("Accept-Ranges"); acceptRanges != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", acceptRanges)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(content) {
		t.Fatalf("body = %q, want file content", body)
	}
}

func TestService_RangeRequest(t *testing.T) {
	_, base := newTestService(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.mp4")
	content := []byte("0123456789abcdef")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodGet, resourceURL(base, path), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=2-5")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "bytes 2-5/16" {
		t.Fatalf("Content-Range = %q, want bytes 2-5/16", contentRange)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "2345" {
		t.Fatalf("body = %q, want 2345", body)
	}
}

func TestService_RejectsBadRequests(t *testing.T) {
	_, base := newTestService(t)
	dir := t.TempDir()
	noToken := strings.Replace(base, "/media/", "/media/wrong-token-", 1)

	tests := []struct {
		name string
		url  string
	}{
		{"wrong token", resourceURL(noToken, filepath.Join(dir, "a.mp3"))},
		{"invalid encoding", base + "/!!bad!!"},
		{"relative path", resourceURL(base, "song.mp3")},
		{"traversal path", resourceURL(base, filepath.Join(dir, "..", "escape.mp3"))},
		{"missing file", resourceURL(base, filepath.Join(dir, "nope.mp3"))},
		{"directory", resourceURL(base, dir)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 404 or 400", resp.StatusCode)
			}
		})
	}
}

func TestService_CORSHeader(t *testing.T) {
	_, base := newTestService(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(resourceURL(base, path))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", origin)
	}
}

func TestService_BaseURLEmptyAfterStop(t *testing.T) {
	s := NewService()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	base, err := s.GetBaseURL()
	if err != nil {
		t.Fatalf("GetBaseURL() error = %v", err)
	}
	if base != "" {
		t.Fatalf("base = %q after stop, want empty", base)
	}
}

func TestService_StartIsIdempotent(t *testing.T) {
	s := NewService()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	first, err := s.GetBaseURL()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	second, err := s.GetBaseURL()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("base changed after re-start: %q -> %q", first, second)
	}
}

func TestService_StopShutsDown(t *testing.T) {
	s, base := newTestService(t)
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	// 关闭后同一连接目标应立即失败（端口已释放）。
	client := &http.Client{Timeout: 2 * time.Second}
	if _, err := client.Get(resourceURL(base, "C:/whatever.mp3")); err == nil {
		t.Fatal("request after stop should fail")
	}
}
