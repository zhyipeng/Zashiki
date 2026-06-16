package filemanager

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	maxTextPreviewBytes   = 256 * 1024
	maxImagePreviewBytes  = 10 * 1024 * 1024
	maxOfficePreviewBytes = 100 * 1024 * 1024
	maxPdfPreviewBytes    = 50 * 1024 * 1024
)

type previewContext struct {
	path     string
	name     string
	ext      string
	mimeType string
	size     int64
	version  string
}

type previewProvider interface {
	Match(previewContext) bool
	Build(previewContext) (FilePreview, error)
}

var filePreviewProviders = []previewProvider{
	imagePreviewProvider{},
	officePreviewProvider{},
	pdfPreviewProvider{},
	textPreviewProvider{},
	unsupportedPreviewProvider{},
}

func (f *FileService) GetFilePreview(path string) (FilePreview, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FilePreview{}, err
	}

	ctx := previewContext{
		path:     path,
		name:     info.Name(),
		ext:      strings.ToLower(filepath.Ext(info.Name())),
		mimeType: previewMimeType(info.Name()),
		size:     info.Size(),
		version:  previewVersion(info),
	}

	if info.IsDir() {
		preview := baseFilePreview(ctx, "unsupported")
		preview.Message = "目录暂不支持预览"
		return preview, nil
	}

	for _, provider := range filePreviewProviders {
		if provider.Match(ctx) {
			return provider.Build(ctx)
		}
	}

	return unsupportedPreviewProvider{}.Build(ctx)
}

func (f *FileService) SaveTextPreview(path string, content string, expectedVersion string) (FilePreview, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FilePreview{}, err
	}
	if info.IsDir() {
		return FilePreview{}, fmt.Errorf("cannot edit directory %q", path)
	}
	if expectedVersion != "" && previewVersion(info) != expectedVersion {
		return FilePreview{}, fmt.Errorf("file changed since preview was opened")
	}
	if err := os.WriteFile(path, []byte(content), info.Mode().Perm()); err != nil {
		return FilePreview{}, err
	}
	return f.GetFilePreview(path)
}

type imagePreviewProvider struct{}

func (imagePreviewProvider) Match(ctx previewContext) bool {
	return strings.HasPrefix(ctx.mimeType, "image/") || previewImageMIMETypes[ctx.ext] != ""
}

func (imagePreviewProvider) Build(ctx previewContext) (FilePreview, error) {
	if ctx.size > maxImagePreviewBytes {
		preview := baseFilePreview(ctx, "unsupported")
		preview.Message = "图片超过 10 MB，暂不生成预览"
		return preview, nil
	}

	data, err := os.ReadFile(ctx.path)
	if err != nil {
		return FilePreview{}, err
	}

	mimeType := ctx.mimeType
	if mimeType == "" {
		mimeType = previewImageMIMETypes[ctx.ext]
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	preview := baseFilePreview(ctx, "image")
	preview.MimeType = mimeType
	preview.DataURL = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return preview, nil
}

type officePreviewProvider struct{}

func (officePreviewProvider) Match(ctx previewContext) bool {
	return previewOfficeMIMETypes[ctx.ext] != ""
}

func (officePreviewProvider) Build(ctx previewContext) (FilePreview, error) {
	if ctx.size > maxOfficePreviewBytes {
		preview := baseFilePreview(ctx, "unsupported")
		preview.Message = "Office 文件超过 100 MB，暂不生成预览"
		return preview, nil
	}

	data, err := os.ReadFile(ctx.path)
	if err != nil {
		return FilePreview{}, err
	}

	mimeType := previewOfficeMIMETypes[ctx.ext]

	preview := baseFilePreview(ctx, "office")
	preview.MimeType = mimeType
	preview.DataURL = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return preview, nil
}

type pdfPreviewProvider struct{}

func (pdfPreviewProvider) Match(ctx previewContext) bool {
	return previewPdfMIMETypes[ctx.ext] != ""
}

func (pdfPreviewProvider) Build(ctx previewContext) (FilePreview, error) {
	if ctx.size > maxPdfPreviewBytes {
		preview := baseFilePreview(ctx, "unsupported")
		preview.Message = "PDF 文件超过 50 MB，暂不生成预览"
		return preview, nil
	}

	data, err := os.ReadFile(ctx.path)
	if err != nil {
		return FilePreview{}, err
	}

	mimeType := previewPdfMIMETypes[ctx.ext]

	preview := baseFilePreview(ctx, "pdf")
	preview.MimeType = mimeType
	preview.DataURL = "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return preview, nil
}

type textPreviewProvider struct{}

func (textPreviewProvider) Match(ctx previewContext) bool {
	return true
}

func (textPreviewProvider) Build(ctx previewContext) (FilePreview, error) {
	data, err := readPreviewPrefix(ctx.path, maxTextPreviewBytes+utf8.UTFMax)
	if err != nil {
		return FilePreview{}, err
	}

	content, ok := validTextPreviewContent(data, maxTextPreviewBytes)
	if !ok {
		preview := baseFilePreview(ctx, "unsupported")
		preview.Message = "此文件不是可识别的文本"
		return preview, nil
	}

	mimeType := ctx.mimeType
	if mimeType == "" {
		mimeType = previewTextMIMETypes[ctx.ext]
	}
	if mimeType == "" {
		mimeType = "text/plain"
	}

	preview := baseFilePreview(ctx, "text")
	preview.MimeType = mimeType
	preview.Content = string(content)
	preview.Truncated = ctx.size > int64(len(content))
	return preview, nil
}

type unsupportedPreviewProvider struct{}

func (unsupportedPreviewProvider) Match(previewContext) bool {
	return true
}

func (unsupportedPreviewProvider) Build(ctx previewContext) (FilePreview, error) {
	preview := baseFilePreview(ctx, "unsupported")
	preview.Message = "暂不支持此文件类型预览"
	return preview, nil
}

func baseFilePreview(ctx previewContext, kind string) FilePreview {
	return FilePreview{
		Name:     ctx.name,
		Path:     ctx.path,
		Kind:     kind,
		MimeType: ctx.mimeType,
		Size:     ctx.size,
		Version:  ctx.version,
	}
}

func previewVersion(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())
}

func readPreviewPrefix(path string, limit int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(io.LimitReader(file, int64(limit)))
}

func validTextPreviewContent(data []byte, maxBytes int) ([]byte, bool) {
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, false
	}

	end := 0
	for end < len(data) && end < maxBytes {
		r, size := utf8.DecodeRune(data[end:])
		if r == utf8.RuneError && size == 1 {
			return nil, false
		}
		if end+size > maxBytes {
			break
		}
		end += size
	}

	return data[:end], true
}

func previewMimeType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if mimeType := previewImageMIMETypes[ext]; mimeType != "" {
		return mimeType
	}
	if mimeType := previewOfficeMIMETypes[ext]; mimeType != "" {
		return mimeType
	}
	if mimeType := previewPdfMIMETypes[ext]; mimeType != "" {
		return mimeType
	}
	if mimeType := previewTextMIMETypes[ext]; mimeType != "" {
		return mimeType
	}

	mimeType := mime.TypeByExtension(ext)
	if index := strings.IndexByte(mimeType, ';'); index >= 0 {
		mimeType = mimeType[:index]
	}
	return mimeType
}

var previewImageMIMETypes = map[string]string{
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
}

var previewOfficeMIMETypes = map[string]string{
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

var previewPdfMIMETypes = map[string]string{
	".pdf": "application/pdf",
}

var previewTextMIMETypes = map[string]string{
	".cfg":  "text/plain",
	".conf": "text/plain",
	".css":  "text/css",
	".csv":  "text/csv",
	".go":   "text/plain",
	".html": "text/html",
	".ini":  "text/plain",
	".js":   "text/javascript",
	".json": "application/json",
	".log":  "text/plain",
	".md":   "text/markdown",
	".py":   "text/x-python",
	".rs":   "text/plain",
	".sh":   "text/x-shellscript",
	".toml": "text/plain",
	".ts":   "text/typescript",
	".txt":  "text/plain",
	".vue":  "text/plain",
	".xml":  "application/xml",
	".yaml": "application/yaml",
	".yml":  "application/yaml",
}
