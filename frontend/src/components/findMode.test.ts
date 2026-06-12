import { describe, expect, it } from 'vitest'
import { pinyin } from 'pinyin-pro'
import {
  createFindModeTargets,
  exactFindModeLabelMatch,
  findModeLabelForIndex,
  findModeLabelLength,
  findModeMatchedTargets,
  findModeTargetMatches,
  normalizeFindModeText,
} from './findMode'

function pinyinResolver(text: string, pattern: 'pinyin' | 'first') {
  return pinyin(text, {
    toneType: 'none',
    pattern,
    separator: '',
    nonZh: 'consecutive',
  })
}

describe('find mode labels', () => {
  it('uses fixed-length unique labels to avoid prefix collisions', () => {
    const targets = createFindModeTargets([
      { path: '/tmp/a', name: 'a' },
      { path: '/tmp/b', name: 'b' },
      { path: '/tmp/c', name: 'c' },
    ], undefined, 'ab')

    expect(targets.map(target => target.label)).toEqual(['aa', 'ab', 'ba'])
    expect(new Set(targets.map(target => target.label)).size).toBe(3)
    expect(exactFindModeLabelMatch(targets, 'a')).toBeNull()
    expect(exactFindModeLabelMatch(targets, 'ab')?.path).toBe('/tmp/b')
  })

  it('generates labels by index using the provided alphabet', () => {
    expect(findModeLabelLength(5, 2)).toBe(3)
    expect(findModeLabelForIndex(0, 3, 'ab')).toBe('aaa')
    expect(findModeLabelForIndex(4, 3, 'ab')).toBe('baa')
  })
})

describe('find mode matching', () => {
  it('matches label prefixes and lets Enter choose the first matching item', () => {
    const targets = createFindModeTargets([
      { path: '/tmp/one', name: 'one.txt' },
      { path: '/tmp/two', name: 'two.txt' },
      { path: '/tmp/three', name: 'three.txt' },
    ], undefined, 'ab')

    const matched = findModeMatchedTargets(targets, 'a')

    expect(matched.map(target => target.path)).toEqual(['/tmp/one', '/tmp/two'])
  })

  it('matches file names without case or whitespace sensitivity', () => {
    const target = createFindModeTargets([
      { path: '/tmp/readme', name: 'Read Me.md' },
    ])[0]

    expect(normalizeFindModeText('Read Me')).toBe('readme')
    expect(findModeTargetMatches(target, 'readme')).toBe(true)
  })

  it('matches Chinese names by full pinyin and pinyin initials', () => {
    const target = createFindModeTargets([
      { path: '/tmp/project', name: '项目计划' },
    ], pinyinResolver)[0]

    expect(findModeTargetMatches(target, 'xiangmu')).toBe(true)
    expect(findModeTargetMatches(target, 'xmjh')).toBe(true)
  })
})

