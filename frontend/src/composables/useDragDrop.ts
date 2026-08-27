import { ref, computed, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import type { EntryOperationResult, FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import { notifyDirectoriesChanged } from './useDirectoryEvents'
import { isOperationCancelledError, trackOperationPromise } from './useOperationProgress'

export const FILE_EXPLORER_DRAG_MIME = 'application/x-file-explorer-paths'

interface DragPayload {
  paths: string[]
  sourcePanel: string
  kind: 'html5' | 'native'
  sessionId: number
}

interface PendingDrop {
  paths: string[]
  sourcePanel: string
  targetDir: string
}

interface DragDropOptions {
  onOperationComplete?: (action: 'move' | 'copy', results: EntryOperationResult[]) => void
}

// Shared across all panels
const dragPayload = ref<DragPayload | null>(null)
const hoveredFolderPath = ref('')
const pendingDrop = ref<PendingDrop | null>(null)
const dragOverRefs = new Set<{ value: boolean }>()
let dragCleanupTimer: number | null = null
let nextDragSessionId = 1

function dragCleanup() {
  if (dragCleanupTimer !== null) {
    window.clearTimeout(dragCleanupTimer)
    dragCleanupTimer = null
  }
  dragPayload.value = null
  hoveredFolderPath.value = ''
  for (const r of dragOverRefs) {
    r.value = false
  }
}

export function clearDrag(sessionId?: number) {
  if (!dragPayload.value) return
  if (sessionId !== undefined && dragPayload.value.sessionId !== sessionId) return
  scheduleDragCleanup(dragPayload.value.sessionId)
}

export function finishDragDrop() {
  dragCleanup()
}

export function activeDragPaths(): string[] {
  return dragPayload.value?.paths || []
}

export function hasActiveDragPayload(): boolean {
  return !!dragPayload.value
}

export function hasActiveNativeDragPayload(): boolean {
  return dragPayload.value?.kind === 'native'
}

export function activeDragSourcePanel(): string {
  return dragPayload.value?.sourcePanel || ''
}

export function startFileExplorerDrag(e: DragEvent, paths: string[], sourcePanel = ''): number {
  if (paths.length === 0 || !e.dataTransfer) return 0
  cancelScheduledDragCleanup()
  const sessionId = nextDragSessionId++
  const payload: DragPayload = {
    paths,
    sourcePanel,
    kind: 'html5',
    sessionId,
  }
  dragPayload.value = payload
  hoveredFolderPath.value = ''
  const serialized = JSON.stringify(payload)
  e.dataTransfer.setData(FILE_EXPLORER_DRAG_MIME, serialized)
  e.dataTransfer.setData('text/plain', serialized)
  e.dataTransfer.effectAllowed = 'all'
  return sessionId
}

export function startNativeFileExplorerDrag(paths: string[], sourcePanel = ''): number {
  if (paths.length === 0) return 0
  cancelScheduledDragCleanup()
  const sessionId = nextDragSessionId++
  dragPayload.value = {
    paths,
    sourcePanel,
    kind: 'native',
    sessionId,
  }
  hoveredFolderPath.value = ''
  return sessionId
}

function cancelScheduledDragCleanup() {
  if (dragCleanupTimer !== null) {
    window.clearTimeout(dragCleanupTimer)
    dragCleanupTimer = null
  }
}

function scheduleDragCleanup(sessionId: number) {
  cancelScheduledDragCleanup()
  dragCleanupTimer = window.setTimeout(() => {
    dragCleanupTimer = null
    if (dragPayload.value?.sessionId !== sessionId) return
    dragCleanup()
  }, 800)
}

export function useDragDrop(currentPath: () => string, options: DragDropOptions = {}) {
  const message = useMessage()
  const isDragOver = ref(false)
  dragOverRefs.add(isDragOver)
  onUnmounted(() => {
    dragOverRefs.delete(isDragOver)
  })

  // ---- drag source ----

  let rowDragSessionId = 0

  function onRowDragStart(e: DragEvent, entry: FileEntry, paths?: string[]) {
    rowDragSessionId = startFileExplorerDrag(e, paths && paths.length > 0 ? paths : [entry.path], currentPath())
  }

  function onRowDragEnd() {
    clearDrag(rowDragSessionId)
    rowDragSessionId = 0
  }

  // ---- helpers ----

  function isValidDrop(e: DragEvent): boolean {
    // Native app drags are delivered back through Wails' WindowFilesDropped
    // event. Letting this HTML5 handler process them as well would open two
    // confirmation flows on Windows/WebView2.
    if (hasActiveNativeDragPayload()) return false
    return !!(dragPayload.value) || !!e.dataTransfer?.types.includes(FILE_EXPLORER_DRAG_MIME) || !!e.dataTransfer?.types.includes('Files')
  }

  function targetFromEvent(e: DragEvent): string {
    const el = document.elementFromPoint(e.clientX, e.clientY)
    const row = el?.closest('[data-folder-path]') as HTMLElement | null
    return row?.dataset.folderPath || currentPath()
  }

  // ---- drop target (table-area) ----

  function _onDragOver(e: DragEvent) {
    if (!isValidDrop(e)) return
    e.preventDefault()
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'copy'
    }
    const fp = targetFromEvent(e)
    hoveredFolderPath.value = fp !== currentPath() ? fp : ''
  }

  function _onDragEnter(e: DragEvent) {
    if (!isValidDrop(e)) return
    e.preventDefault()
    isDragOver.value = true
  }

  function _onDragLeave(e: DragEvent) {
    if ((e.currentTarget as HTMLElement)?.contains(e.relatedTarget as HTMLElement)) return
    isDragOver.value = false
    hoveredFolderPath.value = ''
  }

  // ---- drop ----

  async function _onDrop(e: DragEvent) {
    if (!isValidDrop(e)) return
    isDragOver.value = false
    hoveredFolderPath.value = ''
    await prepareDrop(targetFromEvent(e), e)
  }

  async function prepareDrop(targetDir: string, e: DragEvent) {
    const payload = dragPayload.value
    let paths: string[] = []

    if (payload) {
      if (payload.sourcePanel === targetDir) return
      paths = payload.paths
    } else {
      paths = getExternalPaths(e)
    }
    if (paths.length === 0) return

    pendingDrop.value = {
      paths,
      sourcePanel: payload?.sourcePanel || '',
      targetDir,
    }
  }

  async function confirmDrop(action: 'move' | 'copy', conflict: string) {
    const drop = pendingDrop.value
    if (!drop) return
    pendingDrop.value = null
    dragPayload.value = null

    try {
      let results: EntryOperationResult[]
      if (action === 'move') {
        results = await trackOperationPromise(FileService.MoveEntries(drop.paths, drop.targetDir, conflict))
      } else {
        results = await trackOperationPromise(FileService.CopyEntries(drop.paths, drop.targetDir, conflict))
      }
      options.onOperationComplete?.(action, results)
      notifyDirectoriesChanged([drop.sourcePanel, drop.targetDir])
    } catch (err) {
      if (isOperationCancelledError(err)) {
        notifyDirectoriesChanged([drop.sourcePanel, drop.targetDir])
        return
      }
      console.error('Drop operation failed:', err)
      message.error(friendlyDropError(err))
    }
  }

  function cancelDrop() {
    pendingDrop.value = null
    dragPayload.value = null
    isDragOver.value = false
    hoveredFolderPath.value = ''
  }

  // ---- label ----

  const dragLabel = computed(() => {
    const p = dragPayload.value
    if (!p) return ''
    return `已选择 ${p.paths.length} 项`
  })

  function getExternalPaths(e: DragEvent): string[] {
    const paths: string[] = []
    if (e.dataTransfer!.files.length > 0) {
      for (let i = 0; i < e.dataTransfer!.files.length; i++) {
        const f = e.dataTransfer!.files[i] as any
        if (f.path) paths.push(f.path)
      }
    }
    return paths
  }

  function friendlyDropError(err: unknown): string {
    const text = errorText(err)
    const lower = text.toLowerCase()
    if (lower.includes('into itself') || lower.includes('subdirectory')) {
      return '不能复制或移动到自身或子目录'
    }
    if (lower.includes('permission denied') || lower.includes('access denied') || lower.includes('operation not permitted')) {
      return '权限不足，操作失败'
    }
    if (lower.includes('no such file') || lower.includes('not found') || lower.includes('does not exist')) {
      return '源文件或目标目录不存在'
    }
    return `操作失败：${text}`
  }

  function errorText(err: unknown): string {
    if (err instanceof Error) return err.message
    if (typeof err === 'string') return err
    return String(err)
  }

  return {
    isDragOver,
    dragLabel,
    hoveredFolderPath,
    pendingDrop,
    onRowDragStart,
    onRowDragEnd,
    confirmDrop,
    cancelDrop,
    onDragOver: _onDragOver,
    onDragEnter: _onDragEnter,
    onDragLeave: _onDragLeave,
    onDrop: _onDrop,
  }
}
