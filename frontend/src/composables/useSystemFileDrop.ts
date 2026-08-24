import { computed, onUnmounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import type { CancellablePromise } from '@wailsio/runtime'
import type { EntryOperationResult } from '../../bindings/zashiki/internal/filemanager'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import { notifyDirectoriesChanged } from './useDirectoryEvents'
import { isOperationCancelledError, trackOperationPromise } from './useOperationProgress'

// useSystemFileDrop 处理「系统文件管理器 → Wails 拖入」。
//
// 设计要点（修复多 panel 同时复制）：
//   - 整个应用只有「一个」全局 Wails 事件监听器（模块级，首次调用时绑定）。
//   - 每个 FileTable 面板通过 useSystemFileDrop 注册自己的 (currentDir, onOperationComplete)，
//     并分配唯一 panelId（模块计数器）。
//   - 事件到达后用落点坐标 (x, y) + elementFromPoint 找最近 [data-panel-id]：
//       命中某面板 → 该面板弹确认框；
//       未命中（如落在非表格区域）→ 回退到当前激活面板。
//   - 侧边栏（树节点/快速访问，属性带 data-drop-dir）由 Sidebar 注册的 handler 处理，
//     同样进入确认框。
//   - 确认框（DropConfirmModal）决定 复制/移动 + 冲突策略，与内部拖拽一致。
//
// 系统文件拖入不再依赖 HTML5 dataTransfer.files[].path（Wails 下不可靠）。

export interface SystemDropEvent {
  files: string[]
  details?: {
    x: number
    y: number
    id?: string
    classList?: string[]
    attributes?: Record<string, string>
  } | null
}

/** 一次待确认的系统拖入（由确认框消费）。 */
export interface PendingSystemDrop {
  paths: string[]
  targetDir: string
  panelId: number
}

// ---- 模块级共享状态（整个应用只有一份） ----

const pendingSystemDrop = ref<PendingSystemDrop | null>(null)
const panels = new Map<number, { currentDir: () => string, onOperationComplete?: (action: 'move' | 'copy', results: EntryOperationResult[]) => void }>()
let activePanelId = 0
let nextPanelId = 1
let sidebarHandler: ((event: SystemDropEvent) => void) | null = null
let bound = false

/** 当前激活面板 id（由 FileTable 焦点变化时设置）。 */
export function setActivePanelId(id: number) {
  activePanelId = id
}

/** 当前激活面板 id。 */
export function getActivePanelId(): number {
  return activePanelId
}

/** 判断落点是否在侧边栏（树节点或快速访问，属性带 data-drop-dir）。 */
export function hasSidebarDropTarget(details: SystemDropEvent['details']): boolean {
  const attrs = details?.attributes
  if (!attrs) return false
  return !!attrs['data-drop-dir']
}

/** 从落点 attributes 解析侧边栏目标目录（data-drop-dir；__quick_access__ 需坐标定位）。 */
export function sidebarTargetDir(details: SystemDropEvent['details']): string {
  const attrs = details?.attributes
  if (!attrs) return ''
  const dir = attrs['data-drop-dir']
  if (!dir || dir === '__quick_access__') return ''
  return dir
}

/** 从落点坐标解析目标 panel id：找落点元素最近的 [data-panel-id]。返回 0 表示无法识别。 */
export function panelIdFromPoint(x: number, y: number): number {
  if (typeof document === 'undefined' || typeof document.elementFromPoint !== 'function') return 0
  const el = document.elementFromPoint(x, y)
  const panel = el?.closest('[data-panel-id]') as HTMLElement | null
  if (!panel?.dataset.panelId) return 0
  return Number(panel.dataset.panelId) || 0
}

// ---- 全局事件绑定（只绑定一次） ----

function bindGlobal() {
  if (bound) return
  bound = true
  Events.On('window:files-dropped', (payload: { data: SystemDropEvent }) => {
    handleSystemDrop(payload.data)
  })
}

function handleSystemDrop(event: SystemDropEvent) {
  const files = event?.files || []
  if (files.length === 0) return

  // 侧边栏落点（树节点/快速访问）由 Sidebar 注册的 handler 处理
  if (hasSidebarDropTarget(event.details)) {
    sidebarHandler?.(event)
    return
  }

  // 面板落点：坐标解析出目标 panel；解析不到则回退当前激活面板
  const targetPanelId = resolveTargetPanel(event.details)
  const dir = panels.get(targetPanelId)?.currentDir()
  if (!dir) return
  if (pendingSystemDrop.value) return // 已有待确认拖入，忽略新事件
  pendingSystemDrop.value = { paths: files, targetDir: dir, panelId: targetPanelId }
}

/** 从事件 details 解析目标 panel：优先坐标命中，回退当前激活面板。可注入坐标解析便于测试。 */
export function resolveTargetPanel(
  details: SystemDropEvent['details'],
  pointToPanel = panelIdFromPoint,
  active = getActivePanelId(),
): number {
  const x = details?.x
  const y = details?.y
  if (typeof x === 'number' && typeof y === 'number') {
    const hit = pointToPanel(x, y)
    if (hit) return hit
  }
  return active
}

/**
 * 每个 FileTable 面板调用：注册该面板，返回确认框所需的响应式状态与回调。
 */
export function useSystemFileDrop(options: {
  currentDir: () => string
  onOperationComplete?: (action: 'move' | 'copy', results: EntryOperationResult[]) => void
}) {
  const panelId = nextPanelId++
  panels.set(panelId, {
    currentDir: options.currentDir,
    onOperationComplete: options.onOperationComplete,
  })
  bindGlobal()

  onUnmounted(() => {
    panels.delete(panelId)
    if (activePanelId === panelId) activePanelId = 0
  })

  const showConfirm = computed(() => pendingSystemDrop.value?.panelId === panelId)
  const drop = computed(() => (showConfirm.value ? pendingSystemDrop.value : null))

  function onConfirm(action: 'move' | 'copy', conflict: 'overwrite' | 'skip' | 'rename') {
    void confirmSystemDrop(action, conflict, panels.get(panelId)?.onOperationComplete)
  }

  function onCancel() {
    if (pendingSystemDrop.value?.panelId === panelId) {
      pendingSystemDrop.value = null
    }
  }

  /** 焦点进入本面板时调用，标记为激活面板。 */
  function activate() {
    setActivePanelId(panelId)
  }

  return {
    panelId,
    showConfirm,
    drop,
    onConfirm,
    onCancel,
    activate,
  }
}

// ---- 执行（确认框确认后） ----

export async function confirmSystemDrop(
  action: 'move' | 'copy',
  conflict: string,
  onOperationComplete?: (action: 'move' | 'copy', results: EntryOperationResult[]) => void,
): Promise<void> {
  const drop = pendingSystemDrop.value
  if (!drop) return
  pendingSystemDrop.value = null
  const { paths, targetDir } = drop
  try {
    let results: EntryOperationResult[]
    if (action === 'move') {
      results = await trackOperationPromise(
        FileService.MoveEntries(paths, targetDir, conflict) as CancellablePromise<EntryOperationResult[]>,
      )
    } else {
      results = await trackOperationPromise(
        FileService.CopyEntries(paths, targetDir, conflict) as CancellablePromise<EntryOperationResult[]>,
      )
    }
    onOperationComplete?.(action, results)
    notifyDirectoriesChanged([targetDir, ...sourceDirsForPaths(paths)])
  } catch (err) {
    if (isOperationCancelledError(err)) {
      notifyDirectoriesChanged([targetDir, ...sourceDirsForPaths(paths)])
      return
    }
    throw err
  }
}

// ---- 执行（确认框确认后） ----

/** 侧边栏注册系统拖入 handler（Sidebar 组件调用一次）。 */
export function setSidebarSystemDropHandler(handler: ((event: SystemDropEvent) => void) | null) {
  sidebarHandler = handler
}

/**
 * 侧边栏拖入时打开确认框：以当前激活面板的确认框呈现，目标为侧边栏目录。
 */
export function openSystemDropConfirm(paths: string[], targetDir: string) {
  if (paths.length === 0 || !targetDir) return
  if (pendingSystemDrop.value) return
  const panelId = activePanelId
  if (!panels.has(panelId)) return // 无激活面板则忽略
  pendingSystemDrop.value = { paths, targetDir, panelId }
}

function sourceDirsForPaths(paths: string[]): string[] {
  const dirs = new Set<string>()
  for (const p of paths) {
    const idx = Math.max(p.lastIndexOf('/'), p.lastIndexOf('\\'))
    if (idx > 0) dirs.add(p.slice(0, idx))
  }
  return Array.from(dirs)
}
