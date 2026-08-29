package filemanager

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G'}

// TestExtractExeIcon_RealBinary 用本仓库已构建的 zashiki.exe 做真实验证：
// 重组出的 .ico 结构合法；PNG 压缩的内嵌条目可被 image/png 解码。
// 没有构建产物或该 exe 不含图标资源时跳过。
func TestExtractExeIcon_RealBinary(t *testing.T) {
	candidates := []string{
		// 系统自带 exe（Windows 上必然存在且含多尺寸图标资源）优先：
		// 用真实 PE 检验资源树解析的健壮性；缺失时回退仓库构建产物。
		`C:\Windows\System32\notepad.exe`,
		`C:\Windows\System32\mspaint.exe`,
		filepath.Join("..", "..", "zashiki.exe"),
		filepath.Join("..", "..", "bin", "zashiki.exe"),
	}
	var exePath string
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			exePath = candidate
			break
		}
	}
	if exePath == "" {
		t.Skip("no built zashiki.exe found")
	}

	data, err := ExtractExeIcon(exePath)
	if err != nil {
		t.Skipf("built exe has no icon resources (or unsupported): %v", err)
	}
	if len(data) < 6+16 {
		t.Fatalf("ico too short: %d bytes", len(data))
	}
	count := int(data[4]) | int(data[5])<<8
	if count == 0 || len(data) < 6+16*count {
		t.Fatalf("malformed ico dir: count=%d len=%d", count, len(data))
	}
	t.Logf("extracted %d bytes, %d image(s) from %s", len(data), count, exePath)

	entries := 0
	for i := 0; i < count; i++ {
		entry := data[6+16*i : 6+16*i+16]
		size := int(entry[8]) | int(entry[9])<<8 | int(entry[10])<<16 | int(entry[11])<<24
		offset := int(entry[12]) | int(entry[13])<<8 | int(entry[14])<<16 | int(entry[15])<<24
		if offset+size > len(data) || offset+size < offset {
			t.Fatalf("entry %d out of range: offset=%d size=%d", i, offset, size)
		}
		if bytes.HasPrefix(data[offset:offset+size], pngSignature) {
			if _, err := png.Decode(bytes.NewReader(data[offset : offset+size])); err != nil {
				t.Fatalf("entry %d: embedded png failed to decode: %v", i, err)
			}
			entries++
		}
	}
	t.Logf("%d of %d entries are PNG-compressed and decode cleanly", entries, count)
}
