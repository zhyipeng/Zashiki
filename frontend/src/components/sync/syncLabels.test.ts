import { describe, expect, it } from 'vitest'
import {
  syncConfigIssue,
  syncConfigIssueMessage,
  syncExecuteConfirmText,
  syncModeLabel,
  syncOpLabel,
  syncPlanSummary,
  syncResultSummary,
} from './syncLabels'
import type { SyncToolSettings } from '../../composables/useSettings'
import type { SyncPlanLike, SyncResultLike } from './syncLabels'

function baseConfig(overrides: Partial<SyncToolSettings> = {}): SyncToolSettings {
  return {
    sourceDir: '/data/source',
    targetDir: '/data/target',
    mode: 'incremental',
    compareSize: true,
    compareModTime: true,
    compareHash: false,
    ignoreHidden: true,
    ignorePatterns: [],
    ...overrides,
  }
}

function basePlan(overrides: Partial<SyncPlanLike> = {}): SyncPlanLike {
  return {
    copy: [],
    delete: [],
    skippedCount: 0,
    errors: [],
    ...overrides,
  }
}

describe('syncConfigIssue', () => {
  it('returns null for a valid config', () => {
    expect(syncConfigIssue(baseConfig())).toBeNull()
  })

  it('detects empty source dir', () => {
    expect(syncConfigIssue(baseConfig({ sourceDir: '' }))).toBe('source-empty')
    expect(syncConfigIssue(baseConfig({ sourceDir: '   ' }))).toBe('source-empty')
  })

  it('detects empty target dir', () => {
    expect(syncConfigIssue(baseConfig({ targetDir: '' }))).toBe('target-empty')
  })

  it('detects identical dirs ignoring separators, case and trailing slash', () => {
    expect(syncConfigIssue(baseConfig({ targetDir: '/data/source' }))).toBe('same-path')
    expect(syncConfigIssue(baseConfig({ targetDir: '/Data/Source/' }))).toBe('same-path')
    expect(syncConfigIssue(baseConfig({ sourceDir: 'C:\\data', targetDir: 'c:/data/' }))).toBe('same-path')
  })

  it('detects missing comparison dimension', () => {
    const overrides = { compareSize: false, compareModTime: false, compareHash: false }
    expect(syncConfigIssue(baseConfig(overrides))).toBe('no-dimension')
  })

  it('keeps hash-only configs valid', () => {
    expect(syncConfigIssue(baseConfig({ compareSize: false, compareModTime: false, compareHash: true }))).toBeNull()
  })
})

describe('syncConfigIssueMessage', () => {
  it('maps issues to guidance text', () => {
    expect(syncConfigIssueMessage('source-empty')).toContain('源目录')
    expect(syncConfigIssueMessage(null)).toBe('')
  })
})

describe('syncModeLabel', () => {
  it('labels known modes', () => {
    expect(syncModeLabel('mirror')).toBe('全量 1:1 同步')
    expect(syncModeLabel('incremental')).toBe('仅增量同步')
  })
})

describe('syncPlanSummary', () => {
  it('counts copy, delete and skipped entries', () => {
    const plan = basePlan({
      copy: [{ relPath: 'a.txt', reason: '新增' }],
      delete: [{ relPath: 'b.txt', isDir: false }],
      skippedCount: 4,
    })
    expect(syncPlanSummary(plan)).toBe('复制 1 · 删除 1 · 跳过 4')
  })
})

describe('syncOpLabel', () => {
  it('maps backend ops and passes through unknown ones', () => {
    expect(syncOpLabel('copy')).toBe('复制')
    expect(syncOpLabel('unknown')).toBe('unknown')
  })
})

describe('syncResultSummary', () => {
  it('lists only nonzero counters', () => {
    const result: SyncResultLike = { status: 'done', copied: 3, deleted: 0, removedDirs: 0, skippedCount: 0, errors: [] }
    expect(syncResultSummary(result)).toBe('已复制 3 项')
  })

  it('includes deletes, cleanup and errors when present', () => {
    const result: SyncResultLike = {
      status: 'done',
      copied: 2,
      deleted: 1,
      removedDirs: 1,
      skippedCount: 5,
      errors: [{ relPath: 'x', op: 'copy', error: 'boom' }],
    }
    expect(syncResultSummary(result)).toBe('已复制 2 项 · 已删除 1 项 · 清理空目录 1 个 · 跳过 5 项 · 失败 1 项')
  })
})

describe('syncExecuteConfirmText', () => {
  it('warns about trash deletions in mirror mode', () => {
    const plan = basePlan({
      copy: [{ relPath: 'a.txt', reason: '新增' }],
      delete: [{ relPath: 'b.txt', isDir: false }],
    })
    expect(syncExecuteConfirmText(plan)).toContain('移入回收站')
  })

  it('mentions copy count without deletes', () => {
    const plan = basePlan({ copy: [{ relPath: 'a.txt', reason: '新增' }] })
    expect(syncExecuteConfirmText(plan)).toContain('复制 1 项')
    expect(syncExecuteConfirmText(plan)).not.toContain('回收站')
  })

  it('handles empty plans', () => {
    expect(syncExecuteConfirmText(basePlan())).toContain('没有需要同步的内容')
  })
})
