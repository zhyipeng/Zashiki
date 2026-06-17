import { describe, expect, it } from 'vitest'
import { formatPreviewSize, isFormattedJsonPreview, isMarkdownPreview, isCodePreview, resolveCodeLanguage, previewTextContent, resolvePreviewRenderer } from './preview'

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

describe('resolveCodeLanguage', () => {
  it('maps extensions to shiki language ids', () => {
    expect(resolveCodeLanguage({ kind: 'text', name: 'app.ts', path: '/tmp/app.ts', mimeType: 'text/typescript', content: '' } as any)).toBe('typescript')
    expect(resolveCodeLanguage({ kind: 'text', name: 'app.js', path: '/tmp/app.js', mimeType: 'text/javascript', content: '' } as any)).toBe('javascript')
    expect(resolveCodeLanguage({ kind: 'text', name: 'style.css', path: '/tmp/style.css', mimeType: 'text/css', content: '' } as any)).toBe('css')
    expect(resolveCodeLanguage({ kind: 'text', name: 'main.go', path: '/tmp/main.go', mimeType: 'text/plain', content: '' } as any)).toBe('go')
    expect(resolveCodeLanguage({ kind: 'text', name: 'data.json', path: '/tmp/data.json', mimeType: 'application/json', content: '' } as any)).toBe('json')
  })

  it('falls back to mimeType when extension is unknown', () => {
    expect(resolveCodeLanguage({ kind: 'text', name: 'Makefile', path: '/tmp/Makefile', mimeType: 'text/x-python', content: '' } as any)).toBe('python')
  })

  it('recognizes special filenames', () => {
    expect(resolveCodeLanguage({ kind: 'text', name: 'Dockerfile', path: '/tmp/Dockerfile', mimeType: 'text/plain', content: '' } as any)).toBe('dockerfile')
    expect(resolveCodeLanguage({ kind: 'text', name: 'Makefile', path: '/tmp/Makefile', mimeType: '', content: '' } as any)).toBe('makefile')
    expect(resolveCodeLanguage({ kind: 'text', name: '.gitignore', path: '/tmp/.gitignore', mimeType: 'text/plain', content: '' } as any)).toBe('gitignore')
  })

  it('returns undefined for unknown languages', () => {
    expect(resolveCodeLanguage({ kind: 'text', name: 'unknown.xyz', path: '/tmp/unknown.xyz', mimeType: 'text/plain', content: '' } as any)).toBeUndefined()
  })

  it('returns plaintext for .txt', () => {
    expect(resolveCodeLanguage({ kind: 'text', name: 'note.txt', path: '/tmp/note.txt', mimeType: 'text/plain', content: '' } as any)).toBe('plaintext')
  })
})

describe('isCodePreview', () => {
  it('returns true for recognized code languages', () => {
    expect(isCodePreview({ kind: 'text', name: 'app.ts', path: '/tmp/app.ts', mimeType: 'text/typescript', content: '' } as any)).toBe(true)
    expect(isCodePreview({ kind: 'text', name: 'main.go', path: '/tmp/main.go', mimeType: 'text/plain', content: '' } as any)).toBe(true)
    expect(isCodePreview({ kind: 'text', name: 'data.json', path: '/tmp/data.json', mimeType: 'application/json', content: '' } as any)).toBe(true)
  })

  it('returns false for markdown', () => {
    expect(isCodePreview({ kind: 'text', name: 'readme.md', path: '/tmp/readme.md', mimeType: 'text/markdown', content: '' } as any)).toBe(false)
  })

  it('returns false for plain text', () => {
    expect(isCodePreview({ kind: 'text', name: 'note.txt', path: '/tmp/note.txt', mimeType: 'text/plain', content: '' } as any)).toBe(false)
  })

  it('returns false for non-text previews', () => {
    expect(isCodePreview({ kind: 'image', name: 'pic.png', path: '/tmp/pic.png', mimeType: 'image/png', content: '' } as any)).toBe(false)
  })

  it('returns false for unrecognized languages', () => {
    expect(isCodePreview({ kind: 'text', name: 'unknown.xyz', path: '/tmp/unknown.xyz', mimeType: 'text/plain', content: '' } as any)).toBe(false)
  })
})
