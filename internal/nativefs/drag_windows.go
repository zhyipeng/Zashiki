//go:build windows

package nativefs

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Windows 原生拖出（Wails → Explorer）。
//
// 方案：构建一个最小的 IDataObject（暴露 CF_HDROP + Preferred DropEffect），
// 在专用 OLE STA 线程上调用 SHDoDragDrop(hwnd, dataObject, NULL, effects,
// &effect)。SHDoDragDrop 阻塞直到用户释放/Esc，返回实际触发的 effect
// （copy/move/link），供前端判断是否刷新源目录。
//
// 线程模型：OLE/拖拽要求 STA。Wails 的 RPC 在 goroutine 中执行，因此
// StartDrag 在独立 STA 线程运行 SHDoDragDrop，阻塞等待其返回（与消息循环
// 互不冲突——SHDoDragDrop 自带 message loop）。

var (
	ole32ForDrag = syscall.NewLazyDLL("ole32.dll")
	shell32Drag  = syscall.NewLazyDLL("shell32.dll")

	procOleInitializeDrag  = ole32ForDrag.NewProc("OleInitialize")
	procOleUninitialize    = ole32ForDrag.NewProc("OleUninitialize")
	procSHDoDragDrop       = shell32Drag.NewProc("SHDoDragDrop")
	procSHCreateDataObject = shell32Drag.NewProc("SHCreateDataObject")
	procILCreateFromPath   = shell32Drag.NewProc("ILCreateFromPathW")
	procILFree             = shell32Drag.NewProc("ILFree")
)

// IID_IDataObject = {0000010E-0000-0000-C000-000000000046}
var iidIDataObject = &guid{
	Data1: 0x0000010E,
	Data2: 0x0000,
	Data3: 0x0000,
	Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// startWindowsDrag 在 OLE STA 线程上执行 SHDoDragDrop。
func startWindowsDrag(paths []string, effects DropEffect) (DropEffect, error) {
	hwnd := getWindowHandle()
	if hwnd == 0 {
		return 0, fmt.Errorf("no window handle for drag")
	}

	resultCh := make(chan dragResult, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hr := oleInitializeOnThread()
		if hr != 0 {
			resultCh <- dragResult{err: fmt.Errorf("OleInitialize failed: 0x%08x", hr)}
			return
		}
		defer procOleUninitialize.Call()

		effect, err := runSHDoDragDrop(hwnd, paths, effects)
		resultCh <- dragResult{effect: effect, err: err}
	}()
	res := <-resultCh
	return res.effect, res.err
}

type dragResult struct {
	effect DropEffect
	err    error
}

func oleInitializeOnThread() uintptr {
	// OleInitialize(NULL)
	ret, _, _ := procOleInitializeDrag.Call(0)
	return ret
}

// runSHDoDragDrop 构建数据对象并调用 SHDoDragDrop。
func runSHDoDragDrop(hwnd uintptr, paths []string, effects DropEffect) (DropEffect, error) {
	dataObject, err := buildFileDataObject(paths)
	if err != nil {
		return 0, err
	}
	defer releaseIDataObject(dataObject)

	var dwEffect uint32
	// SHDoDragDrop(hwnd, pdata, pdropSource, dwOKEffects, pdwEffect)
	ret, _, _ := procSHDoDragDrop.Call(
		hwnd,
		dataObject,
		0, // pdropSource (nil)
		uintptr(effects),
		uintptr(unsafe.Pointer(&dwEffect)),
	)
	// 成功（S_OK=0 或 DRAGDROP_S_DROP=0x00040100）；其余为失败
	hr := uint32(ret)
	success := hr == 0 || hr == 0x00040100
	if !success {
		return 0, fmt.Errorf("SHDoDragDrop failed: 0x%08x", hr)
	}
	return DropEffect(dwEffect), nil
}

// ---- IDataObject 构建（SHCreateDataObject） ----

// buildFileDataObject 用 SHCreateDataObject 从 PIDL 数组创建 Shell 数据对象。
// 这比手写 COM vtable 稳定，且 Explorer 完全识别。
func buildFileDataObject(paths []string) (uintptr, error) {
	if len(paths) == 0 {
		return 0, fmt.Errorf("no paths for drag")
	}

	pidls := make([]uintptr, len(paths))
	released := false
	defer func() {
		if !released {
			for _, pidl := range pidls {
				if pidl != 0 {
					procILFree.Call(pidl)
				}
			}
		}
	}()

	for i, p := range paths {
		pidl, err := createPIDLFromPath(p)
		if err != nil {
			return 0, err
		}
		pidls[i] = pidl
	}

	var dataObject uintptr
	// SHCreateDataObject(pcidlFolder, cidl, apidl, pdtInner, riid, ppv)
	ret, _, _ := procSHCreateDataObject.Call(
		0,
		uintptr(len(pidls)),
		uintptr(unsafe.Pointer(&pidls[0])),
		0,
		uintptr(unsafe.Pointer(iidIDataObject)),
		uintptr(unsafe.Pointer(&dataObject)),
	)
	if uint32(ret) != 0 {
		return 0, fmt.Errorf("SHCreateDataObject failed: 0x%08x", uint32(ret))
	}
	if dataObject == 0 {
		return 0, fmt.Errorf("SHCreateDataObject returned nil")
	}

	released = true
	// 释放 PIDL（数据对象已持有引用）
	for _, pidl := range pidls {
		if pidl != 0 {
			procILFree.Call(pidl)
		}
	}
	return dataObject, nil
}

// createPIDLFromPath 用 ILCreateFromPathW 从路径创建 PIDL。
func createPIDLFromPath(path string) (uintptr, error) {
	pw, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	ret, _, _ := procILCreateFromPath.Call(uintptr(unsafe.Pointer(pw)))
	if ret == 0 {
		return 0, fmt.Errorf("ILCreateFromPath failed for %q", path)
	}
	return ret, nil
}

// releaseIDataObject 释放 IDataObject（IUnknown::Release，vtable 第 2 槽位）。
func releaseIDataObject(dataObject uintptr) {
	if dataObject == 0 {
		return
	}
	vtbl := *(*uintptr)(unsafe.Pointer(dataObject))
	release := *(*uintptr)(unsafe.Pointer(vtbl + 2*unsafe.Sizeof(uintptr(0))))
	syscall.SyscallN(release, dataObject)
}
