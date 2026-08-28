package lanshare

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestService(t *testing.T, port int) *LanShareService {
	t.Helper()
	svc := NewLanShareService()
	svc.cfgDir = t.TempDir()
	svc.cfg.Port = port
	svc.cfg.ReceiveDir = t.TempDir()
	return svc
}

// reloadServiceAt mimics NewLanShareService against a prepared config dir.
func reloadServiceAt(cfgDir string) *LanShareService {
	svc := &LanShareService{cfgDir: cfgDir, now: time.Now, sess: newSession()}
	svc.cfg = svc.loadConfig()
	return svc
}

func TestDefaultConfig(t *testing.T) {
	svc := newTestService(t, DefaultPort)
	status, err := svc.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Running {
		t.Error("server should not be running before first use")
	}
	if status.Port != DefaultPort {
		t.Errorf("port = %d, want %d", status.Port, DefaultPort)
	}
	if status.ItemCount != 0 || len(status.URLs) != 0 {
		t.Errorf("idle status = %+v", status)
	}
}

func TestEnsureServerStartsAndStops(t *testing.T) {
	svc := newTestService(t, 0) // 0 hits the ephemeral fallback path
	status, err := svc.EnsureServer()
	if err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	if !status.Running {
		t.Fatal("server should be running")
	}
	if status.Token == "" || len(status.URLs) == 0 {
		t.Fatalf("status = %+v, want token and urls", status)
	}
	if status.Port < 1024 {
		t.Errorf("bound port = %d", status.Port)
	}

	// Idempotent: second call keeps the same token and port.
	again, err := svc.EnsureServer()
	if err != nil {
		t.Fatal(err)
	}
	if again.Token != status.Token || again.Port != status.Port {
		t.Errorf("second status = %+v, want stable", again)
	}

	if err := svc.StopServer(); err != nil {
		t.Fatalf("StopServer: %v", err)
	}
	idle, _ := svc.GetStatus()
	if idle.Running || idle.Token != "" || len(idle.URLs) != 0 {
		t.Errorf("idle status after stop = %+v", idle)
	}
}

func TestPortConflictFallsBackToEphemeral(t *testing.T) {
	svcA := newTestService(t, 0)
	statusA, err := svcA.EnsureServer()
	if err != nil {
		t.Fatal(err)
	}
	defer svcA.StopServer()

	// Second service configured on the same busy port must still start.
	svcB := newTestService(t, statusA.Port)
	statusB, err := svcB.EnsureServer()
	if err != nil {
		t.Fatalf("fallback start: %v", err)
	}
	defer svcB.StopServer()
	if statusB.Port == statusA.Port {
		t.Errorf("both servers bound %d", statusA.Port)
	}
}

func TestAddFilesStartsServerAndNotifies(t *testing.T) {
	svc := newTestService(t, 0)
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := svc.AddFiles([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "x.txt" {
		t.Fatalf("items = %+v", items)
	}
	status, _ := svc.GetStatus()
	if !status.Running || status.ItemCount != 1 {
		t.Errorf("status after share = %+v", status)
	}
	if err := svc.RemoveItem(items[0].ID); err != nil {
		t.Fatal(err)
	}
	status, _ = svc.GetStatus()
	if status.ItemCount != 0 {
		t.Errorf("item count after remove = %d", status.ItemCount)
	}
	if err := svc.StopServer(); err != nil {
		t.Fatal(err)
	}
	status, _ = svc.GetStatus()
	if !status.Running {
		return
	}
	t.Error("server should be stopped")
}

func TestAddText(t *testing.T) {
	svc := newTestService(t, 0)
	item, err := svc.AddText("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if item.Kind != ShareItemKindText || item.Text != "hello world" {
		t.Fatalf("item = %+v", item)
	}
	if err := svc.ClearItems(); err != nil {
		t.Fatal(err)
	}
	status, _ := svc.GetStatus()
	if status.ItemCount != 0 {
		t.Errorf("count after clear = %d", status.ItemCount)
	}
	if err := svc.StopServer(); err != nil {
		t.Fatal(err)
	}
}

func TestReceiveDirValidationAndPersistence(t *testing.T) {
	svc := newTestService(t, 0)
	receive, err := svc.GetReceiveDir()
	if err != nil || receive == "" {
		t.Fatalf("receive dir = %q, err %v", receive, err)
	}

	// Relative paths are rejected.
	if err := svc.SetReceiveDir("relative/path"); err == nil {
		t.Error("relative receive dir should be rejected")
	}

	// A valid absolute directory is persisted to disk.
	dir := filepath.Join(t.TempDir(), "接收")
	if err := svc.SetReceiveDir(dir); err != nil {
		t.Fatalf("SetReceiveDir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dir not created: %v", err)
	}

	reloaded := reloadServiceAt(svc.cfgDir)
	if got := reloaded.cfg.ReceiveDir; got != filepath.Clean(dir) {
		t.Errorf("reloaded receive dir = %q, want %q", got, dir)
	}
}

func TestPortSettingPersistenceAndRestart(t *testing.T) {
	svc := newTestService(t, 0)
	if _, err := svc.EnsureServer(); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetPort(80); err == nil {
		t.Error("privileged port should be rejected")
	}
	if err := svc.SetPort(70000); err == nil {
		t.Error("out-of-range port should be rejected")
	}

	if err := svc.SetPort(18432); err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	status, _ := svc.GetStatus()
	if !status.Running {
		t.Fatal("server should survive a port change")
	}

	reloaded := reloadServiceAt(svc.cfgDir)
	if reloaded.cfg.Port != 18432 {
		t.Errorf("reloaded port = %d, want 18432", reloaded.cfg.Port)
	}
	if err := svc.StopServer(); err != nil {
		t.Fatal(err)
	}
}

func TestServiceShutdownStopsServer(t *testing.T) {
	svc := newTestService(t, 0)
	if _, err := svc.EnsureServer(); err != nil {
		t.Fatal(err)
	}
	if err := svc.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown: %v", err)
	}
	status, _ := svc.GetStatus()
	if status.Running {
		t.Error("server should be stopped after ServiceShutdown")
	}
}
