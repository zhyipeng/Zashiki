import { describe, expect, it } from 'vitest'
import { archiveFolderName, archiveParent, isArchiveName } from './archive'

describe('压缩包路径', () => {
  it('识别归档格式和复合扩展名', () => {
    for (const name of ['a.ZIP', 'a.7z', 'a.rar', 'a.tar.gz', 'a.tar.xz', 'a.tar.zst', 'a.tgz']) {
      expect(isArchiveName(name)).toBe(true)
    }
    for (const name of ['a.zip.txt', 'a.gz', 'zip', 'report.docx']) expect(isArchiveName(name)).toBe(false)
  })
  it('为解压目录去除完整扩展名', () => {
    expect(archiveFolderName('资料.tar.gz')).toBe('资料')
    expect(archiveFolderName('archive.ZIP')).toBe('archive')
    expect(archiveFolderName('.zip')).toBe('解压文件')
  })
  it('使用包内路径计算上级目录', () => {
    expect(archiveParent('资料/图片')).toBe('资料')
    expect(archiveParent('资料')).toBe('')
    expect(archiveParent('')).toBe('')
  })
})
