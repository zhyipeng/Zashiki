import { describe, expect, it } from 'vitest'
import {
  applyMarqueeSelection,
  isMarqueeDrag,
  marqueeDragThreshold,
  marqueeRect,
  rectsIntersect,
} from './galleryMarquee'
import type { MarqueeItemRect } from './galleryMarquee'

const item = (path: string, left: number, top: number): MarqueeItemRect => ({
  path,
  left,
  top,
  width: 100,
  height: 120,
})

const items = [
  item('a', 0, 0),
  item('b', 120, 0),
  item('c', 0, 140),
  item('d', 120, 140),
]

describe('isMarqueeDrag', () => {
  it('ignores movement within threshold', () => {
    expect(isMarqueeDrag({ x: 10, y: 10 }, { x: 13, y: 10 })).toBe(false)
  })
  it('activates beyond threshold on either axis', () => {
    expect(isMarqueeDrag({ x: 10, y: 10 }, { x: 10 + marqueeDragThreshold + 1, y: 10 })).toBe(true)
    expect(isMarqueeDrag({ x: 10, y: 10 }, { x: 10, y: 10 - marqueeDragThreshold - 1 })).toBe(true)
  })
})

describe('marqueeRect', () => {
  it('normalizes opposite drag directions', () => {
    expect(marqueeRect({ x: 50, y: 80 }, { x: 10, y: 20 })).toEqual({
      left: 10, top: 20, right: 50, bottom: 80,
    })
  })
})

describe('rectsIntersect', () => {
  it('detects overlap', () => {
    expect(rectsIntersect({ left: 50, top: 50, right: 200, bottom: 200 }, item('a', 0, 0))).toBe(true)
  })
  it('rejects items only touching the rect edge', () => {
    expect(rectsIntersect({ left: 100, top: 0, right: 200, bottom: 100 }, item('a', 0, 0))).toBe(false)
    expect(rectsIntersect({ left: -50, top: 120, right: 0, bottom: 200 }, item('a', 0, 0))).toBe(false)
  })
})

describe('applyMarqueeSelection', () => {
  const rect = { left: 50, top: 50, right: 300, bottom: 300 }

  it('replaces selection with intersected items', () => {
    // 四个单元都与 50,50 起的矩形有重叠
    expect(applyMarqueeSelection(['a', 'b'], items, rect, false)).toEqual(['a', 'b', 'c', 'd'])
  })

  it('toggles intersected items against the base selection in additive mode', () => {
    // 全部在框内：已选的 a/b 被取消，未选的 c/d 被选中
    expect(applyMarqueeSelection(['a', 'b'], items, rect, true)).toEqual(['c', 'd'])
  })

  it('keeps base selection untouched in additive mode when rect misses everything', () => {
    const rectMiss = { left: 500, top: 500, right: 600, bottom: 600 }
    expect(applyMarqueeSelection(['a'], items, rectMiss, true)).toEqual(['a'])
  })

  it('treats zero-area rect as a point probe', () => {
    // 点落在单元内 → 选中该单元（实际交互中框选只从空白处发起，不会命中）
    expect(applyMarqueeSelection(['a'], items, { left: 5, top: 5, right: 5, bottom: 5 }, false)).toEqual(['a'])
    // 点落在单元间隙 → 空选中
    expect(applyMarqueeSelection(['a'], items, { left: 110, top: 5, right: 110, bottom: 5 }, false)).toEqual([])
  })
})
