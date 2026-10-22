package appupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

type testHost struct{}

func (testHost) Emit(string, ...any) bool                              { return true }
func (testHost) OnEvent(string, func(any)) func()                      { return func() {} }
func (testHost) OpenWindow(updater.WindowOptions) updater.WindowHandle { panic("unexpected window") }
func (testHost) Quit()                                                 { panic("unexpected restart") }

func TestMatchAsset(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "Zashiki.dmg"}, {Name: "Zashiki-amd64-installer.exe"},
		{Name: "Zashiki-windows-amd64-update.zip"}, {Name: "Zashiki-darwin-universal-update.zip"},
		{Name: "Zashiki-linux-amd64-update.tar.gz"},
	}
	for _, tt := range []struct {
		platform, arch string
		want           int
	}{
		{"darwin", "arm64", 3}, {"darwin", "amd64", 3}, {"windows", "amd64", 2},
		{"linux", "amd64", 4}, {"windows", "arm64", -1}, {"linux", "arm64", -1}, {"android", "arm64", -1},
	} {
		t.Run(tt.platform+"/"+tt.arch, func(t *testing.T) {
			if got := matchAsset(updater.CheckRequest{Platform: tt.platform, Arch: tt.arch}, assets); got != tt.want {
				t.Fatalf("matchAsset = %d, want %d", got, tt.want)
			}
		})
	}
	if got := matchAsset(updater.CheckRequest{Platform: "windows", Arch: "amd64"}, assets[:2]); got != -1 {
		t.Fatal("must not select installer")
	}
}

func TestDisabledReason(t *testing.T) {
	for _, tt := range []struct {
		platform, arch, exe, appImage string
		debug, disabled               bool
	}{
		{"darwin", "arm64", "/Applications/Zashiki.app/Contents/MacOS/Zashiki", "", false, false},
		{"darwin", "arm64", "/tmp/Zashiki", "", false, true},
		{"windows", "amd64", "Zashiki.exe", "", false, false},
		{"windows", "amd64", "Zashiki.exe", "", true, true},
		{"linux", "amd64", "/usr/local/bin/Zashiki", "", false, true},
		{"linux", "amd64", "/home/user/Zashiki", "", false, false},
		{"linux", "amd64", "/tmp/.mount/Zashiki", "/home/user/Zashiki.AppImage", false, true},
		{"android", "arm64", "/app/Zashiki", "", false, true},
	} {
		if got := disabledReason(tt.platform, tt.arch, tt.exe, tt.appImage, tt.debug); (got != "") != tt.disabled {
			t.Errorf("%+v: disabled reason = %q", tt, got)
		}
	}
}

type fixture struct {
	service   *UpdateService
	downloads atomic.Int32
	mode      atomic.Int32
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create("Zashiki.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("new application")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(archive.Bytes()))
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + Repository + "/releases/latest":
			if f.mode.Load() == 4 {
				http.Error(w, "unavailable", 503)
				return
			}
			tag := "v1.1.0"
			if f.mode.Load() == 3 {
				tag = "v1.0.0"
			}
			assets := []map[string]any{{"name": "Zashiki-windows-amd64-update.zip", "size": archive.Len(), "browser_download_url": server.URL + "/artifact"}}
			if f.mode.Load() != 1 {
				assets = append(assets, map[string]any{"name": "SHA256SUMS.txt", "browser_download_url": server.URL + "/checksum"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "body": "release notes", "assets": assets})
		case "/checksum":
			hash := digest
			if f.mode.Load() == 2 {
				hash = strings.Repeat("0", 64)
			}
			fmt.Fprintf(w, "%s  Zashiki-windows-amd64-update.zip\n", hash)
		case "/artifact":
			f.downloads.Add(1)
			_, _ = w.Write(archive.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	p, err := github.New(github.Config{Repository: Repository, BaseURL: server.URL, ChecksumAsset: "SHA256SUMS.txt", AssetMatcher: matchAsset, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	engine := updater.New(testHost{})
	if err := engine.Init(updater.Config{CurrentVersion: "1.0.0", Platform: "windows", Arch: "amd64", Providers: []updater.Provider{verifiedProvider{p}}, Window: updater.WindowNone}); err != nil {
		t.Fatal(err)
	}
	f.service = NewUpdateService("1.0.0")
	f.service.engine, f.service.ctx, f.service.status.Enabled = engine, context.Background(), true
	t.Cleanup(func() {
		if path := engine.DownloadedPath(); path != "" {
			_ = os.RemoveAll(filepath.Dir(path))
		}
	})
	return f
}

func TestCheckRequiresConfirmationBeforeDownload(t *testing.T) {
	f := newFixture(t)
	if err := f.service.Download(); err == nil {
		t.Fatal("download without check succeeded")
	}
	if err := f.service.Restart(); err == nil {
		t.Fatal("restart without download succeeded")
	}
	status, err := f.service.Check()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != updater.StateAvailable || status.Version != "1.1.0" || status.Notes != "release notes" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if f.downloads.Load() != 0 {
		t.Fatal("check downloaded the update before confirmation")
	}
	if err := f.service.Download(); err != nil {
		t.Fatal(err)
	}
	if f.service.GetStatus().State != updater.StateReady {
		t.Fatal("update not ready")
	}
	content, err := os.ReadFile(f.service.engine.DownloadedPath())
	if err != nil || string(content) != "new application" {
		t.Fatalf("staged content: %q, %v", content, err)
	}
	if _, err := f.service.Check(); err != nil {
		t.Fatal(err)
	}
	if err := f.service.Download(); err != nil {
		t.Fatal(err)
	}
	if f.downloads.Load() != 1 {
		t.Fatal("ready update was downloaded again")
	}
}

func TestRejectsMissingChecksum(t *testing.T) {
	f := newFixture(t)
	f.mode.Store(1)
	if _, err := f.service.Check(); err == nil {
		t.Fatal("missing checksum accepted")
	}
	if err := f.service.Download(); err == nil {
		t.Fatal("download without checksum accepted")
	}
	if f.downloads.Load() != 0 {
		t.Fatal("unverified artifact downloaded")
	}
}

func TestRejectsCorruptDownload(t *testing.T) {
	f := newFixture(t)
	f.mode.Store(2)
	if _, err := f.service.Check(); err != nil {
		t.Fatal(err)
	}
	if err := f.service.Download(); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	if f.service.GetStatus().State != updater.StateError || f.service.engine.DownloadedPath() != "" {
		t.Fatal("corrupt update was staged")
	}
	if err := f.service.Restart(); err == nil {
		t.Fatal("corrupt update allowed restart")
	}
}

func TestRecheckInvalidatesStalePendingRelease(t *testing.T) {
	for _, mode := range []int32{3, 4} {
		f := newFixture(t)
		if _, err := f.service.Check(); err != nil {
			t.Fatal(err)
		}
		f.mode.Store(mode)
		_, _ = f.service.Check()
		if err := f.service.Download(); err == nil {
			t.Fatal("stale pending release accepted")
		}
	}
}

func TestRejectsConcurrentOperations(t *testing.T) {
	f := newFixture(t)
	f.service.op.Lock()
	defer f.service.op.Unlock()
	if _, err := f.service.Check(); err == nil {
		t.Fatal("concurrent check accepted")
	}
	if err := f.service.Download(); err == nil {
		t.Fatal("concurrent download accepted")
	}
	if err := f.service.Restart(); err == nil {
		t.Fatal("concurrent restart accepted")
	}
}
