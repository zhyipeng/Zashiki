import { describe, expect, it } from 'vitest'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { fileTypeLabel, isTextFile, resolveFileIcon } from './fileIcons'

function entry(overrides: Partial<FileEntry>): FileEntry {
  return {
    name: 'file',
    path: '/tmp/file',
    size: 0,
    modTime: '',
    isDir: false,
    isHidden: false,
    isSymlink: false,
    linkTarget: '',
    isExecutable: false,
    ...overrides,
  }
}

describe('fileTypeLabel', () => {
  it('labels directories', () => {
    expect(fileTypeLabel(entry({ isDir: true }))).toBe('目录')
  })

  it('labels known file groups', () => {
    expect(fileTypeLabel(entry({ name: 'photo.png' }))).toBe('图片')
    expect(fileTypeLabel(entry({ name: 'README.md' }))).toBe('文本')
  })

  it('labels audio and video files', () => {
    expect(fileTypeLabel(entry({ name: 'song.mp3' }))).toBe('音频')
    expect(fileTypeLabel(entry({ name: '无损音乐.FLAC' }))).toBe('音频')
    expect(fileTypeLabel(entry({ name: 'movie.mkv' }))).toBe('视频')
    expect(fileTypeLabel(entry({ name: 'clip.webm' }))).toBe('视频')
  })

  it('labels generic extension files', () => {
    expect(fileTypeLabel(entry({ name: 'archive.zip' }))).toBe('ZIP')
  })

  it('labels extensionless files', () => {
    expect(fileTypeLabel(entry({ name: 'LICENSE' }))).toBe('文件')
  })
})

describe('isTextFile', () => {
  it('matches known text extensions only for files', () => {
    expect(isTextFile(entry({ name: 'README.md' }))).toBe(true)
    expect(isTextFile(entry({ name: 'archive.zip' }))).toBe(false)
    expect(isTextFile(entry({ name: 'notes.md', isDir: true }))).toBe(false)
  })
})

describe('resolveFileIcon', () => {
  it('resolves audio files by extension regardless of case', () => {
    const icon = resolveFileIcon(entry({ name: 'song.mp3' }))
    expect(icon.source).toBe('audio')
    expect(icon.label).toBe('音频')
    expect(icon.icon).toBeDefined()
    expect(resolveFileIcon(entry({ name: 'MUSIC.FLAC' })).source).toBe('audio')
  })

  it('resolves video files by extension regardless of case', () => {
    const icon = resolveFileIcon(entry({ name: 'movie.mkv' }))
    expect(icon.source).toBe('video')
    expect(icon.label).toBe('视频')
    expect(icon.icon).toBeDefined()
    expect(resolveFileIcon(entry({ name: 'CLIP.MP4' })).source).toBe('video')
  })

  it('treats .ts as TypeScript text, not MPEG transport stream video', () => {
    expect(resolveFileIcon(entry({ name: 'main.ts' })).source).toBe('text')
  })

  it('does not treat audio or video entries as images', () => {
    expect(resolveFileIcon(entry({ name: 'song.mp3' })).source).not.toBe('image')
    expect(resolveFileIcon(entry({ name: 'movie.mkv' })).source).not.toBe('image')
  })

  it('keeps unknown extensions on the binary fallback', () => {
    expect(resolveFileIcon(entry({ name: 'archive.zip' })).source).toBe('binary')
  })
})
