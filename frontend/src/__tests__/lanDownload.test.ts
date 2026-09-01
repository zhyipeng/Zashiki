import { describe, expect, it } from 'vitest'

// The receiver page is intentionally served as a plain browser module rather
// than bundled with the Vue application.
// @ts-ignore The JavaScript module has no generated TypeScript declaration.
import {
  DOWNLOAD_MAX_CHUNKED_SIZE,
  basename,
  completedBytes,
  fetchPart,
  isRetryableStatus,
  parseContentRange,
  planRanges,
  shouldUseChunkedDownload,
} from '../../../internal/lanshare/web/download.js'

describe('LAN download range planning', () => {
  it('creates contiguous, non-overlapping ranges', () => {
    const ranges = planRanges(20, 8)
    expect(ranges).toEqual([
      { index: 0, start: 0, end: 7, length: 8 },
      { index: 1, start: 8, end: 15, length: 8 },
      { index: 2, start: 16, end: 19, length: 4 },
    ])
    expect(ranges[ranges.length - 1].end).toBe(19)
  })

  it('returns no ranges for an empty file', () => {
    expect(planRanges(0, 8)).toEqual([])
  })

  it('calculates completed bytes from part indexes', () => {
    const ranges = planRanges(20, 8)
    expect(completedBytes(ranges, [0, 2])).toBe(12)
  })
})

describe('LAN download response validation helpers', () => {
  it('parses a byte content range', () => {
    expect(parseContentRange('bytes 8-15/20')).toEqual({ start: 8, end: 15, total: 20 })
    expect(parseContentRange('bytes 0-7/*')).toEqual({ start: 0, end: 7, total: null })
  })

  it('rejects malformed content ranges', () => {
    expect(parseContentRange('bytes 8-15')).toBeNull()
    expect(parseContentRange('items 8-15/20')).toBeNull()
  })

  it('identifies retryable HTTP responses', () => {
    expect(isRetryableStatus(408)).toBe(true)
    expect(isRetryableStatus(429)).toBe(true)
    expect(isRetryableStatus(503)).toBe(true)
    expect(isRetryableStatus(404)).toBe(false)
  })

  it('retries a transient range failure', async () => {
    const originalFetch = globalThis.fetch
    let calls = 0
    const retryEvents: unknown[] = []
    globalThis.fetch = (async () => {
      calls += 1
      if (calls === 1) {
        return {
          status: 503,
          ok: false,
          headers: new Headers(),
          blob: async () => new Blob(),
        } as Response
      }
      const body = new Blob(['1234'])
      return {
        status: 206,
        ok: true,
        headers: new Headers({
          'Content-Length': '4',
          'Content-Range': 'bytes 0-3/4',
        }),
        blob: async () => body,
      } as Response
    }) as typeof fetch
    try {
      const blob = await fetchPart(
        { id: 'file-1' },
        { size: 4, etag: '' },
        { index: 0, start: 0, end: 3, length: 4 },
        new AbortController().signal,
        { maxRetries: 1 },
        { onRetry: event => retryEvents.push(event) },
      )
      expect(blob.size).toBe(4)
      expect(calls).toBe(2)
      expect(retryEvents).toHaveLength(1)
    } finally {
      globalThis.fetch = originalFetch
    }
  })
})

describe('LAN download fallback policy', () => {
  it('uses chunked mode only when storage is available and size is bounded', () => {
    expect(shouldUseChunkedDownload(1024, { indexedDBAvailable: true })).toBe(true)
    expect(shouldUseChunkedDownload(DOWNLOAD_MAX_CHUNKED_SIZE + 1, { indexedDBAvailable: true })).toBe(false)
    expect(shouldUseChunkedDownload(1024, { indexedDBAvailable: false })).toBe(false)
    expect(shouldUseChunkedDownload(0, { indexedDBAvailable: true })).toBe(false)
  })

  it('uses only the basename for blob downloads', () => {
    expect(basename('目录\\文件.txt')).toBe('文件.txt')
    expect(basename('目录/文件.txt')).toBe('文件.txt')
    expect(basename('')).toBe('download')
  })
})
