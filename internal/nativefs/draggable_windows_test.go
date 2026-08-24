//go:build windows

package nativefs

// 验证 buildFileDataObject 产出的 Shell 数据对象能被 Explorer 之类拖放目标
// 识别：必须暴露 CF_HDROP（含正确路径）。回归：早期实现用「NULL 父目录 +
// 绝对 PIDL」会生成空对象，导致拖拽光标一直「禁止」、无法释放。

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

var (
	procOleSetClipboardTest = ole32ForDrag.NewProc("OleSetClipboard")
	procOleFlushClipboard   = ole32ForDrag.NewProc("OleFlushClipboard")
)

// FORMATETC（x64，MSVC 布局）：WORD cfFormat + 6B padding + 指针 ptd + DWORD dwAspect + LONG lindex + DWORD tymed
type formatEtc struct {
	cfFormat uint16
	_        [6]byte
	ptd      uintptr
	dwAspect uint32
	lindex   int32
	tymed    uint32
}

// STGMEDIUM（x64）：DWORD tymed + union（8B，按 HGLOBAL）+ IUnknown* pUnkForRelease
type stgMedium struct {
	tymed   uint32
	_       [4]byte
	hGlobal uintptr
	pUnk    uintptr
}

func TestFileDataObjectExposesHDROP(t *testing.T) {
	// OLE/COM 必须在同一 OS 线程（和真实主线程回调一致）。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if ret, _, _ := procOleInitializeDrag.Call(0); ret != 0 && ret != 1 {
		t.Fatalf("OleInitialize failed: 0x%08x", ret)
	}
	defer procOleUninitialize.Call()

	// 构造两个真实临时文件（Windows 下 t.TempDir() 可能是 8.3 短路径，正好
	// 覆盖该场景，实际文件列表路径同样适用）。
	dir := t.TempDir()
	paths := []string{}
	for _, name := range []string{"a.txt", "b.txt"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	obj, err := buildFileDataObject(paths)
	if err != nil {
		t.Fatalf("buildFileDataObject: %v", err)
	}
	defer obj.Release()

	vtbl := *(*uintptr)(unsafe.Pointer(obj.obj))

	// EnumFormatEtc(DATADIR_GET=1, &pEnum) —— IDataObject vtable 槽位 8
	var pEnum uintptr
	enumFn := *(*uintptr)(unsafe.Pointer(vtbl + 8*unsafe.Sizeof(uintptr(0))))
	if hr := comCall(enumFn, obj.obj, 1, uintptr(unsafe.Pointer(&pEnum))); hr != 0 {
		t.Fatalf("EnumFormatEtc failed: %#x", hr)
	}
	if pEnum == 0 {
		t.Fatal("EnumFormatEtc returned no enumerator")
	}

	enumVtbl := *(*uintptr)(unsafe.Pointer(pEnum))
	nextFn := *(*uintptr)(unsafe.Pointer(enumVtbl + 3*unsafe.Sizeof(uintptr(0))))
	releaseEnum := *(*uintptr)(unsafe.Pointer(enumVtbl + 2*unsafe.Sizeof(uintptr(0))))
	defer comCall(releaseEnum, pEnum)

	foundHDROP := false
	var fe formatEtc
	var fetched uintptr
	for {
		fetched = 0
		if comCall(nextFn, pEnum, 1, uintptr(unsafe.Pointer(&fe)), uintptr(unsafe.Pointer(&fetched))) != 0 || fetched == 0 {
			break
		}
		if fe.cfFormat == 15 { // CF_HDROP
			foundHDROP = true
			break
		}
	}
	if !foundHDROP {
		t.Fatal("data object does not expose CF_HDROP")
	}

	// GetData({CF_HDROP, DVASPECT_CONTENT, -1, TYMED_HGLOBAL}, &stg) —— 槽位 3
	getDataFn := *(*uintptr)(unsafe.Pointer(vtbl + 3*unsafe.Sizeof(uintptr(0))))
	fetc := formatEtc{cfFormat: 15, dwAspect: 1, lindex: -1, tymed: 1}
	var stg stgMedium
	if hr := comCall(getDataFn, obj.obj, uintptr(unsafe.Pointer(&fetc)), uintptr(unsafe.Pointer(&stg))); hr != 0 {
		t.Fatalf("GetData(CF_HDROP) failed: %#x", hr)
	}
	got, _ := hdropPaths(stg.hGlobal)
	globalFree(stg.hGlobal)

	if len(got) != len(paths) {
		t.Fatalf("path count mismatch: got %d want %d (%v)", len(got), len(paths), got)
	}
	// Shell 会把 8.3 短路径规范化为长路径，比对前统一规范化。
	normalize := func(p string) string {
		if lp, err := filepath.EvalSymlinks(p); err == nil {
			return lp
		}
		return p
	}
	for i := range paths {
		if normalize(got[i]) != normalize(paths[i]) {
			t.Errorf("path[%d] mismatch: got %q want %q", i, got[i], paths[i])
		}
	}

	// 再走系统侧：把对象放进 OLE 剪贴板，系统应识别 CF_HDROP。
	comCall(procOleFlushClipboard.Addr())
	if hr, _, _ := procOleSetClipboardTest.Call(obj.obj); hr != 0 {
		t.Fatalf("OleSetClipboard failed: %#x", hr)
	}
	defer func() { _, _, _ = procOleSetClipboardTest.Call(0) }()
	if !clipboardHasHDROP() {
		t.Fatal("system clipboard does not expose CF_HDROP after OleSetClipboard")
	}
}

// comCall 对 COM 槽位函数做一次性调用，返回 HRESULT。
func comCall(proc uintptr, args ...uintptr) uintptr {
	r1, _, _ := syscall.SyscallN(proc, args...)
	return r1
}
