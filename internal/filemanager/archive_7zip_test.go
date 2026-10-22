package filemanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mholt/archives"
)

func requireSevenZip(t *testing.T) string {
	t.Helper()
	filename := findSevenZip(context.Background(), sevenZipCandidates())
	if filename == "" {
		t.Skip("integration test requires a real 7zz/7z/7za executable")
	}
	return filename
}

func TestSevenZipDetectionRejectsMissingAndNonExecutable(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "7zz")
	if err := os.WriteFile(invalid, []byte("not an executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findSevenZip(context.Background(), []string{filepath.Join(dir, "missing"), dir, invalid, invalid}); got != "" {
		t.Fatal("accepted invalid 7-Zip candidate")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	formats, err := (&FileService{}).GetArchiveFormats(ctx)
	if !errors.Is(err, context.Canceled) || !slices.Equal(formats, []string{"zip", "tar.gz", "tar"}) {
		t.Fatalf("cancelled discovery = %v, %v", formats, err)
	}
}

func TestSevenZipCapabilities(t *testing.T) {
	formats, err := (&FileService{}).GetArchiveFormats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zip", "tar.gz", "tar"}
	if findSevenZip(context.Background(), sevenZipCandidates()) != "" {
		want = append(want, "7z")
	}
	if !slices.Equal(formats, want) {
		t.Fatalf("formats = %v, want %v", formats, want)
	}
}

func TestSevenZipCreateRoundTripAndNoOverwrite(t *testing.T) {
	requireSevenZip(t)
	dir := t.TempDir()
	// Different source parents exercise archive updates and root-name preservation.
	left, right := filepath.Join(dir, "left"), filepath.Join(dir, "right")
	for _, folder := range []string{filepath.Join(left, "资料", "空目录"), right} {
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	first := filepath.Join(left, "资料")
	second := filepath.Join(right, "@列表 [测试] -文件.txt")
	want := map[string]string{"资料/说明.txt": "目录中的内容", "@列表 [测试] -文件.txt": "独立文件"}
	if err := os.WriteFile(filepath.Join(first, "说明.txt"), []byte(want["资料/说明.txt"]), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte(want["@列表 [测试] -文件.txt"]), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "结果.7z")
	service := &FileService{}
	if _, err := service.CreateArchive(context.Background(), []string{first, second}, output, "7z"); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListArchive(context.Background(), output, "", 0, 200)
	if err != nil || page.Total != 2 {
		t.Fatalf("root = %+v, %v", page, err)
	}
	extracted := filepath.Join(dir, "extracted")
	if _, err := service.ExtractArchive(context.Background(), output, extracted, nil); err != nil {
		t.Fatal(err)
	}
	for name, expected := range want {
		actual, err := os.ReadFile(filepath.Join(extracted, filepath.FromSlash(name)))
		if err != nil || string(actual) != expected {
			t.Fatalf("content for %s: %q, %v", name, actual, err)
		}
	}
	if info, err := os.Stat(filepath.Join(extracted, "资料", "空目录")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory missing: %v", err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateArchive(context.Background(), []string{first}, output, "7z"); err == nil {
		t.Fatal("overwrote existing archive")
	}
	after, err := os.ReadFile(output)
	if err != nil || !slices.Equal(before, after) {
		t.Fatal("existing archive changed")
	}
	assertNoSevenZipWorkdirs(t, dir)
}

func TestSevenZipProcessFailureAndCancellationCleanWorkdir(t *testing.T) {
	executable := requireSevenZip(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(source, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := archives.FilesFromDisk(context.Background(), nil, map[string]string{source: "file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	// The executable disappears after detection; a real process start must fail.
	output := filepath.Join(dir, "failure.7z")
	err = createSevenZip(context.Background(), filepath.Join(dir, "missing-7zz"), output, map[string]string{source: "file.txt"}, files, newProgressEmitter(OperationKindCompress, 1, nil))
	if err == nil {
		t.Fatal("expected process failure")
	}
	assertNoSevenZipWorkdirs(t, dir)
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("left failed output")
	}

	large, err := os.OpenFile(source, os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(128 << 20); err != nil {
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	files, err = archives.FilesFromDisk(context.Background(), nil, map[string]string{source: "file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	// Stop a real native compression job while its output still lives in the workdir.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err = createSevenZip(ctx, executable, output, map[string]string{source: "file.txt"}, files, newProgressEmitter(OperationKindCompress, 1, nil))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline: %v", err)
	}
	assertNoSevenZipWorkdirs(t, dir)
}

func assertNoSevenZipWorkdirs(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".zashiki-7z-") {
			t.Fatal("temporary archive directory was not cleaned")
		}
	}
}

func TestSevenZipProgressAcrossChunks(t *testing.T) {
	p := newProgressEmitter(OperationKindCompress, 1, nil)
	writer := &sevenZipProgressWriter{emitter: p, baseBytes: 100, batchBytes: 1000}
	for _, chunk := range []string{"\r  2", "5%", "\b\b\b50%", "\r 20%", "\r 999%"} {
		if _, err := io.WriteString(writer, chunk); err != nil {
			t.Fatal(err)
		}
	}
	if p.snapshot.DoneBytes != 600 {
		t.Fatalf("progress = %d, want 600", p.snapshot.DoneBytes)
	}
}

func TestSevenZipBatchArgumentLimit(t *testing.T) {
	dir := t.TempDir()
	inputs := make(map[string]string)
	for i := range 200 {
		name := fmt.Sprintf("%03d-%s.txt", i, strings.Repeat("x", 100))
		inputs[filepath.Join(dir, name)] = name
	}
	batches := sevenZipBatches(inputs, nil)
	if len(batches) < 2 {
		t.Fatal("did not split long command line")
	}
	var count int
	for _, batch := range batches {
		var size int
		for _, name := range batch.names {
			size += len(name) + 1
			count++
		}
		if size > 8192 {
			t.Fatalf("argument bytes = %d", size)
		}
	}
	if count != len(inputs) {
		t.Fatal("lost input paths")
	}
}
