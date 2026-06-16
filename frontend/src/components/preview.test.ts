import { describe, expect, it } from 'vitest'
import { formatPreviewSize, isFormattedJsonPreview, isMarkdownPreview, previewTextContent, resolvePreviewRenderer } from './preview'

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

  it('resolves pdf preview renderer kind', () => {
    const renderer = resolvePreviewRenderer({ kind: 'pdf' } as any)
    expect(renderer.kind).toBe('pdf')
    expect(renderer.label).toBe('PDF')
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

describe('previewTextContent', () => {
  it('formats json previews without changing the source content', () => {
    const preview = {
      kind: 'text',
      mimeType: 'application/json',
      name: 'data.json',
      path: '/tmp/data.json',
      content: '{"name":"zashiki","items":[1,2]}',
    } as any

    expect(previewTextContent(preview)).toBe('{\n  "name": "zashiki",\n  "items": [\n    1,\n    2\n  ]\n}')
    expect(preview.content).toBe('{"name":"zashiki","items":[1,2]}')
    expect(isFormattedJsonPreview(preview)).toBe(true)
  })

  it('keeps invalid json unchanged', () => {
    const preview = {
      kind: 'text',
      mimeType: 'application/json',
      name: 'broken.json',
      path: '/tmp/broken.json',
      content: '{"name":',
    } as any

    expect(previewTextContent(preview)).toBe('{"name":')
    expect(isFormattedJsonPreview(preview)).toBe(false)
  })

  it('keeps non-json text unchanged', () => {
    const preview = {
      kind: 'text',
      mimeType: 'text/plain',
      name: 'note.txt',
      path: '/tmp/note.txt',
      content: '{"name":"not-json-preview"}',
    } as any

    expect(previewTextContent(preview)).toBe('{"name":"not-json-preview"}')
  })
})

describe('isMarkdownPreview', () => {
  it('recognizes markdown by mimeType', () => {
    expect(isMarkdownPreview({ kind: 'text', mimeType: 'text/markdown', name: 'doc', path: '/tmp/doc' } as any)).toBe(true)
  })

  it('recognizes markdown by .md extension in name', () => {
    expect(isMarkdownPreview({ kind: 'text', mimeType: 'text/plain', name: 'readme.md', path: '/tmp/readme.md' } as any)).toBe(true)
  })

  it('recognizes markdown by .md extension in path', () => {
    expect(isMarkdownPreview({ kind: 'text', mimeType: 'text/plain', name: 'README', path: '/tmp/README.md' } as any)).toBe(true)
  })

  it('returns false for non-text preview', () => {
    expect(isMarkdownPreview({ kind: 'image', mimeType: 'image/png', name: 'pic.png', path: '/tmp/pic.png' } as any)).toBe(false)
  })

  it('returns false for non-markdown text', () => {
    expect(isMarkdownPreview({ kind: 'text', mimeType: 'text/plain', name: 'note.txt', path: '/tmp/note.txt' } as any)).toBe(false)
  })
})
