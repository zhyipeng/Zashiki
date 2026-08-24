// clipboardState 提供系统剪贴板 UI 镜像的纯逻辑：
//  - 判断剪贴板是否被外部程序改写（序列号变化）
//  - 维护「剪切态」半透明集合
//  - 从服务端 ClipboardContent 同步本地状态
//
// 设计：本地 UI 只保存 (paths, mode, seq) 镜像；系统剪贴板是唯一事实来源。
// 当 ClipboardSequence() 返回的序号与本地记录不一致时，说明剪贴板已被外部
// 改写，应清空剪切态（否则 UI 会一直显示不存在的剪切状态）。

export type ClipboardMode = 'copy' | 'cut'

export interface ClipboardSnapshot {
  paths: string[]
  mode: ClipboardMode
  seq: number
}

export interface ClipboardSyncResult {
  snapshot: ClipboardSnapshot | null
  changed: boolean
}

/** 外部改写判定：seq 从 0 变为非 0 表示「我们写入过但被替换」；从非 0 变为另一个非 0 也表示改写。 */
export function isClipboardRewritten(prev: ClipboardSnapshot | null, nextSeq: number): boolean {
  if (!prev) return false
  if (nextSeq === prev.seq) return false
  // 系统剪贴板 seq 只在内容变化时递增；若 seq 不变说明还是我们的内容
  return true
}

/** 生成新快照（用于写入成功后记录）。 */
export function snapshotFor(paths: string[], mode: ClipboardMode, seq: number): ClipboardSnapshot {
  return { paths: [...paths], mode, seq }
}

/** 本地镜像与系统剪贴板同步：seq 一致则保留，不一致则清空（外部改写）。 */
export function syncSnapshot(prev: ClipboardSnapshot | null, seq: number): ClipboardSyncResult {
  if (prev && prev.seq === seq) {
    return { snapshot: prev, changed: false }
  }
  // 无法确认属于我们 → 视为外部内容
  return { snapshot: null, changed: true }
}

/** 剪切态路径集合（用于表格半透明显示）。 */
export function cutPathSetOf(snapshot: ClipboardSnapshot | null): Set<string> {
  if (!snapshot || snapshot.mode !== 'cut') return new Set<string>()
  return new Set(snapshot.paths)
}

/** 是否有可粘贴内容。 */
export function hasPasteableContent(snapshot: ClipboardSnapshot | null): boolean {
  return !!snapshot && snapshot.paths.length > 0
}
