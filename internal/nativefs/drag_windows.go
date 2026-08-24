//go:build windows

package nativefs

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// Windows 原生拖出（Wails → Explorer）。
//
// 方案：构建一个最小的 IDataObject（暴露 CF_HDROP + Preferred DropEffect），
// 调用 SHDoDragDrop(hwnd, dataObject, NULL, effects, &effect)。SHDoDragDrop
// 阻塞直到用户释放/Esc，返回实际触发的 effect（copy/move/link），供前端
// 判断是否刷新源目录。
//
// 线程模型：SHDoDragDrop 内部运行自己的消息循环，并要求调用线程就是拥有
// 源窗口 hwnd 的线程（Wails 主线程）。拖拽期间 OLE 需要对 hwnd 设置鼠标
// 捕获，而只有拥有窗口的线程允许 SetCapture；鼠标消息（WM_MOUSEMOVE /
// WM_LBUTTONUP）也只投递到拥有窗口的主线程消息队列。若像早期实现那样在
// 独立 worker goroutine 上调用：SetCapture 失败、拖拽循环永远收不到鼠标
// 消息 → 拖拽无反馈、无法落下、RPC 持续挂起（表现为「无效且无报错」）。
// 因此与 darwin 一致，通过 dispatchOnMain（application.InvokeSync）把整个
// 拖拽会话调度到主线程执行。

var (
	ole32ForDrag = syscall.NewLazyDLL("ole32.dll")
	shell32Drag  = syscall.NewLazyDLL("shell32.dll")

	procOleInitializeDrag  = ole32ForDrag.NewProc("OleInitialize")
	procOleUninitialize    = ole32ForDrag.NewProc("OleUninitialize")
	procSHDoDragDrop       = shell32Drag.NewProc("SHDoDragDrop")
	procSHCreateDataObject = shell32Drag.NewProc("SHCreateDataObject")
	procILCreateFromPath   = shell32Drag.NewProc("ILCreateFromPathW")
	procILFree             = shell32Drag.NewProc("ILFree")
	procILClone            = shell32Drag.NewProc("ILClone")
	procILFindLastID       = shell32Drag.NewProc("ILFindLastID")
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

// dragTimeout 拖拽会话总时限（超时避免 RPC 永久挂起，与 darwin 一致）。
const dragTimeout = 60 * time.Second

// startWindowsDrag 在拥有 hwnd 的线程（Wails 主线程）上执行 SHDoDragDrop。
func startWindowsDrag(paths []string, effects DropEffect) (DropEffect, error) {
	if len(paths) == 0 {
		return 0, fmt.Errorf("no paths for drag")
	}
	hwnd := getWindowHandle()
	if hwnd == 0 {
		return 0, fmt.Errorf("no window handle for drag")
	}

	done := make(chan dragResult, 1)
	go func() {
		dispatchOnMain(func() {
			hr := oleInitializeOnThread()
			// S_OK=0 或 S_FALSE=1 均为成功；其他（如 RPC_E_CHANGED_MODE）
			// 说明该线程已是 MTA，无法在其上运行 OLE 拖拽。
			if hr != 0 && hr != 1 {
				done <- dragResult{err: fmt.Errorf("OleInitialize failed: 0x%08x", hr)}
				return
			}
			defer procOleUninitialize.Call()

			effect, err := runSHDoDragDrop(hwnd, paths, effects)
			done <- dragResult{effect: effect, err: err}
		})
	}()

	select {
	case res := <-done:
		return res.effect, res.err
	case <-time.After(dragTimeout):
		// 兜底：拖拽异常未结束时避免 StartDrag 入口永久挂起。
		return 0, fmt.Errorf("native drag timed out after %s", dragTimeout)
	}
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
	dragObj, err := buildFileDataObject(paths)
	if err != nil {
		return 0, err
	}
	// 数据对象借用 folder/child PIDL，必须保持存活直到拖拽结束（跨进程
	// 渲染 CF_HDROP 时才读取它们），因此拖拽返回后再统一释放。
	defer dragObj.Release()

	var dwEffect uint32
	// SHDoDragDrop(hwnd, pdata, pdropSource, dwOKEffects, pdwEffect)
	ret, _, _ := procSHDoDragDrop.Call(
		hwnd,
		dragObj.obj,
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

// dragDataObject 持有 Shell 数据对象及其借用的 PIDL。
//
// SHCreateDataObject 只「借用」传入的父目录 PIDL 与子 PIDL、不会复制；
// 数据对象在跨进程渲染（GetData/拖放目标读取 CF_HDROP）时才真正读取它们。
// 因此这些 PIDL 必须保持存活直到数据对象释放，由 Release 统一清理。
type dragDataObject struct {
	obj      uintptr   // IDataObject*
	folder   uintptr   // 父目录 PIDL
	children []uintptr // 子 PIDL 数组
}

func (d *dragDataObject) Release() {
	if d == nil {
		return
	}
	if d.obj != 0 {
		releaseIDataObject(d.obj)
	}
	if d.folder != 0 {
		procILFree.Call(d.folder)
	}
	for _, c := range d.children {
		if c != 0 {
			procILFree.Call(c)
		}
	}
	*d = dragDataObject{}
}

// buildFileDataObject 用 SHCreateDataObject 创建文件拖拽数据对象。
//
// 关键（踩坑记录）：pidlFolder 必须传「父目录 PIDL」而非 NULL，apidl 传
// 相对父目录的「子 PIDL」。早期实现传 NULL folder + 绝对 PIDL，在部分
// Windows 上会生成一个不暴露任何格式的空对象 —— Explorer 的
// QueryGetData(CF_HDROP) 失败 → 光标始终「禁止」、无法释放。
func buildFileDataObject(paths []string) (*dragDataObject, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no paths for drag")
	}

	folder := filepath.Dir(paths[0])
	if len(folder) == 2 && folder[1] == ':' {
		folder += `\` // 卷根（如 C:\a.txt）时 filepath.Dir 返回 "C:"，需补反斜杠
	}
	folderPidl, err := createPIDLFromPath(folder)
	if err != nil {
		return nil, err
	}

	children := make([]uintptr, len(paths))
	cleanupChildren := func() {
		for _, c := range children {
			if c != 0 {
				procILFree.Call(c)
			}
		}
		procILFree.Call(folderPidl)
	}

	for i, p := range paths {
		abs, err := createPIDLFromPath(p)
		if err != nil {
			cleanupChildren()
			return nil, err
		}
		child, _, _ := procILFindLastID.Call(abs)
		if child == 0 {
			procILFree.Call(abs)
			cleanupChildren()
			return nil, fmt.Errorf("ILFindLastID failed for %q", p)
		}
		// 克隆子 PIDL：abs 即将释放，而数据对象会借用它
		clone, _, _ := procILClone.Call(child)
		procILFree.Call(abs)
		if clone == 0 {
			cleanupChildren()
			return nil, fmt.Errorf("ILClone failed for %q", p)
		}
		children[i] = clone
	}

	var dataObject uintptr
	// SHCreateDataObject(pcidlFolder, cidl, apidl, pdtInner, riid, ppv)
	ret, _, _ := procSHCreateDataObject.Call(
		folderPidl,
		uintptr(len(children)),
		uintptr(unsafe.Pointer(&children[0])),
		0,
		uintptr(unsafe.Pointer(iidIDataObject)),
		uintptr(unsafe.Pointer(&dataObject)),
	)
	if uint32(ret) != 0 {
		cleanupChildren()
		return nil, fmt.Errorf("SHCreateDataObject failed: 0x%08x", uint32(ret))
	}
	if dataObject == 0 {
		cleanupChildren()
		return nil, fmt.Errorf("SHCreateDataObject returned nil")
	}

	return &dragDataObject{obj: dataObject, folder: folderPidl, children: children}, nil
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
