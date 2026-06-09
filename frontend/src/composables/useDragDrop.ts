import { ref, computed, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { FileService } from '../../bindings/zashiki/internal/filemanager'

const DRAG_MIME = 'application/x-file-explorer-paths'

interface DragPayload {
  paths: string[]
  sourcePanel: string
}

interface PendingDrop {
  paths: string[]
  sourcePanel: string
  targetDir: string
}

// Shared across all panels
const dragPayload = ref<DragPayload | null>(null)
const hoveredFolderPath = ref('')
const pendingDrop = ref<PendingDrop | null>(null)
const refreshCallbacks = new Set<() => void>()
const dragOverRefs = new Set<{ value: boolean }>()

function dragCleanup() {
  dragPayload.value = null
  hoveredFolderPath.value = ''
  for (const r of dragOverRefs) {
    r.value = false
  }
}

export function clearDrag() {
  dragCleanup()
}

export function useDragDrop(currentPath: () => string, onChanged: () => void) {
  const message = useMessage()
  refreshCallbacks.add(onChanged)
  const isDragOver = ref(false)
  dragOverRefs.add(isDragOver)
  onUnmounted(() => {
    refreshCallbacks.delete(onChanged)
    dragOverRefs.delete(isDragOver)
  })

  // ---- drag source ----

  function onRowDragStart(e: DragEvent, entry: FileEntry, paths?: string[]) {
    const payload: DragPayload = {
      paths: paths && paths.length > 0 ? paths : [entry.path],
      sourcePanel: currentPath(),
    }
    dragPayload.value = payload
    hoveredFolderPath.value = ''
    e.dataTransfer!.setData(DRAG_MIME, JSON.stringify(payload))
    e.dataTransfer!.effectAllowed = 'all'
  }

  // ---- helpers ----

  function isValidDrop(e: DragEvent): boolean {
    return !!(dragPayload.value) || e.dataTransfer!.types.includes('Files')
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
      if (action === 'move') {
        await FileService.MoveEntries(drop.paths, drop.targetDir, conflict)
      } else {
        await FileService.CopyEntries(drop.paths, drop.targetDir, conflict)
      }
      for (const cb of refreshCallbacks) {
        cb()
      }
    } catch (err) {
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
    confirmDrop,
    cancelDrop,
    onDragOver: _onDragOver,
    onDragEnter: _onDragEnter,
    onDragLeave: _onDragLeave,
    onDrop: _onDrop,
  }
}
