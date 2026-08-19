package filemanager

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const operationProgressEventName = "filemanager:operation-progress"

type OperationKind string

const (
	OperationKindCopy   OperationKind = "copy"
	OperationKindMove   OperationKind = "move"
	OperationKindDelete OperationKind = "delete"
	OperationKindTrash  OperationKind = "trash"
)

type OperationProgressPhase string

const (
	OperationProgressPhaseScan      OperationProgressPhase = "scan"
	OperationProgressPhaseRun       OperationProgressPhase = "run"
	OperationProgressPhaseDone      OperationProgressPhase = "done"
	OperationProgressPhaseError     OperationProgressPhase = "error"
	OperationProgressPhaseCancelled OperationProgressPhase = "cancelled"
)

// OperationProgress is emitted to the frontend via the Wails event system
// while a batch file operation is running.
type OperationProgress struct {
	OperationID string                 `json:"operationId"`
	Kind        OperationKind          `json:"kind"`
	Phase       OperationProgressPhase `json:"phase"`
	TotalItems  int                    `json:"totalItems"`
	DoneItems   int                    `json:"doneItems"`
	TotalBytes  int64                  `json:"totalBytes"` // -1 when unknown yet
	DoneBytes   int64                  `json:"doneBytes"`
	CurrentName string                 `json:"currentName"`
	Error       string                 `json:"error,omitempty"`
}

func init() {
	application.RegisterEvent[OperationProgress](operationProgressEventName)
}

func newOperationID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}

// progressEmitter tracks one running operation and throttles byte-level
// progress events: high-frequency chunk updates are only emitted when enough
// time has passed, while item completion and phase changes are always emitted.
type progressEmitter struct {
	now        func() time.Time
	minGap     time.Duration
	mu         sync.Mutex
	lastEmitAt time.Time
	snapshot   OperationProgress
}

func newProgressEmitter(kind OperationKind, totalItems int, paths []string) *progressEmitter {
	currentName := ""
	if len(paths) > 0 {
		currentName = filepath.Base(paths[0])
	}
	return &progressEmitter{
		now: time.Now,
		// 100ms is far below human perception latency while capping event
		// traffic at 10 events per second per operation.
		minGap: 100 * time.Millisecond,
		snapshot: OperationProgress{
			OperationID: newOperationID(),
			Kind:        kind,
			Phase:       OperationProgressPhaseScan,
			TotalItems:  totalItems,
			TotalBytes:  -1,
			CurrentName: currentName,
		},
	}
}

// emit publishes the current snapshot. Byte-only updates are throttled.
// The force flag bypasses throttling for phase changes and item completion.
// Emission is skipped entirely when the app instance is unavailable (unit
// tests, service-level usage outside a running app).
func (p *progressEmitter) emit(phase OperationProgressPhase, force bool) {
	p.mu.Lock()
	p.snapshot.Phase = phase
	snapshot := p.snapshot
	now := p.now()
	if !force && now.Sub(p.lastEmitAt) < p.minGap {
		p.mu.Unlock()
		return
	}
	p.lastEmitAt = now
	p.mu.Unlock()

	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(operationProgressEventName, snapshot)
}

func (p *progressEmitter) id() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snapshot.OperationID
}

func (p *progressEmitter) setTotalBytes(total int64) {
	p.mu.Lock()
	p.snapshot.TotalBytes = total
	if p.snapshot.DoneBytes > total {
		p.snapshot.DoneBytes = total
	}
	p.mu.Unlock()
	p.emit(OperationProgressPhaseRun, true)
}

func (p *progressEmitter) setCurrentName(name string) {
	p.mu.Lock()
	if p.snapshot.CurrentName != name {
		p.snapshot.CurrentName = name
		p.mu.Unlock()
		p.emit(OperationProgressPhaseRun, true)
		return
	}
	p.mu.Unlock()
}

func (p *progressEmitter) itemDone() {
	p.mu.Lock()
	p.snapshot.DoneItems++
	p.mu.Unlock()
	p.emit(OperationProgressPhaseRun, true)
}

// bytesCopied accumulates copied bytes and emits a throttled update.
func (p *progressEmitter) bytesCopied(n int64) {
	p.mu.Lock()
	p.snapshot.DoneBytes += n
	p.mu.Unlock()
	p.emit(OperationProgressPhaseRun, false)
}

func (p *progressEmitter) finish(err error, cancelled bool) {
	phase := OperationProgressPhaseDone
	if cancelled {
		phase = OperationProgressPhaseCancelled
	} else if err != nil {
		phase = OperationProgressPhaseError
		p.mu.Lock()
		p.snapshot.Error = err.Error()
		p.mu.Unlock()
	}
	p.emit(phase, true)
}

// scanTotalBytes walks paths in the calling goroutine and reports the sum of
// regular file sizes. Missing paths are skipped so a deleted source does not
// abort the operation.
func scanTotalBytes(paths []string) int64 {
	var total int64
	for _, path := range paths {
		_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				if err == fs.SkipDir || err == fs.SkipAll {
					return err
				}
				// Unreadable entries contribute no size; keep walking siblings.
				return nil
			}
			if d.Type().IsRegular() {
				info, infoErr := d.Info()
				if infoErr == nil {
					total += info.Size()
				}
			}
			return nil
		})
	}
	return total
}

// scanTotalBytesAsync starts a background scan that reports into the emitter
// and cancels with ctx. It returns immediately.
func (p *progressEmitter) scanTotalBytesAsync(ctx context.Context, paths []string) {
	go func() {
		total := scanTotalBytes(paths)
		select {
		case <-ctx.Done():
			return
		default:
		}
		p.setTotalBytes(total)
	}()
}
