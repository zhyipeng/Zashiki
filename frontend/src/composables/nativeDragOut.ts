// nativeDragOut 提供「Wails → 系统」原生拖出的前端指针检测。
//
// 设计（调研文档第 4 步）：
//   - 不用 HTML draggable（HTML 拖拽无法逃出窗口，且与原生拖拽冲突）。
//   - pointerdown 记录起点；pointermove 超过阈值（默认 5px）后调用一次
//     onDragStart(paths)，进入原生拖拽。
//   - pointerup / pointercancel 重置状态。
//
// 纯逻辑（detectDragThreshold）独立成函数，便于 Vitest 覆盖。

export interface PointerPoint {
  x: number
  y: number
}

export const DRAG_THRESHOLD_PX = 5

/** 指针移动是否超过拖拽阈值（从起点开始的欧氏距离）。 */
export function hasExceededDragThreshold(start: PointerPoint, current: PointerPoint, threshold = DRAG_THRESHOLD_PX): boolean {
  const dx = current.x - start.x
  const dy = current.y - start.y
  return Math.hypot(dx, dy) >= threshold
}

export interface NativeDragOutOptions {
  /** 阈值（px），默认 5。 */
  threshold?: number
  /** 拖动超过阈值后调用（幂等：一次拖拽只调一次）。参数为路径快照与越过阈值时的指针位置。 */
  onDragStart?: (paths: string[], point: PointerPoint) => void
}

/**
 * 指针起点与本次拖拽的路径快照（pointerdown 时由调用方捕获，
 * 避免拖拽开始时选中状态已被后续 click 改变）。
 */
export interface NativeDragOutState {
  active: boolean
  started: boolean
  startPoint: PointerPoint | null
  paths: string[]
}

export function createNativeDragOut(options: NativeDragOutOptions) {
  const threshold = options.threshold ?? DRAG_THRESHOLD_PX
  const state: NativeDragOutState = {
    active: false,
    started: false,
    startPoint: null,
    paths: [],
  }

  function onPointerDown(point: PointerPoint, paths: string[]): void {
    state.active = true
    state.started = false
    state.startPoint = point
    state.paths = paths
  }

  function onPointerMove(point: PointerPoint): boolean {
    if (!state.active || state.started) return false
    if (!state.startPoint) return false
    if (!hasExceededDragThreshold(state.startPoint, point, threshold)) return false
    // 已越过阈值：触发原生拖拽（幂等）
    state.started = true
    if (state.paths.length > 0) {
      options.onDragStart?.(state.paths, point)
    }
    return true
  }

  function onPointerUp(): void {
    state.active = false
    state.started = false
    state.startPoint = null
    state.paths = []
  }

  function reset(): void {
    onPointerUp()
  }

  return {
    state,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    reset,
  }
}
