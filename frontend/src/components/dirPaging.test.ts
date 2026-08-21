import { describe, it, expect } from 'vitest'
import { nextPageRange, shouldContinueFetch } from './dirPaging'

describe('nextPageRange', () => {
  it('returns the exact range when the last chunk is a full page', () => {
    expect(nextPageRange(0, 500, 500)).toEqual({ offset: 0, limit: 500 })
  })

  it('returns a remainder page smaller than pageSize', () => {
    expect(nextPageRange(500, 1200, 500)).toEqual({ offset: 500, limit: 500 })
    expect(nextPageRange(1000, 1200, 500)).toEqual({ offset: 1000, limit: 200 })
  })

  it('returns null when nothing remains to fetch', () => {
    expect(nextPageRange(1200, 1200, 500)).toBeNull()
    expect(nextPageRange(1300, 1200, 500)).toBeNull()
  })

  it('returns null for total == 0 (empty directory)', () => {
    expect(nextPageRange(0, 0, 500)).toBeNull()
  })

  it('returns null when loaded exceeds total (offset out of range)', () => {
    expect(nextPageRange(10, 5, 500)).toBeNull()
  })

  it('handles pageSize boundary: exactly one entry at a time', () => {
    expect(nextPageRange(0, 2, 1)).toEqual({ offset: 0, limit: 1 })
    expect(nextPageRange(1, 2, 1)).toEqual({ offset: 1, limit: 1 })
    expect(nextPageRange(2, 2, 1)).toBeNull()
  })

  it('is defensive about invalid inputs', () => {
    expect(nextPageRange(-1, 10, 5)).toBeNull()
    expect(nextPageRange(0, -1, 5)).toBeNull()
    expect(nextPageRange(0, 10, 0)).toBeNull()
  })
})

describe('shouldContinueFetch', () => {
  it('is true while more entries remain', () => {
    expect(shouldContinueFetch(500, 1200)).toBe(true)
  })

  it('is false when all entries are loaded', () => {
    expect(shouldContinueFetch(1200, 1200)).toBe(false)
    expect(shouldContinueFetch(0, 0)).toBe(false)
  })
})