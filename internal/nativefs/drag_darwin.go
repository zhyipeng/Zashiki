//go:build darwin

package nativefs

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>

// 在窗口内启动拖拽会话（必须在主线程调用）。返回 0 成功。
int startNativeFileDrag(void *windowPtr, const char **paths, int count, int x, int y);
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// darwinDrag 处理 macOS 原生拖出（Wails → Finder）。
//
// 方案：合成事件 + 主线程调度（确定性优于 event monitor）。
//   - 前端 pointer 阈值检测 → StartDrag(paths, x, y)
//   - dispatchOnMain 在主线程创建合成 LeftMouseDragged 事件，
//     beginDraggingSessionWithItems 启动系统拖拽
//   - NSDraggingSource ended 回调 → nativeDragEnded(effect) → 解除阻塞
type darwinDrag struct {
	mu      sync.Mutex
	pending map[uint64]chan DropEffect
	nextID  uint64
}

// nativeDragEnded 由 ObjC 侧回调：通知对应等待者拖拽已结束。
//
//export nativeDragEnded
func nativeDragEnded(effect C.int) {
	globalDrag.resolve(int(effect))
}

var globalDrag = newDarwinDrag()

func newDarwinDrag() *darwinDrag {
	return &darwinDrag{pending: map[uint64]chan DropEffect{}}
}

// StartDrag 在主线程启动原生拖拽并阻塞直到拖拽会话结束，返回最终 effect。
func (d *darwinDrag) StartDrag(paths []string, x, y int, effects DropEffect) (DropEffect, error) {
	if len(paths) == 0 {
		return 0, fmt.Errorf("no paths to drag")
	}
	windowPtr := getWindowHandle()
	if windowPtr == 0 {
		return 0, fmt.Errorf("no window handle for drag")
	}

	// 注册等待者（先注册，避免回调先于等待发生）
	id, done := d.register()
	defer d.unregister(id)

	// 必须在主线程调用 AppKit
	dispatchOnMain(func() {
		d.startOnMain(paths, x, y, windowPtr)
	})

	select {
	case effect := <-done:
		if effect == 0 {
			return effects, nil // 无 effect（如 Esc 取消）视为保持允许集合
		}
		return effect, nil
	case <-time.After(dragTimeout):
		return 0, fmt.Errorf("native drag timed out after %s", dragTimeout)
	}
}

// startOnMain 在主线程执行实际的拖拽启动。
func (d *darwinDrag) startOnMain(paths []string, x, y int, windowPtr uintptr) {
	cpaths := make([]*C.char, len(paths))
	for i, p := range paths {
		cpaths[i] = C.CString(p)
	}
	defer func() {
		for _, cp := range cpaths {
			C.free(unsafe.Pointer(cp))
		}
	}()

	ret := C.startNativeFileDrag(
		unsafe.Pointer(windowPtr),
		&cpaths[0],
		C.int(len(paths)),
		C.int(x),
		C.int(y),
	)
	if ret != 0 {
		// 拖拽未启动（参数错误/异常）。若 ObjC 侧已回调 nativeDragEnded，
		// 等待者会被立即解除；否则在这里主动通知，避免 RPC 挂到超时。
		d.resolveIfPending(0)
	}
}

// resolveIfPending 若有等待者则立即以 effect 通知（幂等）。
func (d *darwinDrag) resolveIfPending(effect int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pending) == 0 {
		return
	}
	effectDrop := dropEffectFromInt(effect)
	for id, ch := range d.pending {
		ch <- effectDrop
		delete(d.pending, id)
	}
}

// dragTimeout 拖拽会话总时限（超时避免 RPC 永久挂起）。
const dragTimeout = 60 * time.Second

func (d *darwinDrag) register() (uint64, chan DropEffect) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	id := d.nextID
	ch := make(chan DropEffect, 1)
	d.pending[id] = ch
	return id, ch
}

func (d *darwinDrag) unregister(id uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.pending, id)
}

func (d *darwinDrag) resolve(effect int) {
	d.resolveIfPending(effect)
}

// dropEffectFromInt 把 AppKit NSDragOperation 位掩码转成 DropEffect。
func dropEffectFromInt(op int) DropEffect {
	var e DropEffect
	if op&1 != 0 { // NSDragOperationCopy
		e |= DropEffectCopy
	}
	if op&2 != 0 { // NSDragOperationMove
		e |= DropEffectMove
	}
	if op&4 != 0 { // NSDragOperationLink
		e |= DropEffectLink
	}
	return e
}
