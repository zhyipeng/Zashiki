// Package nativefs 桥接 Zashiki 与系统文件管理器（Finder/Explorer/文件管理器）的
// 文件交互能力：复制/剪切到系统剪贴板、从系统剪贴板读取文件引用、原生拖拽。
//
// 设计要点：
//   - 平台实现遵循「把文件对象引用写入系统剪贴板」模型，而不是文件二进制。
//   - darwin 走 NSPasteboard + NSURL（cgo/AppKit）；windows 走 IDataObject +
//     CF_HDROP（go-ole + OleSetClipboard）；linux 走 text/uri-list 剪贴板约定。
//   - 第 4 步（Wails → 系统原生拖出）本轮不做，StartDrag 保留桩接口供后续实现。
package nativefs

import "errors"

// ClipboardOperation 表示写入系统剪贴板时文件引用的语义。
type ClipboardOperation int

const (
	// ClipboardCopy 复制语义：目标粘贴后保留源文件。
	ClipboardCopy ClipboardOperation = iota
	// ClipboardMove 剪切/移动语义：目标粘贴后应删除源文件。
	ClipboardMove
)

// DropEffect 表示拖拽操作允许的效果位（与 Windows DROPEFFECT / 调研定义一致）。
type DropEffect uint32

const (
	DropEffectCopy DropEffect = 1 << iota
	DropEffectMove
	DropEffectLink
)

// ClipboardContent 是从系统剪贴板读取到的文件引用。
type ClipboardContent struct {
	Paths []string           `json:"paths"`
	Op    ClipboardOperation `json:"op"`
	Seq   uint64             `json:"seq"`
}

// FileTransfer 是平台剪贴板/拖拽实现的统一接口。
type FileTransfer interface {
	// Copy 将 paths 指向的文件引用以复制语义写入系统剪贴板。
	Copy(paths []string) error
	// Cut 将 paths 指向的文件引用以移动语义写入系统剪贴板。
	Cut(paths []string) error
	// ClipboardFiles 从系统剪贴板解析出文件引用与操作语义。
	ClipboardFiles() (ClipboardContent, error)
	// CurrentSequence 返回平台剪贴板的变更序号：外部程序改写剪贴板后该值变化，
	// 用于前端判断「剪切态 UI」是否仍然可信。
	CurrentSequence() (uint64, error)
	// ClearClipboard 清空系统剪贴板（剪切粘贴完成后消费剪切态）。
	ClearClipboard() error
	// StartDrag 启动原生拖出，阻塞直到拖拽结束（Drop 或 Esc）。
	// effects 声明允许的效果位；x/y 为前端 CSS 坐标（逻辑点，左上原点），
	// 用于合成平台拖拽事件。返回用户实际触发/系统决定的效果（如 move）。
	StartDrag(paths []string, x, y int, effects DropEffect) (DropEffect, error)
}

// ErrNotImplemented 表示对应平台/能力尚未实现（目前仅 Linux 原生拖出）。
var ErrNotImplemented = errors.New("not implemented")

// WindowProvider 返回平台原生窗口句柄（Windows: HWND；darwin: NSWindow*）。
// 由 main.go 在窗口创建后注入，nativefs 保持与 Wails 解耦。
type WindowProvider func() uintptr

// MainThreadDispatcher 在应用主线程上同步执行 fn（AppKit/COM 等要求主线程）。
// 由 main.go 注入 application.InvokeSync。
type MainThreadDispatcher func(fn func())

var windowProvider WindowProvider
var mainThreadDispatcher MainThreadDispatcher

// SetWindowProvider 注册窗口句柄提供器（应用启动时调用一次）。
func SetWindowProvider(p WindowProvider) {
	windowProvider = p
}

// SetMainThreadDispatcher 注册主线程调度器（应用启动时调用一次）。
func SetMainThreadDispatcher(d MainThreadDispatcher) {
	mainThreadDispatcher = d
}

// getWindowHandle 返回当前窗口句柄（无提供器/未注册时为 0）。
func getWindowHandle() uintptr {
	if windowProvider == nil {
		return 0
	}
	return windowProvider()
}

// dispatchOnMain 在主线程上执行 fn；未注入调度器时直接执行。
func dispatchOnMain(fn func()) {
	if mainThreadDispatcher == nil {
		fn()
		return
	}
	mainThreadDispatcher(fn)
}

// NewFileTransfer 返回当前平台的 FileTransfer 实现。
func NewFileTransfer() FileTransfer {
	return newPlatformTransfer()
}
