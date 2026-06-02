import { describe, expect, it } from 'vitest'
import { ancestorPaths, baseName, joinPath, parentPath, pathRoot } from './path'

describe('path helpers', () => {
  it('handles Windows drive paths', () => {
    expect(pathRoot('C:\\Users\\alice', '\\')).toBe('C:\\')
    expect(baseName('C:\\Users\\alice', '\\')).toBe('alice')
    expect(joinPath('C:\\Users\\alice', 'Desktop', '\\')).toBe('C:\\Users\\alice\\Desktop')
    expect(parentPath('C:\\Users\\alice', '\\')).toBe('C:\\Users')
    expect(parentPath('C:\\Users', '\\')).toBe('C:\\')
    expect(parentPath('C:\\', '\\')).toBeNull()
    expect(ancestorPaths('C:\\Users\\alice', '\\')).toEqual(['C:\\', 'C:\\Users'])
  })

  it('handles Windows UNC paths', () => {
    expect(pathRoot('\\\\server\\share\\dir', '\\')).toBe('\\\\server\\share\\')
    expect(joinPath('\\\\server\\share\\', 'dir', '\\')).toBe('\\\\server\\share\\dir')
    expect(parentPath('\\\\server\\share\\dir', '\\')).toBe('\\\\server\\share\\')
    expect(parentPath('\\\\server\\share\\', '\\')).toBeNull()
  })

  it('handles POSIX paths', () => {
    expect(pathRoot('/home/alice', '/')).toBe('/')
    expect(baseName('/home/alice', '/')).toBe('alice')
    expect(joinPath('/home/alice', 'Desktop', '/')).toBe('/home/alice/Desktop')
    expect(parentPath('/home/alice', '/')).toBe('/home')
    expect(parentPath('/', '/')).toBeNull()
  })
})
