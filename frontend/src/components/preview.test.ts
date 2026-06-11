import { describe, expect, it } from 'vitest'
import { formatPreviewSize, resolvePreviewRenderer } from './preview'

describe('preview renderer registry', () => {
  it('resolves known preview renderer kinds', () => {
    const renderer = resolvePreviewRenderer({ kind: 'text' } as any)
    expect(renderer.kind).toBe('text')
    expect(renderer.label).toBe('文本')
  })

  it('falls back to unsupported renderer for unknown kinds', () => {
    const renderer = resolvePreviewRenderer({ kind: 'video' } as any)
    expect(renderer.kind).toBe('unsupported')
  })
})

describe('formatPreviewSize', () => {
  it('formats zero bytes', () => {
    expect(formatPreviewSize(0)).toBe('0 B')
  })

  it('formats larger sizes with one decimal place', () => {
    expect(formatPreviewSize(1536)).toBe('1.5 KB')
  })
})
