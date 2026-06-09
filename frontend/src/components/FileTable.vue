<script lang="ts">
let nextFileTableShortcutScopeId = 1
let activeFileTableShortcutScopeId = 0
</script>

<script setup lang="ts">
import { ref, watch, computed, h, nextTick } from 'vue'
import { NDataTable, NButton, NText, NSpin, NIcon, NEmpty, NAlert, NInput, NDropdown, NModal, NSpace, useMessage } from 'naive-ui'
import type { DataTableColumns, DataTableInst, DropdownOption } from 'naive-ui'
import { Clipboard } from '@wailsio/runtime'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { CloseSharp, ArrowBackRound, ArrowForwardRound, RefreshSharp, ChecklistOutlined, SearchOutlined } from '@vicons/material'
import { SplitVertical28Regular, SplitHorizontal28Regular, FolderArrowUp24Regular, Home28Regular } from '@vicons/fluent'
import { useSettings } from '../composables/useSettings'
import { useDragDrop, clearDrag } from '../composables/useDragDrop'
import { useFileClipboard } from '../composables/useFileClipboard'
import { formatShortcutBinding, useKeyboardShortcuts } from '../composables/useKeyboardShortcuts'
import type { ShortcutAction } from '../composables/useKeyboardShortcuts'
import DropConfirmModal from './DropConfirmModal.vue'
import { fileTypeLabel, resolveFileIcon } from './fileIcons'
import { parentPath as getParentPath } from './path'

const { settings } = useSettings()
const message = useMessage()
const parentEntryPathPrefix = '__zashiki_parent__:'
const shortcutScopeId = nextFileTableShortcutScopeId++
if (activeFileTableShortcutScopeId === 0) {
  activeFileTableShortcutScopeId = shortcutScopeId
}
const fileTableRef = ref<HTMLElement | null>(null)
const dataTableRef = ref<DataTableInst | null>(null)
const fileClipboard = useFileClipboard()
const cutPathSet = computed(() => {
  const clipboard = fileClipboard.clipboard.value
  if (!clipboard || clipboard.mode !== 'cut') return new Set<string>()
  return new Set(clipboard.paths)
})
const visibleEntries = computed(() => {
  const filteredEntries = settings.showHiddenFiles
    ? entries.value
    : entries.value.filter((e: FileEntry) => !e.isHidden)
  const matchedEntries = filterEntriesBySearch(filteredEntries)
  const parent = parentEntry.value
  return parent ? [parent, ...matchedEntries] : matchedEntries
})

const props = defineProps<{
  path: string
  closable?: boolean
  separator: string
  homeDir: string
}>()

function loadDir(p: string) {
  loading.value = true
  errorMsg.value = ''
  pathError.value = false
  entries.value = []
  FileService.ListDir(p).then((result) => {
    console.log('ListDir', p, '→', result?.length, 'entries')
    entries.value = result || []
  }).catch((err) => {
    console.error('ListDir failed:', p, err)
    errorMsg.value = friendlyError(err, p)
    pathError.value = true
  }).finally(() => {
    loading.value = false
  })
}

function refresh() {
  if (props.path) {
    loadDir(props.path)
  }
}

const {
  isDragOver, dragLabel, hoveredFolderPath,
  pendingDrop, confirmDrop, cancelDrop,
  onRowDragStart,
  onDragOver, onDragEnter, onDragLeave, onDrop,
} = useDragDrop(() => props.path, refresh)

const showConfirm = ref(false)
watch(pendingDrop, (val) => { showConfirm.value = !!val })

function onConfirm(action: 'move' | 'copy', conflict: 'overwrite' | 'skip' | 'rename') {
  showConfirm.value = false
  confirmDrop(action, conflict)
}

function onCancel() {
  showConfirm.value = false
  cancelDrop()
}

const emit = defineEmits<{
  navigate: [path: string]
  splitH: []
  splitV: []
  close: []
}>()

const entries = ref<FileEntry[]>([])
const loading = ref(false)
const errorMsg = ref('')
const pathError = ref(false)
const multiSelectMode = ref(false)
const selectedRowKeys = ref<string[]>([])
const currentRowKey = ref('')
const selectedPathSet = computed(() => new Set(selectedRowKeys.value))
type ContextTarget = { kind: 'blank', dir: string } | { kind: 'entry', entry: FileEntry }
type ContextActionKey = 'new-folder' | 'open-terminal' | 'paste' | 'refresh' | 'open' | 'copy-path' | 'copy' | 'cut' | 'delete'

interface ContextMenuAction {
  key: ContextActionKey
  label: string
  targets: ContextTarget['kind'][]
  disabled?: (target: ContextTarget) => boolean
  run: (target: ContextTarget) => Promise<void> | void
}

const contextMenu = ref({
  show: false,
  x: 0,
  y: 0,
  target: null as ContextTarget | null,
})
const contextActivePath = computed(() => {
  const target = contextMenu.value.target
  if (!contextMenu.value.show || multiSelectMode.value || target?.kind !== 'entry') return ''
  return target.entry.path
})
const createFolderModal = ref({
  show: false,
  dir: '',
  name: '新建文件夹',
})
const deleteConfirmModal = ref({
  show: false,
  entries: [] as FileEntry[],
  fallbackIndex: -1,
})
const shortcutHelpModal = ref(false)

const contextMenuActions: ContextMenuAction[] = [
  {
    key: 'new-folder',
    label: '新建文件夹',
    targets: ['blank'],
    run: (target) => {
      if (target.kind !== 'blank') return
      openCreateFolderModal(target.dir)
    },
  },
  {
    key: 'open-terminal',
    label: '在终端打开',
    targets: ['blank'],
    run: async (target) => {
      if (target.kind !== 'blank') return
      await FileService.OpenTerminal(target.dir)
    },
  },
  {
    key: 'paste',
    label: '粘贴',
    targets: ['blank'],
    disabled: () => !fileClipboard.hasClipboard.value,
    run: async (target) => {
      if (target.kind !== 'blank' || !fileClipboard.clipboard.value) return
      const { paths, mode } = fileClipboard.clipboard.value
      if (mode === 'cut') {
        await FileService.MoveEntries(paths, target.dir, 'rename')
        fileClipboard.clearClipboard()
      } else {
        await FileService.CopyEntries(paths, target.dir, 'rename')
      }
      refresh()
    },
  },
  {
    key: 'refresh',
    label: '刷新',
    targets: ['blank'],
    run: () => refresh(),
  },
  {
    key: 'open',
    label: '打开',
    targets: ['entry'],
    run: async (target) => {
      if (target.kind !== 'entry') return
      await openEntries(operationEntriesForEntry(target.entry))
    },
  },
  {
    key: 'copy-path',
    label: '复制路径',
    targets: ['blank', 'entry'],
    run: async (target) => {
      const paths = pathsForCopyPath(target)
      await Clipboard.SetText(paths.join('\n'))
      message.success(paths.length > 1 ? `已复制 ${paths.length} 个路径` : '已复制路径')
    },
  },
  {
    key: 'copy',
    label: '复制',
    targets: ['entry'],
    run: (target) => {
      if (target.kind !== 'entry') return
      const paths = operationEntriesForEntry(target.entry).map(entry => entry.path)
      fileClipboard.setClipboard(paths, 'copy')
      message.success('已复制到应用剪贴板')
    },
  },
  {
    key: 'cut',
    label: '剪切',
    targets: ['entry'],
    run: (target) => {
      if (target.kind !== 'entry') return
      const paths = operationEntriesForEntry(target.entry).map(entry => entry.path)
      fileClipboard.setClipboard(paths, 'cut')
      message.success('已剪切到应用剪贴板')
    },
  },
  {
    key: 'delete',
    label: '删除',
    targets: ['entry'],
    run: (target) => {
      if (target.kind !== 'entry') return
      openDeleteConfirmModal(operationEntriesForEntry(target.entry))
    },
  },
]

const contextMenuOptions = computed<DropdownOption[]>(() => {
  const target = contextMenu.value.target
  if (!target) return []
  return contextMenuActions
    .filter(action => action.targets.includes(target.kind))
    .map(action => ({
      label: action.label,
      key: action.key,
      disabled: action.disabled?.(target) || false,
    }))
})

function friendlyError(err: unknown, p: string): string {
  const msg = String(err).toLowerCase()
  if (msg.includes('no such file') || msg.includes('not found') || msg.includes('does not exist')) {
    return `路径不存在: ${p}`
  }
  if (msg.includes('not a directory') || msg.includes('not directory')) {
    return `不是文件夹: ${p}`
  }
  if (msg.includes('permission denied') || msg.includes('access denied') || msg.includes('operation not permitted')) {
    return `权限不足: ${p}`
  }
  return String(err)
}

// navigation history
const history = ref<string[]>([])
const historyIndex = ref(-1)

const canGoBack = computed(() => historyIndex.value > 0)
const canGoForward = computed(() => historyIndex.value < history.value.length - 1)
const canGoHome = computed(() => !!props.homeDir && props.path !== props.homeDir)
const canGoUp = computed(() => parentPath.value !== null)

watch(() => props.path, (newPath) => {
  if (!newPath) return
  exitMultiSelectMode()
  currentRowKey.value = ''
  const existingIndex = history.value.indexOf(newPath)
  const currentHistoryPath = historyIndex.value >= 0 ? history.value[historyIndex.value] : null
  if (newPath !== currentHistoryPath) {
    if (existingIndex >= 0) {
      historyIndex.value = existingIndex
    } else {
      history.value = [...history.value.slice(0, historyIndex.value + 1), newPath]
      historyIndex.value = history.value.length - 1
    }
  }
  if (newPath) {
    loadDir(newPath)
  }
}, { immediate: true })

function formatSize(bytes: number): string {
  if (bytes === 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  if (i === 0) return `${bytes} B`
  return `${(bytes / 1024 ** i).toFixed(1)} ${units[i]}`
}

function formatTime(t: unknown): string {
  if (!t) return '-'
  try {
    return new Date(t as string).toLocaleString()
  } catch {
    return String(t)
  }
}

function compareText(a: string, b: string): number {
  return a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' })
}

function compareTime(a: unknown, b: unknown): number {
  const left = a ? new Date(a as string).getTime() : 0
  const right = b ? new Date(b as string).getTime() : 0
  return left - right
}

function isParentEntry(row: FileEntry): boolean {
  return row.path.startsWith(parentEntryPathPrefix)
}

function actualEntryPath(row: FileEntry): string {
  return isParentEntry(row) ? row.path.slice(parentEntryPathPrefix.length) : row.path
}

function comparePinnedParent(row1: FileEntry, row2: FileEntry): number | null {
  const leftParent = isParentEntry(row1)
  const rightParent = isParentEntry(row2)
  if (leftParent && !rightParent) return -1
  if (!leftParent && rightParent) return 1
  return null
}

function goBack() {
  if (!canGoBack.value) return
  historyIndex.value--
  emit('navigate', history.value[historyIndex.value])
}

function goForward() {
  if (!canGoForward.value) return
  historyIndex.value++
  emit('navigate', history.value[historyIndex.value])
}

function goUp() {
  if (!canGoUp.value) return
  emit('navigate', parentPath.value!)
}

function goHome() {
  if (!canGoHome.value) return
  emit('navigate', props.homeDir)
}

const parentPath = computed(() => {
  return getParentPath(props.path, props.separator)
})
const parentEntry = computed<FileEntry | null>(() => {
  if (!parentPath.value) return null
  return {
    name: '..',
    path: `${parentEntryPathPrefix}${parentPath.value}`,
    size: 0,
    modTime: null,
    isDir: true,
    isHidden: false,
    isSymlink: false,
    linkTarget: '',
    isExecutable: false,
  }
})

const pathInput = ref(props.path)
watch(() => props.path, (p) => { pathInput.value = p })
const pathInputRef = ref<InstanceType<typeof NInput> | null>(null)
const searchVisible = ref(false)
const searchQuery = ref('')
const searchInputRef = ref<InstanceType<typeof NInput> | null>(null)
type PinyinFn = typeof import('pinyin-pro')['pinyin']
let pinyinFn: PinyinFn | null = null
let pinyinLoadPromise: Promise<void> | null = null
const pinyinReady = ref(false)

function onPathSubmit() {
  const trimmed = pathInput.value.trim()
  if (trimmed && trimmed !== props.path) {
    emit('navigate', trimmed)
  }
  focusFileTable()
}

function toggleSearch() {
  if (searchVisible.value) {
    closeSearch()
    return
  }
  searchVisible.value = true
  void ensurePinyinLoaded()
  nextTick(() => {
    searchInputRef.value?.focus()
  })
}

function closeSearch() {
  searchQuery.value = ''
  searchVisible.value = false
}

function focusPathInput() {
  pathInputRef.value?.focus()
}

function focusFileTable() {
  fileTableRef.value?.focus({ preventScroll: true })
}

function ensurePinyinLoaded(): Promise<void> {
  if (pinyinFn) return Promise.resolve()
  if (!pinyinLoadPromise) {
    pinyinLoadPromise = import('pinyin-pro').then((module) => {
      pinyinFn = module.pinyin
      pinyinReady.value = true
    }).catch((err) => {
      console.error('Failed to load pinyin search:', err)
    })
  }
  return pinyinLoadPromise
}

function normalizeSearchText(text: string): string {
  return text.toLowerCase().replace(/\s+/g, '')
}

function pinyinText(text: string, pattern: 'pinyin' | 'first'): string {
  if (!pinyinFn) return ''
  return normalizeSearchText(pinyinFn(text, {
    toneType: 'none',
    pattern,
    separator: '',
    nonZh: 'consecutive',
  }))
}

function matchesSearch(entry: FileEntry, query: string): boolean {
  const name = normalizeSearchText(entry.name)
  if (name.includes(query)) return true
  return pinyinText(entry.name, 'pinyin').includes(query) || pinyinText(entry.name, 'first').includes(query)
}

function filterEntriesBySearch(source: FileEntry[]): FileEntry[] {
  const query = normalizeSearchText(searchQuery.value)
  pinyinReady.value
  if (!query) return source
  return source.filter(entry => matchesSearch(entry, query))
}

const baseColumns: DataTableColumns<FileEntry> = [
  {
    title: 'Name',
    key: 'name',
    sorter: (row1, row2) => comparePinnedParent(row1, row2) ?? compareText(row1.name, row2.name),
    render(row) {
      const fileIcon = resolveFileIcon(row)
      return h('div', { class: 'file-name-cell' }, [
        h(NIcon, {
          class: 'file-icon',
          color: fileIcon.color,
          size: 18,
          title: fileIcon.label,
        }, { default: () => h(fileIcon.icon) }),
        h('span', { class: 'file-name-text' }, row.name),
      ])
    },
  },
  {
    title: 'Type',
    key: 'type',
    width: '110px',
    sorter: (row1, row2) => comparePinnedParent(row1, row2) ?? compareText(fileTypeLabel(row1), fileTypeLabel(row2)),
    render(row) {
      return fileTypeLabel(row)
    },
  },
  {
    title: 'Size',
    key: 'size',
    width: '85px',
    sorter: (row1, row2) => comparePinnedParent(row1, row2) ?? row1.size - row2.size,
    render(row) {
      return row.isDir ? '-' : formatSize(row.size)
    },
  },
  {
    title: 'Modified',
    key: 'modTime',
    width: '165px',
    sorter: (row1, row2) => comparePinnedParent(row1, row2) ?? compareTime(row1.modTime, row2.modTime),
    render(row) {
      return formatTime(row.modTime)
    },
  },
]

const columns = computed<DataTableColumns<FileEntry>>(() => {
  if (!multiSelectMode.value) return baseColumns
  return [
    {
      type: 'selection',
      width: 36,
      disabled: (row) => isParentEntry(row),
    },
    ...baseColumns,
  ]
})

async function onRowDblclick(row: FileEntry) {
  await openEntries(operationEntriesForEntry(row))
}

async function openEntry(row: FileEntry) {
  if (row.isDir) {
    emit('navigate', actualEntryPath(row))
  } else {
    try {
      await FileService.OpenFile(row.path)
    } catch (err) {
      console.error('OpenFile failed:', row.path, err)
      errorMsg.value = friendlyError(err, row.path)
    }
  }
}

async function openEntries(rows: FileEntry[]) {
  if (rows.length === 1) {
    await openEntry(rows[0])
    return
  }
  for (const row of rows) {
    await FileService.OpenFile(row.path)
  }
}

function toggleMultiSelectMode() {
  multiSelectMode.value = !multiSelectMode.value
  if (!multiSelectMode.value) {
    selectedRowKeys.value = []
  }
}

function exitMultiSelectMode() {
  if (!multiSelectMode.value && selectedRowKeys.value.length === 0) return
  multiSelectMode.value = false
  selectedRowKeys.value = []
}

function onUpdateCheckedRowKeys(keys: Array<string | number>) {
  selectedRowKeys.value = keys.map(key => String(key))
  if (selectedRowKeys.value.length > 0) {
    currentRowKey.value = selectedRowKeys.value[selectedRowKeys.value.length - 1]
  }
}

function onRowClick(e: MouseEvent, row: FileEntry) {
  const target = e.target as HTMLElement | null
  if (target?.closest('.n-checkbox, button, input, textarea, a')) return
  currentRowKey.value = row.path
  if (isParentEntry(row)) return
  if (!multiSelectMode.value) {
    selectedRowKeys.value = [row.path]
    return
  }
  toggleSelectedRow(row.path)
}

function toggleSelectedRow(path: string) {
  const selected = selectedPathSet.value
  if (selected.has(path)) {
    selectedRowKeys.value = selectedRowKeys.value.filter(key => key !== path)
  } else {
    selectedRowKeys.value = [...selectedRowKeys.value, path]
  }
}

function selectedEntries() {
  const selected = selectedPathSet.value
  return entries.value.filter(entry => selected.has(entry.path))
}

function currentEntry(): FileEntry | null {
  if (currentRowKey.value) {
    const entry = visibleEntries.value.find(item => item.path === currentRowKey.value)
    if (entry) return entry
  }
  if (selectedRowKeys.value.length > 0) {
    const lastSelectedKey = selectedRowKeys.value[selectedRowKeys.value.length - 1]
    return visibleEntries.value.find(item => item.path === lastSelectedKey) || null
  }
  return null
}

function fileOperationEntriesForCurrent(): FileEntry[] {
  const selected = selectedEntries()
  if (multiSelectMode.value && selected.length > 0) return selected
  const entry = currentEntry()
  if (!entry || isParentEntry(entry)) return []
  return [entry]
}

function setCurrentEntry(entry: FileEntry) {
  currentRowKey.value = entry.path
  if (!multiSelectMode.value) {
    selectedRowKeys.value = [entry.path]
  }
  scrollCurrentEntryIntoView(entry.path)
}

function scrollCurrentEntryIntoView(path: string) {
  const rowIndex = visibleEntries.value.findIndex(entry => entry.path === path)
  if (rowIndex < 0) return
  nextTick(() => {
    dataTableRef.value?.scrollTo({ index: rowIndex } as any)
  })
}

function selectEntryByOffset(offset: number) {
  const source = visibleEntries.value
  if (source.length === 0) return
  const currentIndex = source.findIndex(entry => entry.path === currentRowKey.value)
  const nextIndex = currentIndex < 0
    ? (offset > 0 ? 0 : source.length - 1)
    : Math.min(Math.max(currentIndex + offset, 0), source.length - 1)
  setCurrentEntry(source[nextIndex])
}

function selectFirstEntry() {
  const first = visibleEntries.value[0]
  if (first) setCurrentEntry(first)
}

function selectLastEntry() {
  const last = visibleEntries.value[visibleEntries.value.length - 1]
  if (last) setCurrentEntry(last)
}

async function openCurrentEntry() {
  const entry = currentEntry()
  if (!entry) return
  await openEntries(operationEntriesForEntry(entry))
}

function copyCurrentEntries() {
  const entries = fileOperationEntriesForCurrent()
  if (entries.length === 0) return
  fileClipboard.setClipboard(entries.map(entry => entry.path), 'copy')
  message.success(entries.length > 1 ? `已复制 ${entries.length} 项到应用剪贴板` : '已复制到应用剪贴板')
}

function cutCurrentEntries() {
  const entries = fileOperationEntriesForCurrent()
  if (entries.length === 0) return
  fileClipboard.setClipboard(entries.map(entry => entry.path), 'cut')
  message.success(entries.length > 1 ? `已剪切 ${entries.length} 项到应用剪贴板` : '已剪切到应用剪贴板')
}

async function pasteClipboardEntries() {
  const clipboard = fileClipboard.clipboard.value
  if (!clipboard) return
  const { paths, mode } = clipboard
  if (mode === 'cut') {
    await FileService.MoveEntries(paths, props.path, 'rename')
    fileClipboard.clearClipboard()
  } else {
    await FileService.CopyEntries(paths, props.path, 'rename')
  }
  refresh()
}

function deleteCurrentEntries() {
  const entries = fileOperationEntriesForCurrent()
  if (entries.length === 0) return
  openDeleteConfirmModal(entries)
}

function toggleCurrentEntrySelection() {
  if (!multiSelectMode.value) return
  const entry = currentEntry()
  if (!entry || isParentEntry(entry)) return
  toggleSelectedRow(entry.path)
}

function operationEntriesForEntry(entry: FileEntry): FileEntry[] {
  if (isParentEntry(entry)) return [entry]
  if (multiSelectMode.value && selectedPathSet.value.has(entry.path)) {
    const selected = selectedEntries()
    if (selected.length > 0) return selected
  }
  if (multiSelectMode.value) {
    exitMultiSelectMode()
  }
  return [entry]
}

function dragPathsForRow(row: FileEntry): string[] {
  if (isParentEntry(row)) return []
  return operationEntriesForEntry(row).map(entry => entry.path)
}

function pathsForCopyPath(target: ContextTarget): string[] {
  if (target.kind === 'blank') {
    return [target.dir]
  }
  return operationEntriesForEntry(target.entry).map(entry => entry.path)
}

function showContextMenu(e: MouseEvent, target: ContextTarget) {
  e.preventDefault()
  contextMenu.value = {
    show: false,
    x: e.clientX,
    y: e.clientY,
    target,
  }
  requestAnimationFrame(() => {
    contextMenu.value.show = true
  })
}

function onTableContextMenu(e: MouseEvent) {
  showContextMenu(e, { kind: 'blank', dir: props.path })
}

function onRowContextMenu(e: MouseEvent, row: FileEntry) {
  e.stopPropagation()
  if (isParentEntry(row)) {
    e.preventDefault()
    return
  }
  if (multiSelectMode.value && !selectedPathSet.value.has(row.path)) {
    exitMultiSelectMode()
  }
  showContextMenu(e, { kind: 'entry', entry: row })
}

function hideContextMenu() {
  contextMenu.value.show = false
}

function openCreateFolderModal(dir: string) {
  createFolderModal.value = {
    show: true,
    dir,
    name: '新建文件夹',
  }
}

function closeCreateFolderModal() {
  createFolderModal.value.show = false
}

async function confirmCreateFolder() {
  const { dir, name } = createFolderModal.value
  if (!dir) return
  try {
    await FileService.CreateFolder(dir, name.trim() || '新建文件夹')
    closeCreateFolderModal()
    refresh()
  } catch (err) {
    console.error('Create folder failed:', err)
    message.error(friendlyActionError(err))
  }
}

function openDeleteConfirmModal(entries: FileEntry[]) {
  const deletePathSet = new Set(entries.map(entry => entry.path))
  const fallbackIndex = visibleEntries.value
    .filter(entry => !isParentEntry(entry))
    .findIndex(entry => deletePathSet.has(entry.path))
  deleteConfirmModal.value = {
    show: true,
    entries,
    fallbackIndex,
  }
}

function closeDeleteConfirmModal() {
  deleteConfirmModal.value.show = false
}

async function confirmDeleteEntry() {
  const { entries, fallbackIndex } = deleteConfirmModal.value
  if (entries.length === 0) return
  try {
    const deletedPaths = await FileService.DeleteEntries(entries.map(entry => entry.path))
    closeDeleteConfirmModal()
    removeEntries(deletedPaths, fallbackIndex)
  } catch (err) {
    console.error('Delete entry failed:', err)
    message.error(friendlyActionError(err))
  }
}

function removeEntries(paths: string[], fallbackIndex = -1) {
  const deleted = new Set(paths)
  entries.value = entries.value.filter(entry => !deleted.has(entry.path))
  selectedRowKeys.value = selectedRowKeys.value.filter(path => !deleted.has(path))
  nextTick(() => {
    selectEntryNearIndex(fallbackIndex)
  })
}

function selectEntryNearIndex(index: number) {
  const candidates = visibleEntries.value.filter(entry => !isParentEntry(entry))
  if (candidates.length === 0) {
    currentRowKey.value = ''
    selectedRowKeys.value = []
    return
  }
  const normalizedIndex = index < 0 ? 0 : Math.min(index, candidates.length - 1)
  setCurrentEntry(candidates[normalizedIndex])
}

async function onContextMenuSelect(key: string | number) {
  const target = contextMenu.value.target
  hideContextMenu()
  if (!target) return
  const action = contextMenuActions.find(item => item.key === key)
  if (!action || action.disabled?.(target)) return
  try {
    await action.run(target)
  } catch (err) {
    console.error('Context menu action failed:', key, err)
    message.error(friendlyActionError(err))
  }
}

function friendlyActionError(err: unknown): string {
  const text = err instanceof Error ? err.message : String(err)
  const lower = text.toLowerCase()
  if (lower.includes('permission denied') || lower.includes('access denied') || lower.includes('operation not permitted')) {
    return '权限不足，操作失败'
  }
  if (lower.includes('no such file') || lower.includes('not found') || lower.includes('does not exist')) {
    return '源文件或目标目录不存在'
  }
  if (lower.includes('into itself') || lower.includes('subdirectory')) {
    return '不能复制或移动到自身或子目录'
  }
  return `操作失败：${text}`
}

function activateShortcutScope() {
  activeFileTableShortcutScopeId = shortcutScopeId
}

function isShortcutScopeActive() {
  return activeFileTableShortcutScopeId === shortcutScopeId
}

function handleEscapeShortcut() {
  if (shortcutHelpModal.value) {
    shortcutHelpModal.value = false
    return
  }
  if (deleteConfirmModal.value.show) {
    closeDeleteConfirmModal()
    return
  }
  if (showConfirm.value) {
    onCancel()
    return
  }
  if (createFolderModal.value.show) {
    closeCreateFolderModal()
    return
  }
  if (contextMenu.value.show) {
    hideContextMenu()
    return
  }
  if (searchVisible.value) {
    closeSearch()
    return
  }
  if (multiSelectMode.value) {
    exitMultiSelectMode()
  }
}

async function handleEnterShortcut() {
  if (deleteConfirmModal.value.show) {
    await confirmDeleteEntry()
  }
}

const shortcutActions: ShortcutAction[] = [
  { id: 'select-next', label: '选择下一项', keys: [{ key: 'j' }], run: () => selectEntryByOffset(1) },
  { id: 'select-prev', label: '选择上一项', keys: [{ key: 'k' }], run: () => selectEntryByOffset(-1) },
  { id: 'go-up', label: '返回上级目录', keys: [{ key: 'h' }], run: () => goUp(), disabled: () => !canGoUp.value },
  { id: 'open', label: '打开当前项', keys: [{ key: 'l' }], run: () => openCurrentEntry() },
  { id: 'toggle-search', label: '切换搜索栏', keys: [{ key: '/' }, { key: 'f', ctrlOrMeta: true, allowInEditable: true }], run: () => toggleSearch() },
  { id: 'escape', label: '退出搜索/多选/弹窗', keys: [{ key: 'escape' }], run: () => handleEscapeShortcut(), allowInEditable: true },
  { id: 'toggle-multi-select', label: '切换多选模式', keys: [{ key: 'm' }], run: () => toggleMultiSelectMode() },
  { id: 'toggle-current-selection', label: '切换当前项选择', keys: [{ key: 'space' }], run: () => toggleCurrentEntrySelection(), disabled: () => !multiSelectMode.value },
  { id: 'help', label: '显示热键速查表', keys: [{ key: '?', shift: true }], run: () => { shortcutHelpModal.value = true } },
  { id: 'copy', label: '复制当前项', keys: [{ key: 'y' }, { key: 'c', ctrlOrMeta: true }], run: () => copyCurrentEntries() },
  { id: 'paste', label: '粘贴到当前目录', keys: [{ key: 'p' }, { key: 'v', ctrlOrMeta: true }], run: () => pasteClipboardEntries(), disabled: () => !fileClipboard.hasClipboard.value },
  { id: 'cut', label: '剪切当前项', keys: [{ key: 'x' }, { key: 'x', ctrlOrMeta: true }], run: () => cutCurrentEntries() },
  { id: 'select-first', label: '选择第一项', keys: [[{ key: 'g' }, { key: 'g' }]], run: () => selectFirstEntry() },
  { id: 'select-last', label: '选择最后一项', keys: [{ key: 'g', shift: true }], run: () => selectLastEntry() },
  { id: 'delete', label: '删除当前项', keys: [{ key: 'd' }, { key: 'delete' }], run: () => deleteCurrentEntries() },
  { id: 'confirm', label: '确认当前弹窗', keys: [{ key: 'enter' }], run: () => handleEnterShortcut(), allowInEditable: true, disabled: () => !deleteConfirmModal.value.show },
  { id: 'focus-path', label: '聚焦路径栏', keys: [{ key: 'o' }], run: () => focusPathInput() },
  { id: 'split-vertical', label: '竖直分屏', keys: [{ key: 'd', ctrlOrMeta: true }], run: () => emit('splitV') },
  { id: 'split-horizontal', label: '水平分屏', keys: [{ key: 'd', ctrlOrMeta: true, shift: true }], run: () => emit('splitH') },
]

const shortcutHelpRows = computed(() => shortcutActions.map(action => ({
  id: action.id,
  label: action.label,
  keys: action.keys.map(formatShortcutBinding).join(' / '),
})))

useKeyboardShortcuts(() => shortcutActions, {
  active: isShortcutScopeActive,
})
</script>

<template>
  <div
    ref="fileTableRef"
    class="file-table"
    tabindex="-1"
    @pointerdown="activateShortcutScope"
    @focusin="activateShortcutScope"
  >
    <div class="toolbar">
      <div class="toolbar-left">
        <NButton
          text
          :disabled="!canGoBack"
          @click="goBack"
        >
          <template #icon>
            <n-icon><ArrowBackRound/></n-icon>
          </template>
        </NButton>
        <NButton
          text
          :disabled="!canGoForward"
          @click="goForward"
        >
          <template #icon>
            <n-icon><ArrowForwardRound/></n-icon>
          </template>
        </NButton>
        <NButton
          text
          :disabled="!canGoUp"
          @click="goUp"
        >
          <template #icon>
            <n-icon><FolderArrowUp24Regular/></n-icon>
          </template>
        </NButton>
        <NButton
            text
            :disabled="!canGoHome"
            title="主页"
            @click="goHome"
        >
          <template #icon>
            <n-icon><Home28Regular/></n-icon>
          </template>
        </NButton>
        <NButton
            text
            title="刷新"
            @click="refresh"
        >
          <template #icon>
            <n-icon><RefreshSharp/></n-icon>
          </template>
        </NButton>
        <NInput
          ref="pathInputRef"
          class="path-input"
          v-model:value="pathInput"
          size="tiny"
          placeholder="输入路径后回车"
          :status="pathError ? 'error' : undefined"
          @keyup.enter="onPathSubmit"
          @input="pathError = false"
        />
        <NButton
          text
          :type="searchVisible ? 'primary' : 'default'"
          title="搜索"
          @click="toggleSearch"
        >
          <template #icon>
            <n-icon><SearchOutlined/></n-icon>
          </template>
        </NButton>
        <NButton
          text
          :type="multiSelectMode ? 'primary' : 'default'"
          title="多选"
          @click="toggleMultiSelectMode"
        >
          <template #icon>
            <n-icon><ChecklistOutlined/></n-icon>
          </template>
        </NButton>
      </div>
      <div class="toolbar-right">
        <NButton
          text
          title="竖直分屏"
          @click="emit('splitV')"
        >
          <template #icon>
            <n-icon><SplitVertical28Regular/></n-icon>
          </template>
        </NButton>
        <NButton
          text
          title="水平分屏"
          @click="emit('splitH')"
        >
          <template #icon>
            <n-icon><SplitHorizontal28Regular/></n-icon>
          </template>
        </NButton>
        <NButton
            v-if="closable"
            text
            type="error"
            @click="emit('close')"
        >
          <template #icon>
            <n-icon><CloseSharp/></n-icon>
          </template>
        </NButton>
      </div>
    </div>
    <div v-if="searchVisible" class="search-row">
      <NInput
        ref="searchInputRef"
        v-model:value="searchQuery"
        size="small"
        clearable
        placeholder="搜索当前文件夹"
        @keyup.enter="focusFileTable"
      />
    </div>
    <div
      class="table-area"
      @dragover="onDragOver"
      @dragenter="onDragEnter"
      @dragleave="onDragLeave"
      @drop="onDrop"
      @contextmenu="onTableContextMenu"
    >
      <div v-if="isDragOver" class="drag-overlay">
        <span class="drag-label">{{ dragLabel }}</span>
      </div>
      <NAlert v-if="errorMsg" type="error" :title="errorMsg" class="error-alert" />
      <NSpin v-else-if="loading" class="spin-fill" />
      <NDataTable
        ref="dataTableRef"
        v-else-if="visibleEntries.length > 0"
        :columns="columns"
        :data="visibleEntries"
        :row-key="(row: FileEntry) => row.path"
        :checked-row-keys="selectedRowKeys"
        :on-update:checked-row-keys="onUpdateCheckedRowKeys"
        :row-props="(row: FileEntry) => ({
          style: 'cursor: pointer',
          class: [
            hoveredFolderPath === row.path ? 'drag-target-folder' : '',
            contextActivePath === row.path ? 'context-active-entry' : '',
            currentRowKey === row.path ? 'current-entry' : '',
            selectedPathSet.has(row.path) ? 'selected-entry' : '',
            cutPathSet.has(row.path) ? 'cut-entry' : '',
          ].filter(Boolean).join(' '),
          'data-folder-path': row.isDir && !isParentEntry(row) ? row.path : undefined,
          draggable: !isParentEntry(row),
          onDragstart: (e: DragEvent) => onRowDragStart(e, row, dragPathsForRow(row)),
          onDragend: () => clearDrag(),
          onClick: (e: MouseEvent) => onRowClick(e, row),
          onDblclick: () => onRowDblclick(row),
          onContextmenu: (e: MouseEvent) => onRowContextMenu(e, row),
        })"
        :bordered="false"
        single-line
        size="small"
        flex-height
        :virtual-scroll="true"
        class="data-table"
      />
      <NEmpty v-else :description="searchQuery ? 'No matches' : 'Empty directory'" class="empty-fill" />
      <NDropdown
        trigger="manual"
        placement="bottom-start"
        :show="contextMenu.show"
        :x="contextMenu.x"
        :y="contextMenu.y"
        :options="contextMenuOptions"
        @select="onContextMenuSelect"
        @clickoutside="hideContextMenu"
      />
    </div>
    <DropConfirmModal
      :show="showConfirm"
      :sources="pendingDrop?.paths || []"
      :target-dir="pendingDrop?.targetDir || ''"
      @confirm="onConfirm"
      @update:show="(v: boolean) => !v && onCancel()"
    />
    <NModal
      v-model:show="createFolderModal.show"
      preset="card"
      title="新建文件夹"
      style="width: 360px"
    >
      <div class="modal-body">
        <NInput
          v-model:value="createFolderModal.name"
          placeholder="文件夹名"
          autofocus
          @keyup.enter="confirmCreateFolder"
        />
      </div>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="closeCreateFolderModal">取消</NButton>
          <NButton type="primary" @click="confirmCreateFolder">创建</NButton>
        </NSpace>
      </template>
    </NModal>
    <NModal
      v-model:show="deleteConfirmModal.show"
      preset="card"
      title="确认删除"
      style="width: 360px"
    >
      <div class="modal-body">
        <template v-if="deleteConfirmModal.entries.length === 1">
          确定删除「{{ deleteConfirmModal.entries[0]?.name }}」吗？
        </template>
        <template v-else>
          确定删除选中的 {{ deleteConfirmModal.entries.length }} 项吗？
        </template>
      </div>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="closeDeleteConfirmModal">取消</NButton>
          <NButton type="error" @click="confirmDeleteEntry">删除</NButton>
        </NSpace>
      </template>
    </NModal>
    <NModal
      v-model:show="shortcutHelpModal"
      preset="card"
      title="快捷键"
      style="width: 420px"
    >
      <div class="shortcut-help">
        <div
          v-for="shortcut in shortcutHelpRows"
          :key="shortcut.id"
          class="shortcut-help-row"
        >
          <span class="shortcut-help-keys">{{ shortcut.keys }}</span>
          <span class="shortcut-help-label">{{ shortcut.label }}</span>
        </div>
      </div>
    </NModal>
  </div>
</template>

<style scoped>
.file-table {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 8px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color-embedded);
  flex-shrink: 0;
  gap: 8px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  flex: 1;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.path-input {
  flex: 1;
  min-width: 0;
}

.search-row {
  padding: 6px 8px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--n-color);
  flex-shrink: 0;
}

.table-area {
  flex: 1;
  overflow: auto;
  min-height: 0;
  position: relative;
}

.spin-fill {
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
}

.data-table {
  height: 100%;
}

.empty-fill {
  width: 100%;
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
}

.error-alert {
  margin: 16px;
}

:deep(.file-name-cell) {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  line-height: 18px;
}

:deep(.file-icon) {
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  line-height: 1;
}

:deep(.file-icon svg) {
  display: block;
}

:deep(.file-name-text) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.modal-body {
  font-size: 13px;
}

.drag-overlay {
  position: absolute;
  inset: 0;
  background: rgba(var(--n-primary-color-rgb, 24, 160, 88), 0.08);
  border: 2px dashed var(--n-primary-color, #18a058);
  z-index: 10;
  display: flex;
  justify-content: center;
  align-items: center;
  pointer-events: none;
}

.drag-label {
  background: var(--n-primary-color, #18a058);
  color: #fff;
  padding: 6px 16px;
  border-radius: 4px;
  font-size: 14px;
}

:deep(tr.drag-target-folder) {
  outline: 2px solid var(--n-primary-color, #18a058);
  outline-offset: -2px;
  background: rgba(var(--n-primary-color-rgb, 24, 160, 88), 0.1) !important;
}

:deep(tr.selected-entry td) {
  background: rgba(var(--n-primary-color-rgb, 24, 160, 88), 0.14) !important;
}

:deep(tr.selected-entry:hover td) {
  background: rgba(var(--n-primary-color-rgb, 24, 160, 88), 0.18) !important;
}

:deep(tr.current-entry td:first-child) {
  box-shadow: inset 3px 0 0 var(--n-primary-color, #18a058);
}

:deep(tr.context-active-entry td) {
  background: rgba(var(--n-primary-color-rgb, 24, 160, 88), 0.14) !important;
}

:deep(tr.cut-entry td) {
  opacity: 0.45;
}

.shortcut-help {
  display: grid;
  gap: 6px;
}

.shortcut-help-row {
  display: grid;
  grid-template-columns: 130px 1fr;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.shortcut-help-keys {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  color: var(--n-text-color-2);
}

.shortcut-help-label {
  color: var(--n-text-color);
}
</style>
