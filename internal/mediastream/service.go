// Package mediastream 提供一个只绑 127.0.0.1 的令牌门控 HTTP 服务器，
// 用 http.ServeContent 把本地文件流式喂给 WebView 的原生媒体/文档查看器。
//
// 为什么不走 Wails 资源服务器中间件：Windows 上 Wails v3 的 WebView2 管线
// 会把整个响应体先缓冲进 Go 内存、再整体复制进 Win32 内存流才返回
// （internal/assetserver/webview/responsewriter_windows.go），大视频会直接
// 撑爆内存。WebView2 对非 wails.localhost 的请求走原生网络栈，因此独立的
// loopback HTTP 服务器才能实现真流式与原生 Range 拖动。
package mediastream

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"zashiki/internal/filemanager"
)

// Service 是 Wails-facing 的流式文件服务器。Start 后 GetBaseURL 把
// 含随机令牌的根地址暴露给前端，前端用 encodeAssetPath 兼容的
// base64url 路径段拼接资源 URL。
type Service struct {
	mu     sync.Mutex
	token  string
	port   int
	server *http.Server
}

func NewService() *Service {
	return &Service{}
}

// Start 绑定 127.0.0.1 的临时端口并在后台伺服。重复调用是幂等的。
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}

	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return fmt.Errorf("generate media token: %w", err)
	}
	s.token = base64.RawURLEncoding.EncodeToString(token)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("bind media stream server: %w", err)
	}
	s.port = listener.Addr().(*net.TCPAddr).Port

	server := &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.server = server
	go func() {
		_ = server.Serve(listener)
	}()
	return nil
}

// Stop 关闭服务器（进行中的请求有 3 秒收尾时间）。
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	server := s.server
	s.server = nil
	return server.Shutdown(ctx)
}

// GetBaseURL 返回含令牌的流式根地址，例如
// http://127.0.0.1:53101/media/<token>；服务器未启动时返回空串。
func (s *Service) GetBaseURL() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return "", nil
	}
	return "http://127.0.0.1:" + strconv.Itoa(s.port) + "/media/" + s.token, nil
}

func (s *Service) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /media/{token}/{resource}", s.handleResource)
	return mux
}

// handleResource 流式返回一个本地文件。令牌不匹配时按 404 拒绝。
// ServeContent 负责 Content-Length、Range/206（视频拖动进度条）与
// If-Modified-Since；文件句柄按需读取，内存占用与响应块大小无关。
func (s *Service) handleResource(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.PathValue("token")), []byte(s.tokenValue())) != 1 {
		http.NotFound(w, r)
		return
	}

	encoded := r.PathValue("resource")
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		http.Error(w, "invalid path encoding", http.StatusBadRequest)
		return
	}
	localPath := string(decoded)
	if !filepath.IsAbs(localPath) {
		http.Error(w, "path must be absolute", http.StatusBadRequest)
		return
	}
	cleanPath := filepath.Clean(localPath)
	if cleanPath != localPath {
		http.Error(w, "path contains traversal", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(cleanPath)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		http.Error(w, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 允许 WebView 内的 fetch/媒体元素跨源读取（wails.localhost / vite dev
	// 端口都不同于本服务器）。资源 URL 自带高熵令牌，开放 CORS 不扩大攻击面。
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if mimeType := filemanager.FileMimeType(info.Name()); mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// tokenValue 读取当前令牌；服务器未启动时返回不可能匹配的占位值。
func (s *Service) tokenValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return "<stopped>"
	}
	return s.token
}
