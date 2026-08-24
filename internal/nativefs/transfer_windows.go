//go:build windows

package nativefs

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// windowsTransfer 基于经典剪贴板 API（OpenClipboard / SetClipboardData /
// GetClipboardData）写入与读取 CF_HDROP。
//
// 为什么不用 OLE IDataObject：Windows 剪贴板的底层是共享的。无论是
// OleSetClipboard(IDataObject) 还是经典的 SetClipboardData(CF_HDROP)，
// Explorer 粘贴时读取的都是同一份 CF_HDROP 数据（DragQueryFile）。经典
// API 不需要 OLE 初始化、不需要 STA 线程、不需要手写 COM vtable，是大量
// 文件管理器（Total Commander、WinRAR 等）采用的稳妥路径。第 4 步原生
// 拖出（SHDoDragDrop）需要 IDataObject 时再引入 go-ole 实现。
type windowsTransfer struct{}

func newPlatformTransfer() FileTransfer {
	return &windowsTransfer{}
}

// Copy 把 paths 以复制语义（DROPEFFECT_COPY）写入系统剪贴板。
func (t *windowsTransfer) Copy(paths []string) error {
	return setHDROPClipboard(paths, ClipboardCopy)
}

// Cut 把 paths 以移动语义（DROPEFFECT_MOVE）写入系统剪贴板。
func (t *windowsTransfer) Cut(paths []string) error {
	return setHDROPClipboard(paths, ClipboardMove)
}

// ClipboardFiles 从系统剪贴板读取 CF_HDROP 文件引用与 Preferred DropEffect。
func (t *windowsTransfer) ClipboardFiles() (ClipboardContent, error) {
	return readHDROPClipboard()
}

// CurrentSequence 返回系统剪贴板变更序号（GetClipboardSequenceNumber）。
func (t *windowsTransfer) CurrentSequence() (uint64, error) {
	return getClipboardSequenceNumber()
}

// ClearClipboard 清空系统剪贴板（EmptyClipboard）。
func (t *windowsTransfer) ClearClipboard() error {
	return withClipboardOpen(func() error {
		return callEmptyClipboard()
	})
}

// ---- 剪贴板写入 ----

const (
	gmemMoveable = 0x0002
	cfHDROP      = 15

	// 注册名与 Shell 约定的 Preferred DropEffect 剪贴板格式一致。
	cfstrPreferredDropEffect = "Preferred DropEffect"

	dropEffectCopy uint32 = 1 // DROPEFFECT_COPY
	dropEffectMove uint32 = 2 // DROPEFFECT_MOVE
)

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	kernel  = syscall.NewLazyDLL("kernel32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormat    = user32.NewProc("RegisterClipboardFormatW")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")

	procGlobalAlloc  = kernel.NewProc("GlobalAlloc")
	procGlobalLock   = kernel.NewProc("GlobalLock")
	procGlobalUnlock = kernel.NewProc("GlobalUnlock")
	procGlobalFree   = kernel.NewProc("GlobalFree")

	procDragQueryFile = shell32.NewProc("DragQueryFileW")
)

// dropFiles 对应 DROPFILES 结构。
type dropFiles struct {
	pFiles uint32
	ptX    int32
	ptY    int32
	fNC    int32
	fWide  int32
}

func setHDROPClipboard(paths []string, op ClipboardOperation) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths to copy")
	}

	hdrop, err := buildHDROPGlobal(paths)
	if err != nil {
		return err
	}
	effect := uint32(dropEffectCopy)
	if op == ClipboardMove {
		effect = dropEffectMove
	}
	heffect, err := buildEffectGlobal(effect)
	if err != nil {
		globalFree(hdrop)
		return err
	}

	if err := withClipboardOpen(func() error {
		if err := callEmptyClipboard(); err != nil {
			return err
		}
		if err := callSetClipboardData(cfHDROP, hdrop); err != nil {
			return err
		}
		hdrop = 0 // 系统接管
		effectFormat := registeredPreferredDropEffect()
		if effectFormat != 0 {
			if err := callSetClipboardData(effectFormat, heffect); err != nil {
				// 主格式已设置成功，效果位失败不阻断复制
				return nil
			}
			heffect = 0 // 系统接管
		}
		return nil
	}); err != nil {
		if hdrop != 0 {
			globalFree(hdrop)
		}
		if heffect != 0 {
			globalFree(heffect)
		}
		return err
	}
	return nil
}

// withClipboardOpen 打开剪贴板（失败时短暂重试，避免与其他程序竞争），
// 在闭包内执行操作，最后关闭剪贴板。
func withClipboardOpen(fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		ret, _, _ := procOpenClipboard.Call(0)
		if ret != 0 {
			defer procCloseClipboard.Call()
			return fn()
		}
		lastErr = fmt.Errorf("OpenClipboard failed")
		time.Sleep(10 * time.Millisecond)
	}
	return lastErr
}

func callEmptyClipboard() error {
	ret, _, _ := procEmptyClipboard.Call()
	if ret == 0 {
		return fmt.Errorf("EmptyClipboard failed")
	}
	return nil
}

func callSetClipboardData(format uint16, hGlobal uintptr) error {
	ret, _, _ := procSetClipboardData.Call(uintptr(format), hGlobal)
	if ret == 0 {
		return fmt.Errorf("SetClipboardData(format=%d) failed", format)
	}
	return nil
}

// buildHDROPGlobal 构造 DROPFILES 内存块（UTF-16 路径）。
func buildHDROPGlobal(paths []string) (uintptr, error) {
	utf16Paths := make([][]uint16, len(paths))
	totalBytes := int(unsafe.Sizeof(dropFiles{})) + 2 // 头 + 结尾 \0
	for i, p := range paths {
		utf16Paths[i] = syscall.StringToUTF16(p)
		totalBytes += len(utf16Paths[i]) * 2
	}

	hGlobal, err := globalAlloc(totalBytes)
	if err != nil {
		return 0, err
	}
	lp, err := globalLock(hGlobal)
	if err != nil {
		globalFree(hGlobal)
		return 0, err
	}
	defer globalUnlock(hGlobal)

	*(*dropFiles)(unsafe.Pointer(lp)) = dropFiles{
		pFiles: uint32(unsafe.Sizeof(dropFiles{})),
		fWide:  1,
	}

	offset := int(unsafe.Sizeof(dropFiles{}))
	for _, utf16 := range utf16Paths {
		n := len(utf16)
		// 用字节视图写路径（避免 vet 对 uintptr 算术的误报）
		base := (*byte)(unsafe.Pointer(lp))
		dst := (*[1 << 20]uint16)(unsafe.Add(unsafe.Pointer(base), uintptr(offset)))[:n]
		copy(dst, utf16)
		offset += n * 2
	}
	return hGlobal, nil
}

func buildEffectGlobal(effect uint32) (uintptr, error) {
	hGlobal, err := globalAlloc(4)
	if err != nil {
		return 0, err
	}
	lp, err := globalLock(hGlobal)
	if err != nil {
		globalFree(hGlobal)
		return 0, err
	}
	defer globalUnlock(hGlobal)
	*(*uint32)(unsafe.Pointer(lp)) = effect
	return hGlobal, nil
}

func globalAlloc(size int) (uintptr, error) {
	ret, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if ret == 0 {
		return 0, fmt.Errorf("GlobalAlloc(%d) failed", size)
	}
	return ret, nil
}

func globalLock(hGlobal uintptr) (uintptr, error) {
	ret, _, _ := procGlobalLock.Call(hGlobal)
	if ret == 0 {
		return 0, fmt.Errorf("GlobalLock failed")
	}
	return ret, nil
}

func globalUnlock(hGlobal uintptr) {
	procGlobalUnlock.Call(hGlobal)
}

func globalFree(hGlobal uintptr) {
	if hGlobal != 0 {
		procGlobalFree.Call(hGlobal)
	}
}

var cachedPreferredDropEffect uint16
var cachedPreferredDropEffectDone bool

// registeredPreferredDropEffect 注册（并缓存）Preferred DropEffect 格式。
func registeredPreferredDropEffect() uint16 {
	if cachedPreferredDropEffectDone {
		return cachedPreferredDropEffect
	}
	cachedPreferredDropEffectDone = true
	name, _ := syscall.UTF16PtrFromString(cfstrPreferredDropEffect)
	ret, _, _ := procRegisterClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	cachedPreferredDropEffect = uint16(ret)
	return cachedPreferredDropEffect
}

// ---- 剪贴板读取 ----

func readHDROPClipboard() (ClipboardContent, error) {
	content := ClipboardContent{Paths: []string{}, Op: ClipboardCopy}

	if !clipboardHasHDROP() {
		return content, nil
	}

	err := withClipboardOpen(func() error {
		hdrop, err := callGetClipboardData(cfHDROP)
		if err != nil {
			return err
		}
		paths, err := hdropPaths(hdrop)
		if err != nil {
			return err
		}
		content.Paths = paths

		effectFormat := registeredPreferredDropEffect()
		if effectFormat != 0 {
			if heffect, err := callGetClipboardData(effectFormat); err == nil {
				if effect, err := readEffect(heffect); err == nil && effect&dropEffectMove != 0 {
					content.Op = ClipboardMove
				}
			}
		}
		return nil
	})
	if err != nil {
		return content, nil // 剪贴板无文件引用时视为空，不报错
	}

	seq, err := getClipboardSequenceNumber()
	if err == nil {
		content.Seq = seq
	}
	return content, nil
}

func clipboardHasHDROP() bool {
	ret, _, _ := procIsClipboardFormatAvailable.Call(cfHDROP)
	return ret != 0
}

func callGetClipboardData(format uint16) (uintptr, error) {
	ret, _, _ := procGetClipboardData.Call(uintptr(format))
	if ret == 0 {
		return 0, fmt.Errorf("GetClipboardData(format=%d) failed", format)
	}
	return ret, nil
}

// hdropPaths 用 DragQueryFileW 枚举 CF_HDROP 中的路径（句柄由系统所有，只读）。
func hdropPaths(hdrop uintptr) ([]string, error) {
	countRet, _, _ := procDragQueryFile.Call(hdrop, 0xFFFFFFFF, 0, 0)
	count := uint32(countRet)
	if count == 0 {
		return []string{}, nil
	}

	paths := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		lenRet, _, _ := procDragQueryFile.Call(hdrop, uintptr(i), 0, 0)
		length := uint32(lenRet)
		if length == 0 {
			continue
		}
		buf := make([]uint16, length+1)
		ret, _, _ := procDragQueryFile.Call(hdrop, uintptr(i), uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
		if ret == 0 {
			continue
		}
		paths = append(paths, syscall.UTF16ToString(buf[:length]))
	}
	return paths, nil
}

func readEffect(heffect uintptr) (uint32, error) {
	lp, err := globalLock(heffect)
	if err != nil {
		return 0, err
	}
	defer globalUnlock(heffect)
	return *(*uint32)(unsafe.Pointer(lp)), nil
}

func getClipboardSequenceNumber() (uint64, error) {
	ret, _, _ := procGetClipboardSequenceNumber.Call()
	return uint64(ret), nil
}
