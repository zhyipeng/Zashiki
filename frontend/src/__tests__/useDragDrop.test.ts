import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  activeDragPaths,
  clearDrag,
  finishDragDrop,
  startNativeFileExplorerDrag,
} from '../composables/useDragDrop'

afterEach(() => {
  finishDragDrop()
  vi.useRealTimers()
})

describe('拖拽会话清理', () => {
  it('旧原生拖拽的结束清理不会误伤新会话', () => {
    vi.useFakeTimers()
    const oldSessionId = startNativeFileExplorerDrag(['/old.txt'], '/source')
    clearDrag(oldSessionId)

    const newSessionId = startNativeFileExplorerDrag(['/new.txt'], '/source')
    vi.advanceTimersByTime(800)
    expect(activeDragPaths()).toEqual(['/new.txt'])

    clearDrag(oldSessionId)
    vi.advanceTimersByTime(800)
    expect(activeDragPaths()).toEqual(['/new.txt'])

    clearDrag(newSessionId)
    vi.advanceTimersByTime(800)
    expect(activeDragPaths()).toEqual([])
  })
})
