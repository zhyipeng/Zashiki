package filemanager

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// 缩略图默认/夹取范围：与前端 112px 网格单元在 2x DPI 下匹配。
const (
	DefaultThumbnailSize   = 256
	minThumbnailSize       = 32
	maxThumbnailSize       = 512
	thumbnailCacheBudget   = 64 << 20 // 生成结果的总字节预算，超出按 LRU 淘汰
	thumbnailCacheMaxItems = 1024
	jpegEncodeQuality      = 85
)

// ErrThumbnailUnsupported 表示该格式无法在 Go 侧光栅化（svg/avif/heic 等），
// 调用方可回退为原样返回文件内容交由 WebView 渲染。
var ErrThumbnailUnsupported = errors.New("thumbnail unsupported")

// ThumbnailResult 是一张已编码的缩略图。生成失败但格式可原样展示时，
// MimeType 仍会填充（Data 为空），供回退路径使用。
type ThumbnailResult struct {
	Data     []byte
	MimeType string
}

// decodableImageExts 是 Go 侧可解码的图片扩展名；其余已知图片扩展名走回退。
var decodableImageExts = map[string]bool{
	".bmp":  true,
	".gif":  true,
	".jpeg": true,
	".jpg":  true,
	".png":  true,
	".tif":  true,
	".tiff": true,
	".webp": true,
}

// alphaImageExts 的缩略图带透明通道，输出 PNG；其余输出 JPEG。
var alphaImageExts = map[string]bool{
	".gif":  true,
	".png":  true,
	".webp": true,
}

// imageMimeType 返回扩展名对应的图片 MIME；非图片返回空串。
func imageMimeType(ext string) string {
	ext = strings.ToLower(ext)
	if mimeType := previewImageMIMETypes[ext]; mimeType != "" {
		return mimeType
	}
	if m := mime.TypeByExtension(ext); strings.HasPrefix(m, "image/") {
		return m
	}
	return ""
}

// NormalizeThumbnailSize 把请求的缩略图边长夹取到支持范围（默认 256，范围 32–512）。
func NormalizeThumbnailSize(size int) int {
	if size <= 0 {
		return DefaultThumbnailSize
	}
	if size < minThumbnailSize {
		return minThumbnailSize
	}
	if size > maxThumbnailSize {
		return maxThumbnailSize
	}
	return size
}

// GenerateThumbnail 生成 path 的 fit-in-box 缩略图（最长边 maxSize，保持宽高比）。
// 原图尺寸不超过 maxSize 时直接返回原文件字节，避免无谓的重编码。
// .exe 走内嵌图标提取（返回重组的 .ico 字节）。
// 结果按 path+mtime+size+maxSize 做 LRU 缓存。
func GenerateThumbnail(path string, maxSize int) (ThumbnailResult, error) {
	maxSize = NormalizeThumbnailSize(maxSize)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".exe" {
		return generateExeIconThumbnail(path, maxSize)
	}
	mimeType := imageMimeType(ext)
	if mimeType == "" {
		return ThumbnailResult{}, ErrThumbnailUnsupported
	}
	if !decodableImageExts[ext] {
		return ThumbnailResult{MimeType: mimeType}, ErrThumbnailUnsupported
	}

	info, err := os.Stat(path)
	if err != nil {
		return ThumbnailResult{}, err
	}
	if info.IsDir() {
		return ThumbnailResult{}, fmt.Errorf("cannot thumbnail directory %q", path)
	}

	mtimeNs := info.ModTime().UnixNano()
	key := filepath.Clean(path) + "\x00" + strconv.Itoa(maxSize)
	if cached, ok := thumbnailCache.get(key, mtimeNs, info.Size()); ok {
		return cached, nil
	}

	result, err := generateThumbnailUncached(path, ext, mimeType, maxSize)
	if err != nil {
		return ThumbnailResult{}, err
	}
	thumbnailCache.put(key, mtimeNs, info.Size(), result)
	return result, nil
}

// generateExeIconThumbnail 提取 exe 内嵌图标，返回重组的 .ico 字节
// （WebView 的 <img> 原生支持 ICO，按需自行选取尺寸）。提取失败返回普通
// 错误而非 ErrThumbnailUnsupported——.exe 原始字节绝不能作为图片回退。
func generateExeIconThumbnail(path string, maxSize int) (ThumbnailResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ThumbnailResult{}, err
	}
	if info.IsDir() {
		return ThumbnailResult{}, fmt.Errorf("cannot thumbnail directory %q", path)
	}

	mtimeNs := info.ModTime().UnixNano()
	key := filepath.Clean(path) + "\x00" + strconv.Itoa(maxSize)
	if cached, ok := thumbnailCache.get(key, mtimeNs, info.Size()); ok {
		return cached, nil
	}

	data, err := ExtractExeIcon(path)
	if err != nil {
		return ThumbnailResult{}, err
	}
	result := ThumbnailResult{Data: data, MimeType: "image/x-icon"}
	thumbnailCache.put(key, mtimeNs, info.Size(), result)
	return result, nil
}

func generateThumbnailUncached(path, ext, mimeType string, maxSize int) (ThumbnailResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return ThumbnailResult{}, err
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return ThumbnailResult{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	if cfg.Width <= maxSize && cfg.Height <= maxSize {
		data, err := os.ReadFile(path)
		if err != nil {
			return ThumbnailResult{}, err
		}
		return ThumbnailResult{Data: data, MimeType: mimeType}, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ThumbnailResult{}, err
	}

	src, _, err := image.Decode(file)
	if err != nil {
		return ThumbnailResult{}, fmt.Errorf("decode %q: %w", path, err)
	}

	dstRect := fitInBox(src.Bounds(), maxSize)
	dst := image.NewRGBA(dstRect)
	draw.CatmullRom.Scale(dst, dstRect, src, src.Bounds(), draw.Src, nil)

	var buf bytes.Buffer
	result := ThumbnailResult{}
	if alphaImageExts[ext] {
		result.MimeType = "image/png"
		err = png.Encode(&buf, dst)
	} else {
		result.MimeType = "image/jpeg"
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegEncodeQuality})
	}
	if err != nil {
		return ThumbnailResult{}, err
	}
	result.Data = buf.Bytes()
	return result, nil
}

// fitInBox 计算把 bounds 等比缩入 0,0 起点且最长边为 maxSize 的目标矩形。
func fitInBox(bounds image.Rectangle, maxSize int) image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return image.Rect(0, 0, 1, 1)
	}
	scale := 1.0
	if width > height {
		scale = float64(maxSize) / float64(width)
	} else {
		scale = float64(maxSize) / float64(height)
	}
	targetW := max(1, int(float64(width)*scale+0.5))
	targetH := max(1, int(float64(height)*scale+0.5))
	return image.Rect(0, 0, targetW, targetH)
}

// thumbnailCacheEntry 缓存一张已编码缩略图，附生成时的源文件指纹用于新鲜度校验。
type thumbnailCacheEntry struct {
	result   ThumbnailResult
	mtimeNs  int64
	fileSize int64
}

// thumbnailLRU 是按字节预算淘汰的进程内缓存。生成耗时（解码+缩放）远大于
// 一次 os.Stat，因此命中时先校验 mtime+size，源文件变化自动失效。
type thumbnailLRU struct {
	mu      sync.Mutex
	entries map[string]thumbnailCacheEntry
	order   []string // LRU 序，最近使用在末尾
	bytes   int
}

var thumbnailCache = &thumbnailLRU{entries: make(map[string]thumbnailCacheEntry)}

func (c *thumbnailLRU) get(key string, mtimeNs, fileSize int64) (ThumbnailResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || entry.mtimeNs != mtimeNs || entry.fileSize != fileSize {
		return ThumbnailResult{}, false
	}
	c.touchLocked(key)
	return entry.result, true
}

func (c *thumbnailLRU) put(key string, mtimeNs, fileSize int64, result ThumbnailResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.entries[key]; ok {
		c.bytes -= len(existing.result.Data)
		c.touchLocked(key)
	} else {
		c.order = append(c.order, key)
	}
	c.entries[key] = thumbnailCacheEntry{result: result, mtimeNs: mtimeNs, fileSize: fileSize}
	c.bytes += len(result.Data)

	for c.bytes > thumbnailCacheBudget || len(c.order) > thumbnailCacheMaxItems {
		evict := c.order[0]
		c.order = c.order[1:]
		c.bytes -= len(c.entries[evict].result.Data)
		delete(c.entries, evict)
	}
}

func (c *thumbnailLRU) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}
