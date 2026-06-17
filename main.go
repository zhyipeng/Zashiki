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
	"time"

	"zashiki/internal/filemanager"
	"zashiki/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
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

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "Zashiki",
		Description: "A file explorer",
		Services: []application.Service{
			application.NewService(&filemanager.FileService{}),
			application.NewService(&settings.SettingsService{}),
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
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Zashiki",
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

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
