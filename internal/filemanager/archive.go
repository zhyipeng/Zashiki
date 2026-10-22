package filemanager

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mholt/archives"
)

const (
	maxArchiveEntries            = 100000
	maxArchiveBytes        int64 = 100 << 30
	maxArchiveCacheEntries       = 4
)

// ArchiveEntry.Path is relative to the archive, never a disk path.
type ArchiveEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	IsDir  bool   `json:"isDir"`
	IsLink bool   `json:"isLink"`
}

type ArchivePage struct {
	Entries []ArchiveEntry `json:"entries"`
	Total   int            `json:"total"`
}

type archiveIndex struct {
	version     string
	directories map[string][]ArchiveEntry
}

// archiveSource retains random access for ZIP/7z while making input reads cancellable.
type archiveSource struct {
	*os.File
	ctx context.Context
}

func (r archiveSource) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.File.Read(p)
}

func (r archiveSource) ReadAt(p []byte, off int64) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.File.ReadAt(p, off)
}

func walkArchive(ctx context.Context, filename string, handle archives.FileHandler) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	format, reader, err := archives.Identify(ctx, filepath.Base(filename), archiveSource{f, ctx})
	if err != nil {
		return fmt.Errorf("无法识别压缩包：%w", err)
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("此格式不支持压缩包浏览或解压")
	}
	return extractor.Extract(ctx, reader, handle)
}

// Reject both POSIX and Windows escape paths regardless of the host platform.
func archiveEntryPath(name string) (string, error) {
	if len(name) > 4096 || strings.Count(name, "/")+strings.Count(name, "\\") > 128 {
		return "", fmt.Errorf("压缩包内路径过长或目录层级过深")
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(name, "/") || strings.ContainsAny(name, ":\x00") {
		return "", fmt.Errorf("压缩包包含不安全路径")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return "", fmt.Errorf("压缩包包含不安全路径")
		}
	}
	clean := path.Clean(name)
	if clean == "." {
		return "", nil
	}
	if !fs.ValidPath(clean) {
		return "", fmt.Errorf("压缩包包含无效路径")
	}
	return clean, nil
}

func archiveEntry(f archives.FileInfo) (ArchiveEntry, error) {
	name, err := archiveEntryPath(f.NameInArchive)
	if err != nil {
		return ArchiveEntry{}, err
	}
	return ArchiveEntry{Name: path.Base(name), Path: name, Size: f.Size(), IsDir: f.IsDir(), IsLink: f.LinkTarget != "" || f.Mode()&os.ModeSymlink != 0}, nil
}

func (f *FileService) loadArchiveIndex(ctx context.Context, filename string) (*archiveIndex, error) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("请选择压缩文件")
	}
	version := previewVersion(info)
	f.mu.Lock()
	cached := f.archiveCache[filename]
	f.mu.Unlock()
	if cached != nil && cached.version == version {
		return cached, ctx.Err()
	}
	entries := make(map[string]ArchiveEntry)
	var count int
	var size int64
	err = walkArchive(ctx, filename, func(ctx context.Context, file archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > maxArchiveEntries || file.Size() < 0 || file.Size() > maxArchiveBytes-size {
			return fmt.Errorf("压缩包超过限制（10 万条目 / 100 GiB）")
		}
		size += file.Size()
		entry, err := archiveEntry(file)
		if err != nil {
			return err
		}
		if entry.Path == "" {
			return nil
		}
		if old, exists := entries[entry.Path]; exists && !(old.IsDir && entry.IsDir) {
			return fmt.Errorf("压缩包包含重复或冲突路径：%s", entry.Path)
		}
		entries[entry.Path] = entry
		for parent := path.Dir(entry.Path); parent != "."; parent = path.Dir(parent) {
			if old, exists := entries[parent]; exists {
				if !old.IsDir || old.IsLink {
					return fmt.Errorf("压缩包目录与文件冲突：%s", parent)
				}
			} else {
				entries[parent] = ArchiveEntry{Name: path.Base(parent), Path: parent, IsDir: true}
				if len(entries) > maxArchiveEntries {
					return fmt.Errorf("压缩包目录条目过多")
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	current, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if previewVersion(current) != version || !os.SameFile(info, current) {
		return nil, fmt.Errorf("压缩包已变化，请重新打开")
	}
	index := &archiveIndex{version: version, directories: map[string][]ArchiveEntry{"": {}}}
	for _, entry := range entries {
		parent := path.Dir(entry.Path)
		if parent == "." {
			parent = ""
		}
		index.directories[parent] = append(index.directories[parent], entry)
		if entry.IsDir && index.directories[entry.Path] == nil {
			index.directories[entry.Path] = []ArchiveEntry{}
		}
	}
	for _, children := range index.directories {
		sort.Slice(children, func(i, j int) bool {
			if children[i].IsDir != children[j].IsDir {
				return children[i].IsDir
			}
			return children[i].Name < children[j].Name
		})
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.archiveCache == nil || len(f.archiveCache) >= maxArchiveCacheEntries {
		f.archiveCache = make(map[string]*archiveIndex)
	}
	f.archiveCache[filename] = index
	return index, nil
}

func (f *FileService) ListArchive(ctx context.Context, filename, directory string, offset, limit int) (ArchivePage, error) {
	directory, err := archiveEntryPath(directory)
	if err != nil {
		return ArchivePage{}, err
	}
	index, err := f.loadArchiveIndex(ctx, filename)
	if err != nil {
		return ArchivePage{}, err
	}
	entries, ok := index.directories[directory]
	if !ok {
		return ArchivePage{}, fmt.Errorf("压缩包内目录不存在")
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	return ArchivePage{Entries: append([]ArchiveEntry{}, entries[offset:end]...), Total: len(entries)}, nil
}

// Preview only text and images in memory; archive paths never enter disk preview APIs.
func (f *FileService) GetArchiveEntryPreview(ctx context.Context, filename, entryPath string) (FilePreview, error) {
	entryPath, err := archiveEntryPath(entryPath)
	if err != nil {
		return FilePreview{}, err
	}
	if _, err := f.loadArchiveIndex(ctx, filename); err != nil {
		return FilePreview{}, err
	}
	preview := FilePreview{Name: path.Base(entryPath), Path: entryPath, Kind: "unsupported", Message: "请解压后预览此文件类型"}
	found := false
	err = walkArchive(ctx, filename, func(ctx context.Context, file archives.FileInfo) error {
		entry, err := archiveEntry(file)
		if err != nil {
			return err
		}
		if entry.Path != entryPath {
			return nil
		}
		found = true
		preview.Size = entry.Size
		if entry.IsDir || entry.IsLink || !file.Mode().IsRegular() {
			return fs.SkipAll
		}
		ext := strings.ToLower(path.Ext(entryPath))
		imageMIME := previewImageMIMETypes[ext]
		if previewPdfMIMETypes[ext] != "" || previewOfficeMIMETypes[ext] != "" || previewAudioMIMETypes[ext] != "" || previewVideoMIMETypes[ext] != "" {
			return fs.SkipAll
		}
		limit := int64(maxTextPreviewBytes + utf8.UTFMax)
		if imageMIME != "" {
			if file.Size() > maxImagePreviewBytes {
				preview.Message = "图片超过 10 MB，请解压后预览"
				return fs.SkipAll
			}
			limit = maxImagePreviewBytes + 1
		}
		r, err := file.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(&archiveContentReader{ctx: ctx, reader: r}, limit))
		closeErr := r.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if imageMIME != "" {
			if len(data) > maxImagePreviewBytes {
				return fmt.Errorf("图片超过预览大小限制")
			}
			preview.Kind, preview.MimeType = "image", imageMIME
			preview.DataURL = "data:" + imageMIME + ";base64," + base64.StdEncoding.EncodeToString(data)
		} else if content, ok := validTextPreviewContent(data, maxTextPreviewBytes); ok {
			preview.Kind, preview.MimeType = "text", previewMimeType(entryPath)
			preview.Content = string(content)
			preview.Truncated = file.Size() > int64(len(content))
		}
		return fs.SkipAll
	})
	if err != nil {
		return FilePreview{}, err
	}
	if !found {
		return FilePreview{}, fmt.Errorf("压缩包内文件不存在")
	}
	return preview, nil
}

type archiveContentReader struct {
	ctx      context.Context
	reader   io.Reader
	progress *progressEmitter
}

func (r *archiveContentReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if r.progress != nil {
		r.progress.bytesCopied(int64(n))
	}
	return n, err
}

func selectedArchiveEntry(name string, selected []string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, prefix := range selected {
		if name == prefix || strings.HasPrefix(name, prefix+"/") {
			return true
		}
	}
	return false
}

// ExtractArchive creates a new destination; existing files are never overwritten.
// os.Root provides filesystem-level confinement even if a path is concurrently replaced.
func (f *FileService) ExtractArchive(ctx context.Context, filename, destination string, selected []string) (result string, err error) {
	p := newProgressEmitter(OperationKindExtract, 0, []string{filename})
	p.emit(OperationProgressPhaseScan, true)
	defer func() { p.finish(err, errors.Is(err, context.Canceled)) }()
	for i, name := range selected {
		selected[i], err = archiveEntryPath(name)
		if err != nil || selected[i] == "" {
			return "", fmt.Errorf("请选择有效的压缩包内条目")
		}
	}
	index, err := f.loadArchiveIndex(ctx, filename)
	if err != nil {
		return "", err
	}
	known := make(map[string]ArchiveEntry)
	var total int64
	var totalItems int
	for _, entries := range index.directories {
		for _, entry := range entries {
			known[entry.Path] = entry
			if selectedArchiveEntry(entry.Path, selected) && !entry.IsDir {
				total += entry.Size
				totalItems++
			}
		}
	}
	for _, name := range selected {
		if _, ok := known[name]; !ok {
			return "", fmt.Errorf("压缩包内条目不存在")
		}
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(destination) == "" {
		return "", fmt.Errorf("请输入新建的解压目录")
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if err = os.Mkdir(destination, 0o755); err != nil {
		return "", fmt.Errorf("解压目录必须尚不存在：%w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destination)
		}
	}()
	root, err := os.OpenRoot(destination)
	if err != nil {
		return "", err
	}
	defer root.Close()
	p.mu.Lock()
	p.snapshot.TotalItems = totalItems
	p.mu.Unlock()
	p.setTotalBytes(total)
	var written int64
	count := 0
	err = walkArchive(ctx, filename, func(ctx context.Context, file archives.FileInfo) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > maxArchiveEntries {
			return fmt.Errorf("压缩包条目过多")
		}
		entry, err := archiveEntry(file)
		if err != nil {
			return err
		}
		if entry.Path == "" || !selectedArchiveEntry(entry.Path, selected) {
			return nil
		}
		if entry.IsLink || (!entry.IsDir && !file.Mode().IsRegular()) {
			return fmt.Errorf("暂不解压链接或特殊文件：%s", entry.Path)
		}
		p.setCurrentName(entry.Name)
		local := filepath.FromSlash(entry.Path)
		if entry.IsDir {
			return root.MkdirAll(local, 0o755)
		}
		if err := root.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			return err
		}
		output, err := root.OpenFile(local, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			_ = output.Close()
			return err
		}
		n, copyErr := io.Copy(output, io.LimitReader(&archiveContentReader{ctx: ctx, reader: input, progress: p}, maxArchiveBytes-written+1))
		inputErr, outputErr := input.Close(), output.Close()
		written += n
		if written > maxArchiveBytes {
			return fmt.Errorf("解压内容超过 100 GiB 限制")
		}
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		if outputErr != nil {
			return outputErr
		}
		if n != file.Size() {
			return fmt.Errorf("解压内容与声明大小不一致")
		}
		if err := root.Chmod(local, 0o644|file.Mode().Perm()&0o111); err != nil {
			return err
		}
		if !file.ModTime().IsZero() {
			if err := root.Chtimes(local, file.ModTime(), file.ModTime()); err != nil {
				return err
			}
		}
		p.itemDone()
		return nil
	})
	if err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	return destination, nil
}

type archiveInputFile struct {
	fs.File
	ctx      context.Context
	progress *progressEmitter
}

func (f archiveInputFile) Close() error {
	err := f.File.Close()
	if err == nil {
		f.progress.itemDone()
	}
	return err
}

func (f archiveInputFile) Read(p []byte) (int, error) {
	return (&archiveContentReader{ctx: f.ctx, reader: f.File, progress: f.progress}).Read(p)
}

// Some archive writers close their codec in a defer without returning its error.
// Retain output errors, including those from the final header/trailer flush.
type archiveOutputWriter struct {
	writer io.Writer
	err    error
}

func (w *archiveOutputWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n < len(p) && err == nil {
		err = io.ErrShortWrite
	}
	if w.err == nil {
		w.err = err
	}
	return n, err
}

func (f *FileService) CreateArchive(ctx context.Context, sources []string, destination, format string) (result string, err error) {
	p := newProgressEmitter(OperationKindCompress, len(sources), sources)
	p.emit(OperationProgressPhaseScan, true)
	defer func() { p.finish(err, errors.Is(err, context.Canceled)) }()
	var engine archives.Archiver
	var sevenZipExecutable string
	switch format {
	case "zip":
		engine = archives.Zip{Compression: zip.Deflate, SelectiveCompression: true}
	case "tar":
		engine = archives.Tar{}
	case "tar.gz":
		engine = archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}}
	case "7z":
		sevenZipExecutable = findSevenZip(ctx, sevenZipCandidates())
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if sevenZipExecutable == "" {
			return "", fmt.Errorf("未找到可用的 7-Zip，请安装 7zz、7z 或 7za 后重试")
		}
	default:
		return "", fmt.Errorf("不支持创建此压缩格式")
	}
	if len(sources) == 0 {
		return "", fmt.Errorf("请选择需要压缩的文件")
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	// Resolve the parent so aliases cannot place the output inside a selected directory.
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return "", err
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	inputs := make(map[string]string)
	names := make(map[string]bool)
	for _, source := range sources {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		source, err = filepath.Abs(source)
		if err != nil {
			return "", err
		}
		info, statErr := os.Lstat(source)
		if statErr != nil {
			return "", statErr
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return "", fmt.Errorf("暂不压缩链接或特殊文件")
		}
		resolved, resolveErr := filepath.EvalSymlinks(source)
		if resolveErr != nil {
			return "", resolveErr
		}
		rel, relErr := filepath.Rel(resolved, destination)
		if relErr == nil && (rel == "." || info.IsDir() && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return "", fmt.Errorf("压缩文件不能放在所选源目录内部")
		}
		name := filepath.Base(source)
		if names[strings.ToLower(name)] {
			return "", fmt.Errorf("所选文件存在同名项，请分别压缩")
		}
		if _, pathErr := archiveEntryPath(name); pathErr != nil {
			return "", pathErr
		}
		names[strings.ToLower(name)] = true
		inputs[source] = name
	}
	files, err := archives.FilesFromDisk(ctx, nil, inputs)
	if err != nil {
		return "", err
	}
	if len(files) > maxArchiveEntries {
		return "", fmt.Errorf("需要压缩的条目过多")
	}
	var total int64
	var totalItems int
	for i := range files {
		file := &files[i]
		if _, err = archiveEntryPath(file.NameInArchive); err != nil {
			return "", err
		}
		if file.LinkTarget != "" || (!file.IsDir() && !file.Mode().IsRegular()) {
			return "", fmt.Errorf("暂不压缩链接或特殊文件：%s", file.NameInArchive)
		}
		if !file.IsDir() {
			if file.Size() < 0 || file.Size() > maxArchiveBytes-total {
				return "", fmt.Errorf("压缩内容超过 100 GiB 限制")
			}
			total += file.Size()
			totalItems++
		}
		if format == "7z" {
			continue
		}
		open, name := file.Open, file.NameInArchive
		file.Open = func() (fs.File, error) {
			p.setCurrentName(name)
			input, err := open()
			if err != nil {
				return nil, err
			}
			return archiveInputFile{File: input, ctx: ctx, progress: p}, nil
		}
	}
	p.mu.Lock()
	p.snapshot.TotalItems = totalItems
	p.mu.Unlock()
	p.setTotalBytes(total)
	if format == "7z" {
		if err = createSevenZip(ctx, sevenZipExecutable, destination, inputs, files, p); err != nil {
			return "", err
		}
		return destination, nil
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(destination)
		}
	}()
	writer := &archiveOutputWriter{writer: output}
	err = engine.Archive(ctx, writer, files)
	closeErr := output.Close()
	if err != nil {
		return "", err
	}
	if writer.err != nil {
		return "", writer.err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	return destination, nil
}
