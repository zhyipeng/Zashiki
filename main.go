package main

import (
	"embed"
	"encoding/base64"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zashiki/internal/filemanager"
	"zashiki/internal/lanshare"
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
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: htmlAssetMiddleware,
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
