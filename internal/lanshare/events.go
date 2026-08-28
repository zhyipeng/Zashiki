package lanshare

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	eventTransferProgress = "lanshare:transfer-progress"
	eventTextReceived     = "lanshare:text-received"
	eventItemsChanged     = "lanshare:items-changed"
	eventServerStatus     = "lanshare:server-status"
)

func init() {
	application.RegisterEvent[TransferProgress](eventTransferProgress)
	application.RegisterEvent[TextReceived](eventTextReceived)
	application.RegisterEvent[ItemsChanged](eventItemsChanged)
	application.RegisterEvent[ServerStatus](eventServerStatus)
}

// emitEvent publishes a typed event, skipping entirely when the app instance
// is unavailable (unit tests, service usage outside a running app).
func emitEvent[T any](name string, payload T) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(name, payload)
}

func newRandomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}

// newShareToken generates the URL token that gates all browser access.
// 10 random bytes hex-encoded: 80 bits of entropy, immune to on-LAN brute force.
func newShareToken() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}

// transferEmitter tracks one running transfer and throttles byte-level
// progress events; completion and errors are always emitted. Emission is
// skipped when the app instance is unavailable (unit tests).
type transferEmitter struct {
	now        func() time.Time
	minGap     time.Duration
	mu         sync.Mutex
	lastEmitAt time.Time
	snapshot   TransferProgress
}

func newTransferEmitter(direction TransferDirection, remoteAddr, name string, total int64) *transferEmitter {
	if total <= 0 {
		total = -1
	}
	return &transferEmitter{
		now: time.Now,
		// Same cap as filemanager operations: 10 events per second per transfer.
		minGap: 100 * time.Millisecond,
		snapshot: TransferProgress{
			TransferID: newRandomID(),
			Direction:  direction,
			RemoteAddr: remoteAddr,
			Name:       name,
			TotalBytes: total,
		},
	}
}

func (e *transferEmitter) emit(phase TransferProgressPhase, force bool, err error) {
	e.mu.Lock()
	e.snapshot.Phase = phase
	if err != nil {
		e.snapshot.Error = err.Error()
	}
	snapshot := e.snapshot
	now := e.now()
	if !force && now.Sub(e.lastEmitAt) < e.minGap {
		e.mu.Unlock()
		return
	}
	e.lastEmitAt = now
	e.mu.Unlock()

	emitEvent(eventTransferProgress, snapshot)
}

// bytes accumulates transferred bytes and emits a throttled update.
func (e *transferEmitter) bytes(n int64) {
	e.mu.Lock()
	e.snapshot.DoneBytes += n
	e.mu.Unlock()
	e.emit(TransferPhaseRun, false, nil)
}

func (e *transferEmitter) finish(err error) {
	phase := TransferPhaseDone
	if err != nil {
		phase = TransferPhaseError
	}
	e.emit(phase, true, err)
}
