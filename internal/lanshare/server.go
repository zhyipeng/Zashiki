package lanshare

import (
	"archive/zip"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// badAttemptDelay throttles token brute forcing: every rejected request sleeps
// before answering, bounding guesses by latency rather than bandwidth.
// A variable so tests can zero it.
var badAttemptDelay = 300 * time.Millisecond

// errClientGone marks transfers aborted by the receiving side.
var errClientGone = errors.New("client disconnected")

// shareServer is the LAN-facing HTTP server. It is created per start() with a
// fresh token; all state lives in the session and the callbacks.
type shareServer struct {
	sess    *session
	token   string
	port    int
	server  *http.Server
	onText  func(TextReceived)
	receive func() string // receive dir for uploads, resolved lazily
}

// start listens on the requested port, falling back to an ephemeral port when
// it is busy, and serves in a background goroutine. It returns the bound port.
func (s *shareServer) start(requested int) error {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(requested))
	if err != nil {
		listener, err = net.Listen("tcp", ":0")
		if err != nil {
			return err
		}
	}
	s.port = listener.Addr().(*net.TCPAddr).Port

	s.server = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	httpSrv := s.server
	go func() {
		_ = httpSrv.Serve(listener)
	}()
	return nil
}

// routes builds the token-gated mux; extracted so tests can exercise the
// handlers through httptest without binding a real port.
func (s *shareServer) routes() *http.ServeMux {
	mux := http.NewServeMux()
	// The bare-token registration also prevents ServeMux from redirecting
	// "/x" to "/x/" for the {tok}/ subtree, which would mask auth failures.
	mux.HandleFunc("GET /{tok}", s.auth(s.handlePage))
	mux.HandleFunc("GET /{tok}/", s.auth(s.handlePage))
	mux.HandleFunc("GET /{tok}/items", s.auth(s.handleItems))
	mux.HandleFunc("GET /{tok}/download/{id}", s.auth(s.handleDownload))
	mux.HandleFunc("GET /{tok}/zip", s.auth(s.handleZip))
	mux.HandleFunc("POST /{tok}/upload", s.auth(s.handleUpload))
	mux.HandleFunc("POST /{tok}/text", s.auth(s.handleText))
	return mux
}

// stop shuts the server down (in-flight requests get 3s), then clears the
// share list: a stopped share has no meaning for its items anymore.
func (s *shareServer) stop() error {
	if s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	s.server = nil
	s.sess.clear()
	return err
}

// auth wraps a handler with constant-time token validation via the {tok}
// path segment. Failures answer 404 (indistinguishable from no server) after
// the brute-force delay.
func (s *shareServer) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.PathValue("tok")
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) != 1 {
			time.Sleep(badAttemptDelay)
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func (s *shareServer) handlePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(receiverPageHTML))
}

func (s *shareServer) handleItems(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, ItemsChanged{Items: s.sess.list()})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

// countReader counts streamed bytes for download progress.
type countReader struct {
	r  io.Reader
	on func(int64)
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.on(int64(n))
	}
	return n, err
}

// seekableCountReader feeds http.ServeContent while counting read bytes.
type seekableCountReader struct {
	f  *os.File
	on func(int64)
}

func (s *seekableCountReader) Read(p []byte) (int, error) {
	n, err := s.f.Read(p)
	if n > 0 {
		s.on(int64(n))
	}
	return n, err
}

func (s *seekableCountReader) Seek(offset int64, whence int) (int64, error) {
	return s.f.Seek(offset, whence)
}

// handleDownload streams one shared file. ServeContent provides
// Content-Length, Range (video seeking) and Content-Type; the caller only
// sets the download disposition. ?preview=1 serves inline for <img> previews.
func (s *shareServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path, ok := s.sess.resolvePath(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	item := s.sess.itemByID(id)
	if item == nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file unavailable", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "file unavailable", http.StatusNotFound)
		return
	}

	if r.URL.Query().Get("preview") != "1" {
		w.Header().Set("Content-Disposition", contentDisposition("attachment", item.Name))
	}
	w.Header().Set("Content-Type", item.MIMEType)

	emitter := newTransferEmitter(TransferDirectionDownload, clientIP(r), item.Name, info.Size())
	defer func() {
		if r.Context().Err() != nil {
			emitter.finish(errClientGone)
			return
		}
		emitter.finish(nil)
	}()
	reader := &seekableCountReader{f: f, on: emitter.bytes}
	http.ServeContent(w, r, item.Name, info.ModTime(), reader)
}

// handleZip streams every shared file as a single store-mode zip: no
// compression overhead for already-compressed media, and file entries can be
// streamed without buffering the archive in memory.
func (s *shareServer) handleZip(w http.ResponseWriter, r *http.Request) {
	items := s.sess.resolveFileItems()
	name := "zashiki-share-" + time.Now().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition("attachment", name))

	emitter := newTransferEmitter(TransferDirectionDownload, clientIP(r), name, -1)
	defer func() {
		if r.Context().Err() != nil {
			emitter.finish(errClientGone)
			return
		}
		emitter.finish(nil)
	}()

	zipWriter := zip.NewWriter(w)
	for _, item := range items {
		path, ok := s.sess.resolvePath(item.ID)
		if !ok {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		header := &zip.FileHeader{
			Name:               item.Name,
			Method:             zip.Store,
			Modified:           item.AddedAt,
			UncompressedSize64: uint64(item.Size),
		}
		header.SetMode(0o644)
		entry, err := zipWriter.CreateHeader(header)
		if err != nil {
			f.Close()
			continue
		}
		if _, err := io.Copy(entry, &countReader{r: f, on: emitter.bytes}); err != nil {
			f.Close()
			return
		}
		f.Close()
	}
	_ = zipWriter.Close()
}

// handleUpload streams multipart file parts straight to the receive dir.
func (s *shareServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "multipart form required", http.StatusBadRequest)
		return
	}
	dir := s.receive()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "receive dir unavailable", http.StatusInternalServerError)
		return
	}

	var saved []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "upload interrupted", http.StatusBadRequest)
			return
		}
		if part.FormName() != "files" || part.FileName() == "" {
			part.Close()
			continue
		}
		name := sanitizeFileName(part.FileName())
		dest := uniquePath(dir, name)
		emitter := newTransferEmitter(TransferDirectionUpload, clientIP(r), name, -1)
		out, err := os.Create(dest)
		if err != nil {
			emitter.finish(err)
			part.Close()
			continue
		}
		counting := &countWriter{w: out, on: emitter.bytes}
		_, copyErr := io.Copy(counting, part)
		closeErr := out.Close()
		part.Close()
		if copyErr != nil {
			_ = os.Remove(dest)
			emitter.finish(copyErr)
			http.Error(w, "upload interrupted", http.StatusBadRequest)
			return
		}
		emitter.finish(closeErr)
		if closeErr == nil {
			saved = append(saved, filepath.Base(dest))
		}
	}
	writeJSON(w, map[string]any{"saved": saved})
}

// handleText receives a text message pushed from the browser page.
func (s *shareServer) handleText(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxTextBytes+1))
	if err != nil || len(body) > maxTextBytes {
		http.Error(w, "text too large", http.StatusRequestEntityTooLarge)
		return
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.Text) == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	if s.onText != nil {
		s.onText(TextReceived{
			ID:         newRandomID(),
			Text:       payload.Text,
			RemoteAddr: clientIP(r),
			At:         time.Now(),
		})
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// countWriter counts streamed bytes for upload progress.
type countWriter struct {
	w  io.Writer
	on func(int64)
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		c.on(int64(n))
	}
	return n, err
}

const maxTextBytes = 1 << 20 // 1 MB

// sanitizeFileName strips any path components or separators from a
// browser-provided filename; the result is a bare name only.
func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || name == "/" || name == `\` {
		return "unnamed"
	}
	return name
}

// uniquePath returns dir/name, or dir/name (n).ext for the first free n when
// the target already exists.
func uniquePath(dir, name string) string {
	base := name
	ext := ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base = name[:dot]
		ext = name[dot:]
	}
	candidate := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); err != nil {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}

// contentDisposition builds an RFC 6266 header with an ASCII fallback name
// and an RFC 5987 filename* carrying the original UTF-8 name.
func contentDisposition(kind, filename string) string {
	var ascii strings.Builder
	for _, r := range filename {
		if r < 128 && r != '"' && r != '\\' && r > 31 {
			ascii.WriteRune(r)
		} else {
			ascii.WriteByte('_')
		}
	}
	encoded := strings.ReplaceAll(url.PathEscape(filename), "'", "%27")
	return kind + `; filename="` + ascii.String() + `"; filename*=UTF-8''` + encoded
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
