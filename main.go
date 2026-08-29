package main

import (
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"zashiki/internal/filemanager"
	"zashiki/internal/lanshare"
	"zashiki/internal/mediastream"
	"zashiki/internal/nativefs"
	"zashiki/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

const htmlAssetPrefix = "/__html_assets__/"

// htmlAssetMiddleware intercepts requests to /__html_assets__/ and serves local files.
// The path after the prefix is a base64-encoded absolute file path.
// This allows HTML previews to reference local static resources (CSS, JS, images, etc.)
// through the WebView's HTTP server instead of file:/// URLs which are blocked.
func htmlAssetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, htmlAssetPrefix) {
			next.ServeHTTP(rw, req)
			return
		}

		encodedPath := strings.TrimPrefix(req.URL.Path, htmlAssetPrefix)
		decodedPath, err := base64.URLEncoding.DecodeString(encodedPath)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("invalid path encoding"))
			return
		}
		localPath := string(decodedPath)

		// Security: only allow absolute paths and prevent traversal
		if !filepath.IsAbs(localPath) {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("path must be absolute"))
			return
		}
		cleanPath := filepath.Clean(localPath)
		if cleanPath != localPath {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("path contains traversal"))
			return
		}

		info, err := os.Stat(cleanPath)
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write([]byte("file not found"))
			return
		}
		if info.IsDir() {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("path is a directory"))
			return
		}
		if info.Size() > 10*1024*1024 {
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte("file too large (>10MB)"))
			return
		}

		data, err := os.ReadFile(cleanPath)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte("failed to read file"))
			return
		}

		mimeType := mime.TypeByExtension(filepath.Ext(cleanPath))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		rw.Header().Set("Content-Type", mimeType)
		rw.Header().Set("Cache-Control", "no-cache")
		rw.WriteHeader(http.StatusOK)
		rw.Write(data)
	})
}

const thumbnailPrefix = "/__thumbnails__/"

// maxRawThumbnailFallbackBytes 限制无法解码格式的原样回退大小；
// 超过该值的大图宁可 404 让前端显示图标，也不把整图塞进 WebView。
const maxRawThumbnailFallbackBytes = 20 * 1024 * 1024

// assetMiddleware 组合本地资源中间件：/__html_assets__/ 与 /__thumbnails__/
// 各自拦截，其余请求交给内置资源服务器。注意这两个中间件响应的都是小文件
// （≤10MB/≤20MB）；音频视频与 Office/PDF 的大文件流式传输走独立的
// mediastream loopback 服务器——Windows 上 Wails 资源管线会把整个响应体
// 缓冲进内存，大文件必须绕开它。
func assetMiddleware(next http.Handler) http.Handler {
	return htmlAssetMiddleware(thumbnailMiddleware(next))
}

// thumbnailMiddleware intercepts requests to /__thumbnails__/{base64 path}?s={size}
// and serves a scaled-down image. Formats Go cannot rasterize (svg/avif/heic)
// fall back to serving the original bytes so the WebView can render them.
func thumbnailMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, thumbnailPrefix) {
			next.ServeHTTP(rw, req)
			return
		}

		encodedPath := strings.TrimPrefix(req.URL.Path, thumbnailPrefix)
		decodedPath, err := base64.URLEncoding.DecodeString(encodedPath)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("invalid path encoding"))
			return
		}
		localPath := string(decodedPath)

		// Security: same validation as htmlAssetMiddleware.
		if !filepath.IsAbs(localPath) {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("path must be absolute"))
			return
		}
		cleanPath := filepath.Clean(localPath)
		if cleanPath != localPath {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("path contains traversal"))
			return
		}

		info, err := os.Stat(cleanPath)
		if err != nil || info.IsDir() {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write([]byte("file not found"))
			return
		}

		thumbSize := filemanager.NormalizeThumbnailSize(parseThumbnailSizeQuery(req.URL.Query().Get("s")))
		etag := fmt.Sprintf(`"%x-%d-%d"`, info.ModTime().UnixNano(), info.Size(), thumbSize)
		if ifNoneMatchMatches(req.Header.Get("If-None-Match"), etag) {
			rw.WriteHeader(http.StatusNotModified)
			return
		}

		result, err := filemanager.GenerateThumbnail(cleanPath, thumbSize)
		if err != nil {
			if errors.Is(err, filemanager.ErrThumbnailUnsupported) {
				// 原样回退：WebView 自行渲染 svg/avif/heic 等格式。
				if result.MimeType == "" || info.Size() > maxRawThumbnailFallbackBytes {
					rw.WriteHeader(http.StatusNotFound)
					rw.Write([]byte("thumbnail unsupported"))
					return
				}
				data, err := os.ReadFile(cleanPath)
				if err != nil {
					rw.WriteHeader(http.StatusNotFound)
					rw.Write([]byte("file not found"))
					return
				}
				writeThumbnail(rw, result.MimeType, data)
				return
			}
			rw.WriteHeader(http.StatusNotFound)
			rw.Write([]byte("thumbnail generation failed"))
			return
		}
		writeThumbnail(rw, result.MimeType, result.Data)
	})
}

func writeThumbnail(rw http.ResponseWriter, mimeType string, data []byte) {
	rw.Header().Set("Content-Type", mimeType)
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Length", strconv.Itoa(len(data)))
	rw.WriteHeader(http.StatusOK)
	rw.Write(data)
}

func parseThumbnailSizeQuery(raw string) int {
	size, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return size
}

func ifNoneMatchMatches(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "" {
		return false
	}
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag || candidate == "*" {
			return true
		}
	}
	return false
}

// resolveInitialDir 从命令行参数解析启动目录。支持 `zashiki /path/to/dir` 形式：
// 取第一个非空且可解析为目录的绝对路径参数；无法解析时返回空串，交由前端回退到 home。
func resolveInitialDir(args []string) string {
	for _, arg := range args {
		if arg == "" {
			continue
		}
		abs, err := filepath.Abs(arg)
		if err != nil {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		return abs
	}
	return ""
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and runs the application, logging any error that might occur.
func main() {

	fileService := &filemanager.FileService{}
	initialDir := resolveInitialDir(os.Args[1:])
	fileService.SetInitialDir(initialDir)

	lanShareService := lanshare.NewLanShareService()

	// 大文件流式预览（音视频 / Office / PDF）走独立 loopback 服务器，
	// 避免经 Wails 资源管线把整个响应体缓冲进内存。
	mediaStreamService := mediastream.NewService()
	if err := mediaStreamService.Start(); err != nil {
		log.Printf("media stream server unavailable, large previews disabled: %v", err)
	}
	defer mediaStreamService.Stop()

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "Zashiki",
		Description: "A file explorer",
		Services: []application.Service{
			application.NewService(fileService),
			application.NewService(&settings.SettingsService{}),
			application.NewService(nativefs.NewFileTransferService()),
			application.NewService(lanShareService),
			application.NewService(mediaStreamService),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: assetMiddleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:          "Zashiki",
		EnableFileDrop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(0, 0, 0),
		URL:              "/",
		Width:            1200,
		Height:           800,
	})

	// 转发系统文件拖入事件给前端（含落点元素详情），由前端映射到目标目录并
	// 执行复制/移动。
	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		details := event.Context().DropTargetDetails()
		app.Event.Emit("window:files-dropped", map[string]any{
			"files":   files,
			"details": details,
		})
	})

	// 注入原生窗口句柄提供器：原生拖出（SHDoDragDrop / 拖拽会话）需要窗口句柄。
	nativefs.SetWindowProvider(func() uintptr {
		return uintptr(win.NativeWindow())
	})

	// 注入主线程调度器：AppKit 拖拽会话必须在主线程启动。
	nativefs.SetMainThreadDispatcher(func(fn func()) {
		application.InvokeSync(fn)
	})

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
