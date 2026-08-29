import { describe, expect, it } from 'vitest'
import { encodeAssetPath, htmlAssetUrl, thumbnailUrl } from './assetUrl'

describe('encodeAssetPath', () => {
  it('produces base64url compatible with Go base64.URLEncoding', () => {
    expect(encodeAssetPath('/tmp/a.png')).toBe(btoa('/tmp/a.png').replaceAll('+', '-').replaceAll('/', '_'))
  })

  it('keeps padding and replaces + and /', () => {
    const encoded = encodeAssetPath('a+b/c?d=ef')
    expect(encoded).not.toContain('+')
    expect(encoded).not.toContain('/')
    expect(encoded.endsWith('=')).toBe(true)
  })

  it('encodes non-ASCII (UTF-8) paths', () => {
    const encoded = encodeAssetPath('/home/用户/图片.png')
    const decoded = new TextDecoder().decode(
      Uint8Array.from(atob(encoded.replaceAll('-', '+').replaceAll('_', '/')), char => char.charCodeAt(0)),
    )
    expect(decoded).toBe('/home/用户/图片.png')
  })
})

describe('asset urls', () => {
  it('builds html asset url', () => {
    expect(htmlAssetUrl('/tmp/a.css')).toBe(`/__html_assets__/${encodeAssetPath('/tmp/a.css')}`)
  })

  it('builds thumbnail url with size query', () => {
    expect(thumbnailUrl('/tmp/a.png')).toBe(`/__thumbnails__/${encodeAssetPath('/tmp/a.png')}?s=256`)
    expect(thumbnailUrl('/tmp/a.png', 128)).toBe(`/__thumbnails__/${encodeAssetPath('/tmp/a.png')}?s=128`)
  })
})
