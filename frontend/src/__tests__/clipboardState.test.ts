import { describe, expect, it } from 'vitest'
import {
  isClipboardRewritten,
  snapshotFor,
  syncSnapshot,
  cutPathSetOf,
  hasPasteableContent,
} from '../composables/clipboardState'

describe('clipboardState', () => {
  it('snapshotFor 记录路径与模式', () => {
    const snap = snapshotFor(['/a', '/b'], 'cut', 5)
    expect(snap).toEqual({ paths: ['/a', '/b'], mode: 'cut', seq: 5 })
    // 不共享数组引用
    expect(snap.paths).not.toBe(['/a', '/b'])
  })

  it('isClipboardRewritten：无先前快照则不是改写', () => {
    expect(isClipboardRewritten(null, 10)).toBe(false)
  })

  it('isClipboardRewritten：seq 不变则不是改写', () => {
    const prev = snapshotFor(['/a'], 'cut', 10)
    expect(isClipboardRewritten(prev, 10)).toBe(false)
  })

  it('isClipboardRewritten：seq 变化则是改写', () => {
    const prev = snapshotFor(['/a'], 'cut', 10)
    expect(isClipboardRewritten(prev, 11)).toBe(true)
  })

  it('syncSnapshot：seq 一致保留', () => {
    const prev = snapshotFor(['/a'], 'cut', 10)
    const result = syncSnapshot(prev, 10)
    expect(result.changed).toBe(false)
    expect(result.snapshot).toEqual(prev)
  })

  it('syncSnapshot：seq 不一致清空', () => {
    const prev = snapshotFor(['/a'], 'cut', 10)
    const result = syncSnapshot(prev, 11)
    expect(result.changed).toBe(true)
    expect(result.snapshot).toBeNull()
  })

  it('cutPathSetOf：cut 模式返回路径集合', () => {
    const snap = snapshotFor(['/a', '/b'], 'cut', 1)
    const set = cutPathSetOf(snap)
    expect(set.has('/a')).toBe(true)
    expect(set.has('/c')).toBe(false)
  })

  it('cutPathSetOf：copy 模式返回空集合', () => {
    const snap = snapshotFor(['/a'], 'copy', 1)
    expect(cutPathSetOf(snap).size).toBe(0)
  })

  it('cutPathSetOf：null 返回空集合', () => {
    expect(cutPathSetOf(null).size).toBe(0)
  })

  it('hasPasteableContent', () => {
    expect(hasPasteableContent(null)).toBe(false)
    expect(hasPasteableContent(snapshotFor([], 'copy', 1))).toBe(false)
    expect(hasPasteableContent(snapshotFor(['/a'], 'copy', 1))).toBe(true)
  })
})
