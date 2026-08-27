import { afterEach, describe, expect, it, vi } from 'vitest'
import { createNativeDragOut, hasExceededDragThreshold, supportsNativeDragOut } from '../composables/nativeDragOut'

const originalWails = (window as any)._wails

afterEach(() => {
  ;(window as any)._wails = originalWails
})

describe('supportsNativeDragOut', () => {
  it('macOS 和 Windows 支持原生拖出', () => {
    for (const os of ['darwin', 'windows']) {
      ;(window as any)._wails = { environment: { OS: os } }
      expect(supportsNativeDragOut()).toBe(true)
    }
  })

  it('Linux 和浏览器预览回退到 HTML5 拖拽', () => {
    for (const os of ['linux', undefined]) {
      ;(window as any)._wails = os ? { environment: { OS: os } } : undefined
      expect(supportsNativeDragOut()).toBe(false)
    }
  })
})

describe('hasExceededDragThreshold', () => {
  it('刚好等于阈值视为越过', () => {
    expect(hasExceededDragThreshold({ x: 0, y: 0 }, { x: 5, y: 0 })).toBe(true)
    expect(hasExceededDragThreshold({ x: 0, y: 0 }, { x: 3, y: 4 })).toBe(true)
  })

  it('小于阈值不越过', () => {
    expect(hasExceededDragThreshold({ x: 0, y: 0 }, { x: 4, y: 0 })).toBe(false)
    expect(hasExceededDragThreshold({ x: 10, y: 10 }, { x: 10, y: 10 })).toBe(false)
  })
})

describe('createNativeDragOut', () => {
  it('指针移动超过阈值触发一次 onDragStart', () => {
    const onDragStart = vi.fn()
    const dnd = createNativeDragOut({ onDragStart })

    dnd.onPointerDown({ x: 0, y: 0 }, ['/a.txt'])
    dnd.onPointerMove({ x: 1, y: 1 })
    dnd.onPointerMove({ x: 6, y: 0 }) // 越过阈值

    expect(onDragStart).toHaveBeenCalledTimes(1)
    expect(onDragStart).toHaveBeenCalledWith(['/a.txt'], { x: 6, y: 0 })
  })

  it('拖拽只触发一次（幂等）', () => {
    const onDragStart = vi.fn()
    const dnd = createNativeDragOut({ onDragStart })

    dnd.onPointerDown({ x: 0, y: 0 }, ['/a'])
    dnd.onPointerMove({ x: 10, y: 0 })
    dnd.onPointerMove({ x: 20, y: 0 }) // 已 started

    expect(onDragStart).toHaveBeenCalledTimes(1)
  })

  it('无路径时不触发', () => {
    const onDragStart = vi.fn()
    const dnd = createNativeDragOut({ onDragStart })

    dnd.onPointerDown({ x: 0, y: 0 }, [])
    dnd.onPointerMove({ x: 10, y: 0 })

    expect(onDragStart).not.toHaveBeenCalled()
  })

  it('pointerup 后重置，可再次拖拽', () => {
    const onDragStart = vi.fn()
    const dnd = createNativeDragOut({ onDragStart })

    dnd.onPointerDown({ x: 0, y: 0 }, ['/a'])
    dnd.onPointerMove({ x: 10, y: 0 })
    expect(onDragStart).toHaveBeenCalledTimes(1)

    dnd.onPointerUp()
    dnd.onPointerDown({ x: 100, y: 100 }, ['/b'])
    dnd.onPointerMove({ x: 110, y: 100 })
    expect(onDragStart).toHaveBeenCalledTimes(2)
    expect(onDragStart).toHaveBeenLastCalledWith(['/b'], { x: 110, y: 100 })
  })
})
