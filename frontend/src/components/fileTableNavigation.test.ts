import { describe, expect, it } from 'vitest'
import { fallbackPageEntryOffset, navigationMouseAction, pageEntryOffset } from './fileTableNavigation'

describe('pageEntryOffset', () => {
  it('uses the visible row count minus one row for page movement context', () => {
    expect(pageEntryOffset(300, 30)).toBe(9)
  })

  it('keeps at least one row of movement for small viewports', () => {
    expect(pageEntryOffset(20, 30)).toBe(1)
  })

  it('falls back when viewport or row size cannot be measured', () => {
    expect(pageEntryOffset(0, 30)).toBe(fallbackPageEntryOffset)
    expect(pageEntryOffset(300, 0)).toBe(fallbackPageEntryOffset)
    expect(pageEntryOffset(Number.NaN, 30)).toBe(fallbackPageEntryOffset)
  })
})

describe('navigationMouseAction', () => {
  it('maps mouse side buttons to back and forward', () => {
    expect(navigationMouseAction(3)).toBe('back')
    expect(navigationMouseAction(4)).toBe('forward')
  })

  it('ignores standard mouse buttons', () => {
    expect(navigationMouseAction(0)).toBeNull()
    expect(navigationMouseAction(1)).toBeNull()
    expect(navigationMouseAction(2)).toBeNull()
    expect(navigationMouseAction(5)).toBeNull()
  })
})

