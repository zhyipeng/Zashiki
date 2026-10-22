package filemanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mholt/archives"
)

func sevenZipCandidates() []string {
	names := []string{"7zz", "7z", "7za"}
	var candidates []string
	for _, name := range names {
		if filename, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, filename)
		}
	}
	var directories []string
	switch runtime.GOOS {
	case "darwin":
		directories = []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin", "/usr/bin"}
	case "linux":
		directories = []string{"/usr/local/bin", "/usr/bin", "/bin"}
	case "windows":
		for _, key := range []string{"ProgramW6432", "ProgramFiles", "ProgramFiles(x86)"} {
			if dir := os.Getenv(key); filepath.IsAbs(dir) {
				directories = append(directories, filepath.Join(dir, "7-Zip"))
			}
		}
		if dir := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(dir) {
			directories = append(directories, filepath.Join(dir, "Programs", "7-Zip"))
		}
		names = []string{"7zz.exe", "7z.exe", "7za.exe"}
	}
	for _, directory := range directories {
		for _, name := range names {
			candidates = append(candidates, filepath.Join(directory, name))
		}
	}
	return candidates
}

// Limit captured diagnostics even if an unrelated executable has the same name.
type sevenZipProbeOutput struct{ bytes.Buffer }

func (b *sevenZipProbeOutput) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 64*1024 - b.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}

func findSevenZip(ctx context.Context, candidates []string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return ""
		}
		filename, err := filepath.Abs(candidate)
		if err != nil || seen[filename] {
			continue
		}
		seen[filename] = true
		info, err := os.Stat(filename)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		probeCtx, stop := context.WithTimeout(ctx, time.Second)
		cmd := exec.CommandContext(probeCtx, filename, "i")
		configureArchiveCommand(cmd)
		cmd.WaitDelay = 250 * time.Millisecond
		var output sevenZipProbeOutput
		cmd.Stdout = &output
		cmd.Stderr = io.Discard
		err = cmd.Run()
		stop()
		if err == nil && (bytes.Contains(output.Bytes(), []byte("7-Zip")) || bytes.Contains(output.Bytes(), []byte("p7zip"))) {
			return filename
		}
	}
	return ""
}

func (f *FileService) GetArchiveFormats(ctx context.Context) ([]string, error) {
	formats := []string{"zip", "tar.gz", "tar"}
	if findSevenZip(ctx, sevenZipCandidates()) != "" {
		formats = append(formats, "7z")
	}
	return formats, ctx.Err()
}

type sevenZipBatch struct {
	directory string
	names     []string
	bytes     int64
	items     int
}

// Batch relative literal arguments to retain root names from different directories
// and stay below Windows' command-line limit without list-file name escaping.
func sevenZipBatches(inputs map[string]string, files []archives.FileInfo) []sevenZipBatch {
	weights := make(map[string]sevenZipBatch)
	for _, file := range files {
		root, _, _ := strings.Cut(file.NameInArchive, "/")
		weight := weights[root]
		if !file.IsDir() {
			weight.bytes += file.Size()
			weight.items++
		}
		weights[root] = weight
	}
	paths := make([]string, 0, len(inputs))
	for filename := range inputs {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	var batches []sevenZipBatch
	var argumentBytes int
	for _, filename := range paths {
		directory, name := filepath.Dir(filename), "."+string(filepath.Separator)+filepath.Base(filename)
		if len(batches) == 0 || batches[len(batches)-1].directory != directory || argumentBytes+len(name)+1 > 8192 {
			batches = append(batches, sevenZipBatch{directory: directory})
			argumentBytes = 0
		}
		batch := &batches[len(batches)-1]
		weight := weights[inputs[filename]]
		batch.names = append(batch.names, name)
		batch.bytes += weight.bytes
		batch.items += weight.items
		argumentBytes += len(name) + 1
	}
	return batches
}

var sevenZipPercent = regexp.MustCompile(`(?:^|[\r\n\x08])\s*(\d{1,3})%`)

type sevenZipProgressWriter struct {
	emitter    *progressEmitter
	baseBytes  int64
	batchBytes int64
	tail       string
}

func (w *sevenZipProgressWriter) Write(data []byte) (int, error) {
	text := w.tail + string(data)
	for _, match := range sevenZipPercent.FindAllStringSubmatch(text, -1) {
		percent, _ := strconv.ParseInt(match[1], 10, 64)
		if percent > 100 {
			continue
		}
		done := w.baseBytes + w.batchBytes*percent/100
		w.emitter.mu.Lock()
		if done > w.emitter.snapshot.DoneBytes {
			w.emitter.snapshot.DoneBytes = done
		}
		w.emitter.mu.Unlock()
		w.emitter.emit(OperationProgressPhaseRun, false)
	}
	if len(text) > 32 {
		text = text[len(text)-32:]
	}
	w.tail = text
	return len(data), nil
}

func createSevenZip(ctx context.Context, executable, destination string, inputs map[string]string, files []archives.FileInfo, p *progressEmitter) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("目标压缩包已存在")
	} else if !os.IsNotExist(err) {
		return err
	}
	// 7-Zip expects a missing archive, not a pre-created empty file.
	work, err := os.MkdirTemp(filepath.Dir(destination), ".zashiki-7z-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	archive := filepath.Join(work, "archive.7z")
	var completedBytes int64
	for _, batch := range sevenZipBatches(inputs, files) {
		p.setCurrentName(strings.TrimPrefix(batch.names[0], "."+string(filepath.Separator)))
		args := []string{"a", "-t7z", "-bsp1", "-bso0", "-bse0", "-bb0", "-spd", "-snl", "-w" + work, "-y", archive, "--"}
		args = append(args, batch.names...)
		cmd := exec.CommandContext(ctx, executable, args...)
		configureArchiveCommand(cmd)
		cmd.Dir = batch.directory
		cmd.WaitDelay = time.Second
		cmd.Stdout = &sevenZipProgressWriter{emitter: p, baseBytes: completedBytes, batchBytes: batch.bytes}
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("7-Zip 压缩失败：%w", err)
		}
		completedBytes += batch.bytes
		p.mu.Lock()
		p.snapshot.DoneBytes = completedBytes
		p.snapshot.DoneItems += batch.items
		p.mu.Unlock()
		p.emit(OperationProgressPhaseRun, true)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Same-volume hard links publish atomically and cannot replace an existing file.
	if err := os.Link(archive, destination); err == nil {
		return nil
	} else if os.IsExist(err) {
		return err
	}
	// FAT and some network filesystems do not support hard links.
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(destination)
		}
	}()
	input, err := os.Open(archive)
	if err != nil {
		_ = output.Close()
		return err
	}
	_, copyErr := io.Copy(output, &archiveContentReader{ctx: ctx, reader: input})
	inputErr, outputErr := input.Close(), output.Close()
	if copyErr != nil {
		return copyErr
	}
	if inputErr != nil {
		return inputErr
	}
	if outputErr != nil {
		return outputErr
	}
	return ctx.Err()
}
