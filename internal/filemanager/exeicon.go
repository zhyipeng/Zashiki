package filemanager

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	peResourceDirectoryIndex = 2 // IMAGE_DIRECTORY_ENTRY_RESOURCE
	peResourceTypeIcon       = 3 // RT_ICON
	peResourceTypeGroupIcon  = 14 // RT_GROUP_ICON
	// 单个 exe 图标资源的大小上限，防御畸形 PE 导致超大分配。
	maxExeIconBytes = 8 << 20
)

// ErrExeIconNotFound 表示 PE 中没有可用的图标资源。
var ErrExeIconNotFound = errors.New("exe icon not found")

// ExtractExeIcon 从 Windows PE（.exe）的资源段提取第一个图标组，
// 重组为独立 .ico 文件字节，供 WebView 的 <img> 直接渲染（Chromium 原生支持 ICO）。
// 仅做只读解析（debug/pe），不加载执行文件本体，任意平台可调用。
func ExtractExeIcon(path string) ([]byte, error) {
	pf, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer pf.Close()

	root, ok := peResourceRoot(pf)
	if !ok {
		return nil, ErrExeIconNotFound
	}
	section := sectionCoveringRVA(pf, root)
	if section == nil {
		return nil, ErrExeIconNotFound
	}
	// 目录条目里的偏移相对资源目录起始，数据条目里的 OffsetToData 才是绝对 RVA。
	r := &rvaReader{section: section, resourceRoot: root}

	// 图标组：RT_GROUP_ICON → 组 ID → 语言 → GRPICONDIR 数据。
	typeEntry, err := r.lookupEntry(root, peResourceTypeGroupIcon, false)
	if err != nil || !typeEntry.isDir {
		return nil, ErrExeIconNotFound
	}
	groupEntry, err := r.lookupEntry(typeEntry.rva, 0, true)
	if err != nil || !groupEntry.isDir {
		return nil, ErrExeIconNotFound
	}
	langEntry, err := r.lookupEntry(groupEntry.rva, 0, true)
	if err != nil || langEntry.isDir {
		return nil, ErrExeIconNotFound
	}
	group, err := r.resourceData(langEntry.rva)
	if err != nil {
		return nil, err
	}

	return buildIco(r, root, group)
}

// peResourceRoot 返回资源目录的 RVA；无资源段时返回 false。
func peResourceRoot(pf *pe.File) (uint32, bool) {
	switch oh := pf.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		dd := oh.DataDirectory[peResourceDirectoryIndex]
		return dd.VirtualAddress, dd.VirtualAddress != 0 && dd.Size != 0
	case *pe.OptionalHeader64:
		dd := oh.DataDirectory[peResourceDirectoryIndex]
		return dd.VirtualAddress, dd.VirtualAddress != 0 && dd.Size != 0
	default:
		return 0, false
	}
}

func sectionCoveringRVA(pf *pe.File, rva uint32) *pe.Section {
	for _, s := range pf.Sections {
		size := s.VirtualSize
		if size == 0 {
			size = s.Size
		}
		if rva >= s.VirtualAddress && rva-s.VirtualAddress < size {
			return s
		}
	}
	return nil
}

// rvaReader 在 PE 某个段内按 RVA 定点读取，避免把整段资源载入内存。
type rvaReader struct {
	section *pe.Section
	// resourceRoot 是资源目录的绝对 RVA；目录条目里的偏移都相对它。
	resourceRoot uint32
}

func (r *rvaReader) read(rva uint32, size int) ([]byte, error) {
	if size <= 0 || size > maxExeIconBytes {
		return nil, fmt.Errorf("invalid resource read size %d", size)
	}
	off := int64(rva) - int64(r.section.VirtualAddress)
	if off < 0 || off+int64(size) > int64(r.section.Size) {
		return nil, fmt.Errorf("resource rva 0x%x size %d out of section range (va 0x%x raw %d)", rva, size, r.section.VirtualAddress, r.section.Size)
	}
	buf := make([]byte, size)
	if _, err := r.section.ReadAt(buf, off); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

type resourceEntry struct {
	rva   uint32
	isDir bool
}

// lookupEntry 在资源目录中查找条目：wantID 精确匹配 ID 条目；
// any 为 true 时取第一个条目（用于语言层）。高位 0x80000000 表示子目录，
// 条目偏移相对资源目录起始，这里换算回绝对 RVA。
func (r *rvaReader) lookupEntry(dirRVA, wantID uint32, any bool) (resourceEntry, error) {
	header, err := r.read(dirRVA, 16)
	if err != nil {
		return resourceEntry{}, err
	}
	named := uint32(binary.LittleEndian.Uint16(header[12:]))
	count := uint32(binary.LittleEndian.Uint16(header[14:]))
	if named > 4096 || count > 4096 {
		return resourceEntry{}, errors.New("resource directory entry count out of range")
	}
	base := dirRVA + 16 + named*8
	for i := uint32(0); i < count; i++ {
		raw, err := r.read(base+i*8, 8)
		if err != nil {
			return resourceEntry{}, err
		}
		nameOrID := binary.LittleEndian.Uint32(raw[0:4])
		offset := binary.LittleEndian.Uint32(raw[4:8])
		isNamed := nameOrID&0x80000000 != 0
		if !any && (isNamed || nameOrID != wantID) {
			continue
		}
		return resourceEntry{rva: r.resourceRoot + (offset &^ 0x80000000), isDir: offset&0x80000000 != 0}, nil
	}
	return resourceEntry{}, ErrExeIconNotFound
}

// resourceData 读取叶子条目（IMAGE_RESOURCE_DATA_ENTRY）指向的数据。
func (r *rvaReader) resourceData(entryRVA uint32) ([]byte, error) {
	raw, err := r.read(entryRVA, 16)
	if err != nil {
		return nil, err
	}
	dataRVA := binary.LittleEndian.Uint32(raw[0:4])
	size := binary.LittleEndian.Uint32(raw[4:8])
	if size == 0 || size > maxExeIconBytes {
		return nil, errors.New("resource data size out of range")
	}
	return r.read(dataRVA, int(size))
}

// iconData 按资源 ID 取 RT_ICON 图像数据。
func (r *rvaReader) iconData(rootRVA uint32, id uint16) ([]byte, error) {
	typeEntry, err := r.lookupEntry(rootRVA, peResourceTypeIcon, false)
	if err != nil || !typeEntry.isDir {
		return nil, ErrExeIconNotFound
	}
	idEntry, err := r.lookupEntry(typeEntry.rva, uint32(id), false)
	if err != nil || !idEntry.isDir {
		return nil, ErrExeIconNotFound
	}
	langEntry, err := r.lookupEntry(idEntry.rva, 0, true)
	if err != nil || langEntry.isDir {
		return nil, ErrExeIconNotFound
	}
	return r.resourceData(langEntry.rva)
}

// buildIco 把 GRPICONDIR 与引用的 RT_ICON 数据重组为标准 .ico 文件：
// ICONDIR + ICONDIRENTRY 列表 + 图像数据，条目里的资源 ID 换成文件内偏移。
func buildIco(r *rvaReader, rootRVA uint32, group []byte) ([]byte, error) {
	if len(group) < 6 {
		return nil, ErrExeIconNotFound
	}
	count := int(binary.LittleEndian.Uint16(group[4:6]))
	if count == 0 || count > 64 || len(group) < 6+count*14 {
		return nil, ErrExeIconNotFound
	}

	type iconImage struct {
		entry [16]byte // ICONDIRENTRY
		data  []byte
	}
	images := make([]iconImage, 0, count)
	for i := 0; i < count; i++ {
		grpEntry := group[6+14*i : 6+14*i+14]
		id := binary.LittleEndian.Uint16(grpEntry[12:14])
		data, err := r.iconData(rootRVA, id)
		if err != nil {
			return nil, err
		}
		var entry [16]byte
		copy(entry[0:8], grpEntry[0:8]) // width/height/colors/reserved/planes/bitCount
		// 以实际数据长度为准，bytesInRes 用文件内偏移占位，随后回填。
		binary.LittleEndian.PutUint32(entry[8:12], uint32(len(data)))
		images = append(images, iconImage{entry: entry, data: data})
	}

	var buf bytes.Buffer
	var dir [6]byte
	binary.LittleEndian.PutUint16(dir[0:2], 0)
	binary.LittleEndian.PutUint16(dir[2:4], 1)
	binary.LittleEndian.PutUint16(dir[4:6], uint16(count))
	buf.Write(dir[:])

	offset := uint32(6 + 16*count)
	for i := range images {
		binary.LittleEndian.PutUint32(images[i].entry[12:16], offset)
		buf.Write(images[i].entry[:])
		offset += uint32(len(images[i].data))
	}
	for i := range images {
		buf.Write(images[i].data)
	}
	return buf.Bytes(), nil
}
