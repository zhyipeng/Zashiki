// 首屏分页 + 后台补齐全的纯逻辑辅助，供 FileTable.loadDir 使用。
// 与文件系统/Wails 绑定解耦，便于单元测试。

export interface PageRange {
  offset: number
  limit: number
}

/**
 * 返回下一页的 { offset, limit }；若已经没有更多条目需要加载则返回 null。
 * - loaded < 0 或 total < 0 视为非法输入，返回 null（防御）
 * - pageSize <= 0 视为异常输入，返回 null
 */
export function nextPageRange(
  loaded: number,
  total: number,
  pageSize: number,
): PageRange | null {
  if (loaded < 0 || total < 0 || pageSize <= 0) return null
  if (loaded >= total) return null
  const offset = loaded
  const limit = Math.min(pageSize, total - loaded)
  return { offset, limit }
}

/**
 * 是否还有更多条目需要后台补齐。
 */
export function shouldContinueFetch(loaded: number, total: number): boolean {
  return nextPageRange(loaded, total, 1) !== null
}