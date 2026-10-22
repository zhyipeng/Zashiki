package appupdate

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

type Status struct {
	CurrentVersion string        `json:"currentVersion"`
	Enabled        bool          `json:"enabled"`
	Reason         string        `json:"reason"`
	State          updater.State `json:"state"`
	Version        string        `json:"version"`
	Notes          string        `json:"notes"`
	Size           int64         `json:"size"`
}

type UpdateService struct {
	mu      sync.RWMutex
	op      sync.Mutex
	engine  *updater.Updater
	ctx     context.Context
	status  Status
	pending bool
}

func NewUpdateService(version string) *UpdateService {
	return &UpdateService{status: Status{CurrentVersion: version, State: updater.StateUnconfigured}}
}

func (s *UpdateService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	app := application.Get()
	exe, err := os.Executable()
	if err != nil {
		s.status.Reason = "无法确定应用安装位置"
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	s.status.Reason = disabledReason(runtime.GOOS, runtime.GOARCH, exe, os.Getenv("APPIMAGE"), app.Env.Info().Debug)
	if s.status.Reason != "" {
		return nil
	}
	provider, err := github.New(github.Config{
		Repository: Repository, AssetMatcher: matchAsset, ChecksumAsset: "SHA256SUMS.txt",
		// 大更新包在慢速网络上可能超过 provider 默认的 30 秒总超时。
		HTTPClient: &http.Client{Timeout: 30 * time.Minute},
	})
	if err != nil {
		return err
	}
	if err := app.Updater.Init(updater.Config{
		CurrentVersion: s.status.CurrentVersion,
		Providers:      []updater.Provider{verifiedProvider{provider}},
		Window:         updater.WindowNone,
	}); err != nil {
		return err
	}
	s.engine, s.ctx = app.Updater, ctx
	s.status.Enabled = true
	return nil
}

func (s *UpdateService) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.status
	if s.engine != nil {
		status.State = s.engine.State()
	}
	return status
}

func (s *UpdateService) Check() (Status, error) {
	if !s.op.TryLock() {
		return s.GetStatus(), errors.New("更新操作正在进行")
	}
	defer s.op.Unlock()
	if !s.status.Enabled {
		return s.GetStatus(), errors.New(s.status.Reason)
	}
	if s.engine.State() == updater.StateReady {
		return s.GetStatus(), nil
	}
	s.mu.Lock()
	s.pending = false
	s.status.Version, s.status.Notes, s.status.Size = "", "", 0
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	release, err := s.engine.Check(ctx)
	if err != nil {
		return s.GetStatus(), err
	}
	if release != nil {
		s.mu.Lock()
		s.status.Version, s.status.Notes, s.status.Size = release.Version, release.Notes, release.Artifact.Size
		s.pending = true
		s.mu.Unlock()
	}
	return s.GetStatus(), nil
}

func (s *UpdateService) Download() error {
	if !s.op.TryLock() {
		return errors.New("更新操作正在进行")
	}
	defer s.op.Unlock()
	if !s.status.Enabled || !s.pending {
		return errors.New("请先检查并确认可用更新")
	}
	if s.engine.State() == updater.StateReady {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
	defer cancel()
	return s.engine.DownloadAndInstall(ctx)
}

func (s *UpdateService) Restart() error {
	if !s.op.TryLock() {
		return errors.New("更新操作正在进行")
	}
	defer s.op.Unlock()
	if !s.status.Enabled || s.engine.State() != updater.StateReady {
		return errors.New("更新尚未准备就绪")
	}
	return s.engine.Restart(s.ctx)
}

func disabledReason(platform, arch, exe, appImage string, debug bool) string {
	if debug {
		return "开发模式不执行自动更新"
	}
	if platform == "darwin" && (arch == "arm64" || arch == "amd64") {
		if !strings.Contains(filepath.ToSlash(exe), ".app/Contents/MacOS/") {
			return "请安装正式的 .app 应用后使用自动更新"
		}
		return ""
	}
	if platform == "windows" && arch == "amd64" {
		return ""
	}
	if platform == "linux" && arch == "amd64" {
		if appImage != "" {
			return "AppImage 请从发布页下载新版替换"
		}
		path := filepath.ToSlash(exe)
		for _, prefix := range []string{"/usr/", "/bin/", "/sbin/", "/snap/", "/nix/"} {
			if strings.HasPrefix(path, prefix) {
				return "系统安装的应用请通过安装包或包管理器更新"
			}
		}
		return ""
	}
	return "当前平台暂无自动更新包，请从发布页下载"
}
