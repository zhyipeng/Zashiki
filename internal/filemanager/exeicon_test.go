package filemanager

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// buildTestIconPNG 生成一张 1x1 PNG 作为 RT_ICON 资源内容（Vista+ 图标允许 PNG 压缩）。
func buildTestIconPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func resourceDirHeader(named, ids uint16) []byte {
	h := make([]byte, 16)
	binary.LittleEndian.PutUint16(h[12:], named)
	binary.LittleEndian.PutUint16(h[14:], ids)
	return h
}

// buildResourceBlob 组装包含 RT_ICON + RT_GROUP_ICON 的资源段内容，
// 返回 blob 与按 ExtractExeIcon 规则应重组出的 .ico 字节。
func buildResourceBlob(t *testing.T, iconPNG []byte) ([]byte, []byte) {
	t.Helper()

	var buf bytes.Buffer
	rva := func(off int) uint32 { return 0x1000 + uint32(off) }
	write := func(p []byte) int {
		off := buf.Len()
		buf.Write(p)
		for buf.Len()%4 != 0 {
			buf.WriteByte(0)
		}
		return off
	}
	// 写入目录头并预留 ids*8 字节的条目空间，返回目录与首条目的偏移。
	writeDir := func(ids uint16) (dirOff, entryOff int) {
		dirOff = write(resourceDirHeader(0, ids))
		entryOff = dirOff + 16
		buf.Write(make([]byte, int(ids)*8))
		for buf.Len()%4 != 0 {
			buf.WriteByte(0)
		}
		return dirOff, entryOff
	}

	_, rootEntryOff := writeDir(2)
	rootIconEntryOff := rootEntryOff
	rootGroupEntryOff := rootEntryOff + 8

	iconTypeDirOff, iconTypeEntryOff := writeDir(1)
	iconLangDirOff, iconLangEntryOff := writeDir(1)
	iconDataOff := write(make([]byte, 16))
	pngOff := write(iconPNG)

	groupTypeDirOff, groupTypeEntryOff := writeDir(1)
	groupIDDirOff, groupIDEntryOff := writeDir(1)
	groupDataOff := write(make([]byte, 16))

	grpDir := make([]byte, 20)
	binary.LittleEndian.PutUint16(grpDir[2:4], 1) // type = icon
	binary.LittleEndian.PutUint16(grpDir[4:6], 1) // count
	// GRPICONDIRENTRY: width(1) height(1) colors(1) reserved(1) planes(2) bitCount(2) bytesInRes(4) id(2)
	grpDir[6] = 1
	grpDir[7] = 1
	binary.LittleEndian.PutUint16(grpDir[10:12], 1)  // planes
	binary.LittleEndian.PutUint16(grpDir[12:14], 32) // bitCount
	binary.LittleEndian.PutUint32(grpDir[14:18], uint32(len(iconPNG)))
	binary.LittleEndian.PutUint16(grpDir[18:20], 1) // 引用的 RT_ICON 资源 ID
	grpDirOff := write(grpDir)

	blob := buf.Bytes()
	putDirEntry := func(at int, id uint32, targetRVA uint32, isDir bool) {
		var e [8]byte
		binary.LittleEndian.PutUint32(e[0:4], id)
		if isDir {
			targetRVA |= 0x80000000
		}
		binary.LittleEndian.PutUint32(e[4:8], targetRVA)
		copy(blob[at:at+8], e[:])
	}
	putLE32 := func(at int, v uint32) {
		binary.LittleEndian.PutUint32(blob[at:at+4], v)
	}

	// 目录条目按真实 PE 格式写相对资源目录起始的偏移；数据条目内的
	// OffsetToData 才是绝对 RVA。
	putDirEntry(rootIconEntryOff, peResourceTypeIcon, uint32(iconTypeDirOff), true)
	putDirEntry(rootGroupEntryOff, peResourceTypeGroupIcon, uint32(groupTypeDirOff), true)
	putDirEntry(iconTypeEntryOff, 1, uint32(iconLangDirOff), true)
	putDirEntry(iconLangEntryOff, 0x409, uint32(iconDataOff), false)
	putLE32(iconDataOff, rva(pngOff))
	putLE32(iconDataOff+4, uint32(len(iconPNG)))
	putDirEntry(groupTypeEntryOff, 1, uint32(groupIDDirOff), true)
	putDirEntry(groupIDEntryOff, 0x409, uint32(groupDataOff), false)
	putLE32(groupDataOff, rva(grpDirOff))
	putLE32(groupDataOff+4, uint32(len(grpDir)))

	// 期望的 .ico：ICONDIR + 单条 ICONDIRENTRY + PNG 数据。
	var expected bytes.Buffer
	var dir [6]byte
	binary.LittleEndian.PutUint16(dir[2:4], 1) // type = icon
	binary.LittleEndian.PutUint16(dir[4:6], 1) // count
	expected.Write(dir[:])
	var entry [16]byte
	entry[0], entry[1] = 1, 1 // width/height
	binary.LittleEndian.PutUint16(entry[4:6], 1)
	binary.LittleEndian.PutUint16(entry[6:8], 32)
	binary.LittleEndian.PutUint32(entry[8:12], uint32(len(iconPNG)))
	binary.LittleEndian.PutUint32(entry[12:16], 6+16)
	expected.Write(entry[:])
	expected.Write(iconPNG)

	return blob, expected.Bytes()
}

// buildTestPE 手工组装一个 debug/pe 可解析的最小 PE64，.rsrc 位于 RVA 0x1000。
func buildTestPE(t *testing.T, resourceBlob []byte) []byte {
	t.Helper()

	var b bytes.Buffer
	w16 := func(v uint16) { var x [2]byte; binary.LittleEndian.PutUint16(x[:], v); b.Write(x[:]) }
	w32 := func(v uint32) { var x [4]byte; binary.LittleEndian.PutUint32(x[:], v); b.Write(x[:]) }
	w64 := func(v uint64) { var x [8]byte; binary.LittleEndian.PutUint64(x[:], v); b.Write(x[:]) }

	b.WriteString("MZ")
	b.Write(make([]byte, 58)) // DOS 头余部（0x02..0x3B）
	w32(0x80)                 // e_lfanew
	b.Write(make([]byte, 64)) // 0x40..0x7F

	b.WriteString("PE\x00\x00")
	// COFF header
	w16(0x8664) // amd64
	w16(1)      // section count
	w32(0)
	w32(0)
	w32(0)
	w16(240)    // sizeOfOptionalHeader（PE32+ 固定 112 + 16×8 数据目录）
	w16(0x0022) // executable image | large address aware
	// Optional header PE32+
	w16(0x020B)
	b.Write([]byte{1, 0}) // linker version
	w32(0)
	w32(0)
	w32(0)
	w32(0) // entry point
	w32(0x1000)
	w64(0x140000000)
	w32(0x1000) // section alignment
	w32(0x200)  // file alignment
	w16(6)
	w16(0)
	w16(0)
	w16(0)
	w16(6)
	w16(0)
	w32(0)
	w32(0x2000) // size of image
	w32(0x200)  // size of headers
	w32(0)
	w16(2) // subsystem GUI
	w16(0)
	w64(0x100000)
	w64(0x1000)
	w64(0x100000)
	w64(0x1000)
	w32(0)
	w32(16) // numberOfRvaAndSizes
	resourceRVA := uint32(0)
	resourceSize := uint32(0)
	if resourceBlob != nil {
		resourceRVA = 0x1000
		resourceSize = uint32(len(resourceBlob))
	}
	for i := 0; i < 16; i++ {
		if i == peResourceDirectoryIndex {
			w32(resourceRVA)
			w32(resourceSize)
		} else {
			w32(0)
			w32(0)
		}
	}
	// Section header
	b.WriteString(".rsrc\x00\x00\x00")
	rawSize := (len(resourceBlob) + 0x1FF) / 0x200 * 0x200
	if rawSize == 0 {
		rawSize = 0x200
	}
	w32(uint32(max(len(resourceBlob), 1))) // virtual size
	w32(0x1000)                            // virtual address
	w32(uint32(rawSize))                   // size of raw data
	w32(0x400)                             // pointer to raw data
	w32(0)
	w32(0)
	w16(0)
	w16(0)
	w32(0x40000040) // initialized data | read
	for b.Len() < 0x400 {
		b.WriteByte(0)
	}
	if resourceBlob != nil {
		b.Write(resourceBlob)
	}
	for b.Len() < 0x400+rawSize {
		b.WriteByte(0)
	}
	return b.Bytes()
}

func writeTestExe(t *testing.T, peBytes []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.exe")
	if err := os.WriteFile(path, peBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractExeIcon(t *testing.T) {
	iconPNG := buildTestIconPNG(t)
	blob, expectedIco := buildResourceBlob(t, iconPNG)
	path := writeTestExe(t, buildTestPE(t, blob))

	got, err := ExtractExeIcon(path)
	if err != nil {
		t.Fatalf("ExtractExeIcon() error = %v", err)
	}
	if !bytes.Equal(got, expectedIco) {
		t.Fatalf("ExtractExeIcon() = %d bytes, want reassembled ico (%d bytes)\ngot: %v\nwant: %v",
			len(got), len(expectedIco), got[:min(len(got), 32)], expectedIco[:min(len(expectedIco), 32)])
	}
}

func TestExtractExeIconWithoutResources(t *testing.T) {
	path := writeTestExe(t, buildTestPE(t, nil))

	if _, err := ExtractExeIcon(path); !errors.Is(err, ErrExeIconNotFound) {
		t.Fatalf("ExtractExeIcon() error = %v, want ErrExeIconNotFound", err)
	}
}

func TestExtractExeIconRejectsNonPE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fake.exe")
	if err := os.WriteFile(path, []byte("not a pe file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractExeIcon(path); err == nil {
		t.Fatal("ExtractExeIcon() expected error for non-PE content")
	}
}

func TestGenerateThumbnail_ExeIcon(t *testing.T) {
	iconPNG := buildTestIconPNG(t)
	blob, expectedIco := buildResourceBlob(t, iconPNG)
	path := writeTestExe(t, buildTestPE(t, blob))

	result, err := GenerateThumbnail(path, 32)
	if err != nil {
		t.Fatalf("GenerateThumbnail() error = %v", err)
	}
	if result.MimeType != "image/x-icon" {
		t.Fatalf("MimeType = %q, want image/x-icon", result.MimeType)
	}
	if !bytes.Equal(result.Data, expectedIco) {
		t.Fatalf("Data mismatch: got %d bytes, want %d bytes", len(result.Data), len(expectedIco))
	}

	cached, err := GenerateThumbnail(path, 32)
	if err != nil {
		t.Fatalf("GenerateThumbnail() cached error = %v", err)
	}
	if !bytes.Equal(cached.Data, expectedIco) {
		t.Fatal("cached result should be identical")
	}
}

func TestGenerateThumbnail_ExeWithoutIconIsPlainError(t *testing.T) {
	path := writeTestExe(t, buildTestPE(t, nil))

	_, err := GenerateThumbnail(path, 256)
	if err == nil {
		t.Fatal("GenerateThumbnail() expected error for exe without icon")
	}
	// 关键：不能是 ErrThumbnailUnsupported，否则中间件会把 exe 原始字节
	// 当图片回退返回给 <img>。
	if errors.Is(err, ErrThumbnailUnsupported) {
		t.Fatal("exe without icon must not trigger raw-bytes fallback")
	}
}
