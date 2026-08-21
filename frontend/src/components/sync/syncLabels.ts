import type { SyncToolSettings } from '../../composables/useSettings'

export type SyncMode = 'mirror' | 'incremental'

export interface SyncPlanLike {
  copy: { relPath: string, reason: string }[]
  delete: { relPath: string, isDir: boolean }[]
  skippedCount: number
  errors: { relPath: string, op: string, error: string }[]
}

export interface SyncResultLike {
  status: string
  copied: number
  deleted: number
  removedDirs: number
  skippedCount: number
  errors: { relPath: string, op: string, error: string }[]
}

export type SyncConfigIssue =
  | 'source-empty'
  | 'target-empty'
  | 'same-path'
  | 'no-dimension'

/** Validate sync tool form values; returns the first blocking issue or null. */
export function syncConfigIssue(config: SyncToolSettings): SyncConfigIssue | null {
  if (!config.sourceDir.trim()) return 'source-empty'
  if (!config.targetDir.trim()) return 'target-empty'
  if (normalizeDirPath(config.sourceDir) === normalizeDirPath(config.targetDir)) return 'same-path'
  if (!config.compareSize && !config.compareModTime && !config.compareHash) return 'no-dimension'
  return null
}

function normalizeDirPath(path: string): string {
  const trimmed = path.trim().replace(/\\/g, '/').replace(/\/+$/, '')
  return trimmed.toLowerCase()
}

const issueMessages: Record<SyncConfigIssue, string> = {
  'source-empty': '请选择源目录',
  'target-empty': '请选择同步目录',
  'same-path': '源目录与同步目录不能相同',
  'no-dimension': '请至少选择一个判重维度',
}

export function syncConfigIssueMessage(issue: SyncConfigIssue | null): string {
  return issue ? issueMessages[issue] : ''
}

const syncModeLabels: Record<SyncMode, string> = {
  mirror: '全量 1:1 同步',
  incremental: '仅增量同步',
}

export function syncModeLabel(mode: SyncMode): string {
  return syncModeLabels[mode] ?? mode
}

/** Summary line of an analysis result, like "复制 3 · 删除 1 · 跳过 5". */
export function syncPlanSummary(plan: SyncPlanLike): string {
  return `复制 ${plan.copy.length} · 删除 ${plan.delete.length} · 跳过 ${plan.skippedCount}`
}

const syncOpLabels: Record<string, string> = {
  analyze: '分析',
  copy: '复制',
  delete: '删除',
  cleanup: '清理',
}

export function syncOpLabel(op: string): string {
  return syncOpLabels[op] ?? op
}

/** Result summary line, e.g. "已复制 3 项 · 已删除 1 项 · 跳过 5 项". */
export function syncResultSummary(result: SyncResultLike): string {
  const parts = [`已复制 ${result.copied} 项`]
  if (result.deleted > 0) parts.push(`已删除 ${result.deleted} 项`)
  if (result.removedDirs > 0) parts.push(`清理空目录 ${result.removedDirs} 个`)
  if (result.skippedCount > 0) parts.push(`跳过 ${result.skippedCount} 项`)
  if (result.errors.length > 0) parts.push(`失败 ${result.errors.length} 项`)
  return parts.join(' · ')
}

/** Confirm wording before execute; mirror deletions deserve an explicit hint. */
export function syncExecuteConfirmText(plan: SyncPlanLike): string {
  const copyCount = plan.copy.length
  if (plan.delete.length > 0) {
    return `即将复制 ${copyCount} 项，并将 ${plan.delete.length} 项多余内容移入回收站，确定开始同步？`
  }
  return copyCount > 0
    ? `即将复制 ${copyCount} 项到同步目录，确定开始同步？`
    : '没有需要同步的内容，仍要执行一次吗？'
}
