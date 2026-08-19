package filemanager

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type recordingCounter struct {
	total atomic.Int64
}

func (r *recordingCounter) bytesCopied(n int64) {
	r.total.Add(n)
}

func TestCopyFileReportsBytesAndContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	content := make([]byte, 300*1024) // spans multiple copy chunks
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out", "dst.bin")

	counter := &recordingCounter{}
	if err := copyFile(context.Background(), src, dst, newCancellationCleaner(), counter); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}
	if got := counter.total.Load(); got != int64(len(content)) {
		t.Fatalf("bytesCopied total = %d, want %d", got, len(content))
	}
	copied, err := os.ReadFile(dst)
	if err != nil || len(copied) != len(content) {
		t.Fatalf("dst content length = %d, err = %v", len(copied), err)
	}
}

func TestCopyFileCancelledContextRemovesPartialDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(src, make([]byte, 1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst.bin")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before any byte is written

	created := newCancellationCleaner()
	err := copyFile(ctx, src, dst, created, &recordingCounter{})
	if err != context.Canceled {
		t.Fatalf("copyFile() error = %v, want context.Canceled", err)
	}
	created.cleanup()
	if _, statErr := os.Lstat(dst); !os.IsNotExist(statErr) {
		t.Fatalf("partial destination should be removed after cancellation, stat error = %v", statErr)
	}
}

func TestCopyEntriesCancelledReturnsPartialResults(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "first.txt")
	if err := os.WriteFile(first, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(second, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &FileService{}
	results, err := s.CopyEntries(ctx, []string{first, second}, dest, "rename")
	if err != nil {
		t.Fatalf("CopyEntries() first call error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("CopyEntries() returned %d results, want 2", len(results))
	}

	cancel()
	third := filepath.Join(dir, "third.txt")
	if err := os.WriteFile(third, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err = s.CopyEntries(ctx, []string{third}, dest, "rename")
	if err != context.Canceled {
		t.Fatalf("CopyEntries() cancelled error = %v, want context.Canceled", err)
	}
	if len(results) != 0 {
		t.Fatalf("CopyEntries() cancelled returned %d results, want 0", len(results))
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "third.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled copy should not leave destination, stat error = %v", statErr)
	}
}

func TestScanTotalBytes(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(fileA, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	fileB := filepath.Join(sub, "b.txt")
	if err := os.WriteFile(fileB, make([]byte, 250), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := scanTotalBytes([]string{fileA, sub}); got != 350 {
		t.Fatalf("scanTotalBytes() = %d, want 350", got)
	}
	// Missing paths are skipped, not fatal.
	if got := scanTotalBytes([]string{filepath.Join(dir, "missing")}); got != 0 {
		t.Fatalf("scanTotalBytes(missing) = %d, want 0", got)
	}
}

func TestProgressEmitterThrottlesByteUpdates(t *testing.T) {
	emitter := newProgressEmitter(OperationKindCopy, 2, []string{"a", "b"})
	current := time.Now()
	emitter.now = func() time.Time { return current }

	// Advance virtual clock and inspect snapshots via exported state instead
	// of app events: exercise accumulation through bytesCopied.
	emitter.bytesCopied(10)
	emitter.bytesCopied(20)

	emitter.mu.Lock()
	got := emitter.snapshot.DoneBytes
	emitter.mu.Unlock()
	if got != 30 {
		t.Fatalf("DoneBytes = %d, want 30 (throttling must not drop accounting)", got)
	}

	// Within the throttle window no emission happens, so DoneItems changes
	// still emit; verify throttling logic by advancing the clock.
	current = current.Add(2 * emitter.minGap)
	emitter.bytesCopied(40)
	emitter.mu.Lock()
	got = emitter.snapshot.DoneBytes
	emitter.mu.Unlock()
	if got != 70 {
		t.Fatalf("DoneBytes after window = %d, want 70", got)
	}
}

func TestProgressEmitterSetTotalBytesClampsDoneBytes(t *testing.T) {
	emitter := newProgressEmitter(OperationKindCopy, 1, []string{"a"})
	emitter.bytesCopied(500)
	emitter.setTotalBytes(300)
	emitter.mu.Lock()
	done, total := emitter.snapshot.DoneBytes, emitter.snapshot.TotalBytes
	emitter.mu.Unlock()
	if total != 300 {
		t.Fatalf("TotalBytes = %d, want 300", total)
	}
	if done > total {
		t.Fatalf("DoneBytes = %d must not exceed TotalBytes = %d", done, total)
	}
}

func TestDeleteEntriesCancelledStopsEarly(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(first, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &FileService{}
	deleted, err := s.DeleteEntries(ctx, []string{first})
	if err != nil {
		t.Fatalf("DeleteEntries() error = %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("DeleteEntries() deleted %d, want 1", len(deleted))
	}

	cancel()
	deleted, err = s.DeleteEntries(ctx, []string{second})
	if err != context.Canceled {
		t.Fatalf("DeleteEntries() cancelled error = %v, want context.Canceled", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("DeleteEntries() cancelled deleted %d, want 0", len(deleted))
	}
	if _, statErr := os.Lstat(second); statErr != nil {
		t.Fatalf("file should remain after cancelled delete, stat error = %v", statErr)
	}
}
