package lanshare

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adrg/xdg"
)

// DefaultPort is the TCP port the share server binds first; it falls back to
// an ephemeral port when busy.
const DefaultPort = 53100

// LanShareService is the Wails-facing API of the LAN share feature: a lazy
// HTTP workbench reachable from any browser on the local network.
type LanShareService struct {
	mu     sync.Mutex
	sess   *session
	server *shareServer
	token  string
	cfgDir string // overrides xdg.ConfigHome in tests
	cfg    lanShareConfig
	now    func() time.Time
}

// lanShareConfig is persisted in the zashiki config dir, mirroring the
// settings service layout, so uploads survive app restarts.
type lanShareConfig struct {
	Port       int    `json:"port"`
	ReceiveDir string `json:"receiveDir"`
}

func NewLanShareService() *LanShareService {
	s := &LanShareService{
		sess: newSession(),
		now:  time.Now,
	}
	s.cfg = s.loadConfig()
	return s
}

// EnsureServer starts the share server if it is not running and returns the
// current status including candidate URLs.
func (s *LanShareService) EnsureServer() (ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureServerLocked(); err != nil {
		return ServerStatus{}, err
	}
	return s.statusLocked(), nil
}

// GetStatus reports the server state without starting it.
func (s *LanShareService) GetStatus() (ServerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked(), nil
}

// AddFiles shares files or folders (folders are expanded recursively) and
// lazily starts the server.
func (s *LanShareService) AddFiles(paths []string) ([]ShareItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureServerLocked(); err != nil {
		return nil, err
	}
	added, err := s.sess.addFiles(paths, s.now())
	if err != nil {
		return added, err
	}
	s.notifyLocked()
	return added, nil
}

// AddText shares a text snippet.
func (s *LanShareService) AddText(text string) (ShareItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureServerLocked(); err != nil {
		return ShareItem{}, err
	}
	item := s.sess.addText(text, s.now())
	s.notifyLocked()
	return item, nil
}

// RemoveItem drops one shared item by ID; unknown IDs are ignored.
func (s *LanShareService) RemoveItem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess.remove(id)
	s.notifyLocked()
	return nil
}

// ClearItems removes every shared item without stopping the server.
func (s *LanShareService) ClearItems() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sess.clear()
	s.notifyLocked()
	return nil
}

// StopServer shuts the share server down and clears the share list.
func (s *LanShareService) StopServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		if err := s.server.stop(); err != nil {
			return err
		}
		s.server = nil
		s.token = ""
	}
	s.sess.clear()
	emitEvent(eventServerStatus, s.statusLocked())
	return nil
}

// GetReceiveDir returns the directory browser uploads are saved into.
func (s *LanShareService) GetReceiveDir() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.ReceiveDir, nil
}

// SetReceiveDir validates and persists the upload target directory.
func (s *LanShareService) SetReceiveDir(dir string) error {
	if filepath.Clean(dir) == "" || !filepath.IsAbs(dir) {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.ReceiveDir = filepath.Clean(dir)
	return s.saveConfigLocked()
}

// GetPort returns the preferred port; the actual bound port may differ when
// it was busy (see ServerStatus.Port).
func (s *LanShareService) GetPort() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Port, nil
}

// SetPort persists the preferred port and restarts a running server on it
// (falling back to ephemeral when busy). Shared items survive the restart.
func (s *LanShareService) SetPort(port int) error {
	if port < 1024 || port > 65535 {
		return os.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Port = port
	if err := s.saveConfigLocked(); err != nil {
		return err
	}
	if s.server != nil {
		if err := s.server.stop(); err != nil {
			return err
		}
		if err := s.startServerLocked(); err != nil {
			return err
		}
	}
	emitEvent(eventServerStatus, s.statusLocked())
	return nil
}

// ServiceShutdown stops the share server when the application exits.
func (s *LanShareService) ServiceShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		if err := s.server.stop(); err != nil {
			return err
		}
		s.server = nil
		s.token = ""
	}
	return nil
}

func (s *LanShareService) ensureServerLocked() error {
	if s.server != nil {
		return nil
	}
	return s.startServerLocked()
}

func (s *LanShareService) startServerLocked() error {
	token := newShareToken()
	srv := &shareServer{
		sess:    s.sess,
		token:   token,
		onText:  s.onTextReceived,
		receive: s.receiveDir,
	}
	if err := srv.start(s.cfg.Port); err != nil {
		return err
	}
	s.server = srv
	s.token = token
	emitEvent(eventServerStatus, s.statusLocked())
	return nil
}

func (s *LanShareService) statusLocked() ServerStatus {
	status := ServerStatus{
		Port: s.cfg.Port,
		URLs: []string{},
	}
	if s.server != nil {
		status.Running = true
		status.Port = s.server.port
		status.Token = s.token
		status.URLs = lanURLs(s.server.port, s.token)
	}
	status.ItemCount = len(s.sess.list())
	return status
}

func (s *LanShareService) notifyLocked() {
	emitEvent(eventItemsChanged, ItemsChanged{Items: s.sess.list()})
	emitEvent(eventServerStatus, s.statusLocked())
}

func (s *LanShareService) onTextReceived(text TextReceived) {
	emitEvent(eventTextReceived, text)
}

func (s *LanShareService) receiveDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.ReceiveDir
}

func (s *LanShareService) baseDir() string {
	if s.cfgDir != "" {
		return s.cfgDir
	}
	return xdg.ConfigHome
}

func (s *LanShareService) configPath() (string, error) {
	dir := filepath.Join(s.baseDir(), "zashiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "lanshare.json"), nil
}

func (s *LanShareService) loadConfig() lanShareConfig {
	cfg := lanShareConfig{Port: DefaultPort, ReceiveDir: defaultReceiveDir()}
	path, err := s.configPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var stored lanShareConfig
	if json.Unmarshal(data, &stored) == nil {
		if stored.Port >= 1024 && stored.Port <= 65535 {
			cfg.Port = stored.Port
		}
		if filepath.IsAbs(stored.ReceiveDir) {
			cfg.ReceiveDir = filepath.Clean(stored.ReceiveDir)
		}
	}
	return cfg
}

func (s *LanShareService) saveConfigLocked() error {
	path, err := s.configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// defaultReceiveDir prefers the user's real Downloads folder and always adds
// a Zashiki subdirectory so uploads never mix with manually downloaded files.
func defaultReceiveDir() string {
	base := ""
	if xdg.UserDirs.Download != "" {
		base = xdg.UserDirs.Download
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		base = filepath.Join(home, "Downloads")
	}
	if base == "" {
		return filepath.Join(os.TempDir(), "Zashiki")
	}
	return filepath.Join(base, "Zashiki")
}
