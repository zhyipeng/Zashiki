import { describe, expect, it } from 'vitest'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { fileTypeLabel, isTextFile } from './fileIcons'

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
