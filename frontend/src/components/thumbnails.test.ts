import { describe, expect, it } from 'vitest'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { isImageEntry } from './thumbnails'

function entry(overrides: Partial<FileEntry>): FileEntry {
  return {
    name: 'file',
    path: '/tmp/file',
    size: 1,
    modTime: '2024-01-01T00:00:00Z',
    isDir: false,
    isHidden: false,
    isSymlink: false,
    linkTarget: '',
    isExecutable: false,
    ...overrides,
  }
}

describe('isImageEntry', () => {
  it('marks image extensions as images', () => {
    expect(isImageEntry(entry({ name: 'photo.jpg', path: '/tmp/photo.jpg' }))).toBe(true)
    expect(isImageEntry(entry({ name: 'photo.PNG', path: '/tmp/photo.PNG' }))).toBe(true)
    expect(isImageEntry(entry({ name: 'icon.svg', path: '/tmp/icon.svg' }))).toBe(true)
  })

  it('rejects non-image files', () => {
    expect(isImageEntry(entry({ name: 'notes.txt', path: '/tmp/notes.txt' }))).toBe(false)
    expect(isImageEntry(entry({ name: 'app.exe', path: '/tmp/app.exe', isExecutable: true }))).toBe(false)
    expect(isImageEntry(entry({ name: 'noext', path: '/tmp/noext' }))).toBe(false)
  })

  it('rejects directories and symlinks even with image names', () => {
    expect(isImageEntry(entry({ name: 'folder.jpg', path: '/tmp/folder.jpg', isDir: true }))).toBe(false)
    expect(isImageEntry(entry({ name: 'link.png', path: '/tmp/link.png', isSymlink: true, linkTarget: '/tmp/real.png' }))).toBe(false)
  })
})
