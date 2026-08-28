package lanshare

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestServer(t *testing.T) (*shareServer, *session, string) {
	t.Helper()
	origDelay := badAttemptDelay
	badAttemptDelay = 0
	t.Cleanup(func() { badAttemptDelay = origDelay })

	sess := newSession()
	receiveDir := t.TempDir()
	srv := &shareServer{
		sess:    sess,
		token:   "testtoken",
		receive: func() string { return receiveDir },
	}
	return srv, sess, receiveDir
}

func doReq(t *testing.T, srv *shareServer, method, target string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

func TestAuthRejectsWrongToken(t *testing.T) {
	srv, _, _ := newTestServer(t)
	for _, target := range []string{
		"/wrongtoken/items",
		"/items",       // no token segment: must not match item route
		"/wrongtoken/", // page subtree with wrong token
	} {
		rec := doReq(t, srv, "GET", target, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: got status %d, want 404", target, rec.Code)
		}
	}
	// Correct token passes.
	rec := doReq(t, srv, "GET", "/testtoken/items", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("correct token: got status %d, want 200", rec.Code)
	}
}

func TestHandleItems(t *testing.T) {
	srv, sess, _ := newTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sess.addFiles([]string{p}, time.Now())

	rec := doReq(t, srv, "GET", "/testtoken/items", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	var payload ItemsChanged
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Name != "a.txt" {
		t.Errorf("items = %+v, want one a.txt", payload.Items)
	}
	if payload.Items[0].Kind != ShareItemKindFile {
		t.Errorf("kind = %v, want file", payload.Items[0].Kind)
	}
}

func TestHandlePage(t *testing.T) {
	srv, _, _ := newTestServer(t)
	rec := doReq(t, srv, "GET", "/testtoken/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("content type %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "Zashiki") {
		t.Error("page body should contain Zashiki branding")
	}
}

func TestHandleDownload(t *testing.T) {
	srv, sess, _ := newTestServer(t)
	dir := t.TempDir()
	content := []byte("zashiki file content 123")
	p := filepath.Join(dir, "报告.png")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := sess.addFiles([]string{p}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	item := added[0]
	if !item.IsImage {
		t.Fatalf("png should be detected as image")
	}

	t.Run("attachment download", func(t *testing.T) {
		rec := doReq(t, srv, "GET", "/testtoken/download/"+item.ID, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		if !bytes.Equal(rec.Body.Bytes(), content) {
			t.Error("body mismatch")
		}
		disp := rec.Header().Get("Content-Disposition")
		if !strings.HasPrefix(disp, "attachment") {
			t.Errorf("disposition %q, want attachment", disp)
		}
		if !strings.Contains(disp, "filename*=UTF-8''") {
			t.Errorf("disposition %q missing UTF-8 filename*", disp)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "image/png") {
			t.Errorf("content type %q, want image/png", ct)
		}
	})

	t.Run("inline preview", func(t *testing.T) {
		rec := doReq(t, srv, "GET", "/testtoken/download/"+item.ID+"?preview=1", nil)
		if disp := rec.Header().Get("Content-Disposition"); strings.Contains(disp, "attachment") {
			t.Errorf("preview should not force attachment, got %q", disp)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		rec := doReq(t, srv, "GET", "/testtoken/download/deadbeef", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want 404", rec.Code)
		}
	})
}

func TestHandleZip(t *testing.T) {
	srv, sess, _ := newTestServer(t)
	dir := t.TempDir()
	subs := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subs, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		filepath.Join(dir, "one.txt"):  []byte("first"),
		filepath.Join(subs, "two.txt"): []byte("second"),
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sess.addFiles([]string{dir}, time.Now()); err != nil {
		t.Fatal(err)
	}

	rec := doReq(t, srv, "GET", "/testtoken/zip", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	reader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("zip entries = %d, want 2", len(reader.File))
	}
	got := map[string]string{}
	for _, f := range reader.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(data)
	}
	// The shared root is a temp dir, so expansion prefixes entry names with it.
	prefix := filepath.Base(dir) + "/"
	if got[prefix+"one.txt"] != "first" || got[prefix+"sub/two.txt"] != "second" {
		t.Errorf("zip contents = %v", got)
	}
}

func TestHandleUpload(t *testing.T) {
	srv, _, receiveDir := newTestServer(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, _ := writer.CreateFormFile("files", "上传 文件.txt")
	fw.Write([]byte("uploaded content"))
	// A non-file field and an empty filename part must be skipped.
	_ = writer.WriteField("note", "hello")
	fw2, _ := writer.CreateFormFile("other", "")
	fw2.Write([]byte("ignored"))
	writer.Close()

	req := httptest.NewRequest("POST", "/testtoken/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(receiveDir, "上传 文件.txt"))
	if err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
	if string(data) != "uploaded content" {
		t.Errorf("content = %q", data)
	}
	if _, err := os.Stat(filepath.Join(receiveDir, "other")); !os.IsNotExist(err) {
		t.Error("empty filename part should not create a file")
	}
}

func TestHandleUploadCollision(t *testing.T) {
	srv, _, receiveDir := newTestServer(t)
	if err := os.WriteFile(filepath.Join(receiveDir, "dup.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, _ := writer.CreateFormFile("files", "dup.txt")
	fw.Write([]byte("new"))
	writer.Close()

	req := httptest.NewRequest("POST", "/testtoken/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	old, _ := os.ReadFile(filepath.Join(receiveDir, "dup.txt"))
	if string(old) != "old" {
		t.Error("existing file must not be overwritten")
	}
	renamed, err := os.ReadFile(filepath.Join(receiveDir, "dup (1).txt"))
	if err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if string(renamed) != "new" {
		t.Errorf("renamed content = %q", renamed)
	}
}

func TestHandleText(t *testing.T) {
	srv, _, _ := newTestServer(t)
	var received []TextReceived
	srv.onText = func(tr TextReceived) { received = append(received, tr) }

	rec := doReq(t, srv, "POST", "/testtoken/text", strings.NewReader(`{"text":"你好，Zashiki"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if len(received) != 1 || received[0].Text != "你好，Zashiki" {
		t.Fatalf("received = %+v", received)
	}
	if received[0].RemoteAddr == "" {
		t.Error("remote addr should be recorded")
	}

	// Empty text is rejected.
	rec = doReq(t, srv, "POST", "/testtoken/text", strings.NewReader(`{"text":"  "}`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty text: status %d, want 400", rec.Code)
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"report.txt", "report.txt"},
		{"报告.docx", "报告.docx"},
		{`C:\fakepath\photo.png`, "photo.png"},
		{"../../etc/passwd", "passwd"},
		{"..\\..\\secret", "secret"},
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
		{"/", "unnamed"},
	}
	for _, c := range cases {
		if got := sanitizeFileName(c.in); got != c.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := uniquePath(dir, "new.txt")
	if p != filepath.Join(dir, "new.txt") {
		t.Errorf("first unique = %q", p)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := uniquePath(dir, "new.txt")
	if second != filepath.Join(dir, "new (1).txt") {
		t.Errorf("second unique = %q", second)
	}
	// Extensionless names collide without a dangling ".ext" split.
	if err := os.WriteFile(filepath.Join(dir, "makefile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := uniquePath(dir, "makefile"); got != filepath.Join(dir, "makefile (1)") {
		t.Errorf("extensionless unique = %q", got)
	}
}

func TestContentDisposition(t *testing.T) {
	h := contentDisposition("attachment", "file.txt")
	if h != `attachment; filename="file.txt"; filename*=UTF-8''file.txt` {
		t.Errorf("ascii header = %q", h)
	}
	h = contentDisposition("attachment", "报告 v1.png")
	if !strings.Contains(h, `filename*=UTF-8''%E6%8A%A5%E5%91%8A%20v1.png`) {
		t.Errorf("utf8 header = %q", h)
	}
	if strings.Contains(h, `"报告`) {
		t.Errorf("quoted fallback must be ascii: %q", h)
	}
}

func TestStartPortFallback(t *testing.T) {
	srv, _, _ := newTestServer(t)
	if err := srv.start(0); err != nil {
		t.Fatalf("start: %v", err)
	}
	if srv.port == 0 {
		t.Error("port should be the bound ephemeral port")
	}
	if err := srv.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestStopClearsSession(t *testing.T) {
	srv, sess, _ := newTestServer(t)
	if err := srv.start(0); err != nil {
		t.Fatal(err)
	}
	text := sess.addText("hello", time.Now())
	if text.ID == "" {
		t.Fatal("text item not added")
	}
	if err := srv.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if items := sess.list(); len(items) != 0 {
		t.Errorf("items after stop = %d, want 0", len(items))
	}
}
