<script lang="ts">
let nextFileTableShortcutScopeId = 1
let activeFileTableShortcutScopeId = 0
</script>

<script setup lang="ts">
import { ref, watch, computed, h, nextTick, onMounted } from 'vue'
import { NDataTable, NButton, NText, NSpin, NIcon, NEmpty, NAlert, NInput, NAutoComplete, NDropdown, NModal, NSpace, NTag, useMessage } from 'naive-ui'
import type { AutoCompleteInst, AutoCompleteOption, DataTableColumns, DataTableInst, DataTableSortState, DropdownOption } from 'naive-ui'
import { Clipboard } from '@wailsio/runtime'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { CloseSharp, ArrowBackRound, ArrowForwardRound, RefreshSharp, ChecklistOutlined, SearchOutlined, UndoSharp, RedoSharp } from '@vicons/material'
import { SplitVertical28Regular, SplitHorizontal28Regular, FolderArrowUp24Regular, Home28Regular } from '@vicons/fluent'
import { useSettings } from '../composables/useSettings'
import { useDragDrop, clearDrag } from '../composables/useDragDrop'
import { useFileClipboard } from '../composables/useFileClipboard'
import { formatShortcutBinding, useKeyboardShortcuts } from '../composables/useKeyboardShortcuts'
import type { ShortcutAction } from '../composables/useKeyboardShortcuts'
import { notifyDirectoriesChanged, useDirectoryEvents } from '../composables/useDirectoryEvents'
import { useFileOperationHistory } from '../composables/useFileOperationHistory'
import DropConfirmModal from './DropConfirmModal.vue'
import { fileTypeLabel, resolveFileIcon } from './fileIcons'
import { joinPath, parentPath as getParentPath } from './path'

const { settings } = useSettings()
const message = useMessage()
const parentEntryPathPrefix = '__zashiki_parent__:'
const shortcutScopeId = nextFileTableShortcutScopeId++
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
  const sortedEntries = sortEntries(matchedEntries)
  const parent = parentEntry.value
  return parent ? [parent, ...sortedEntries] : sortedEntries
})

const props = defineProps<{
  path: string
  closable?: boolean
  focused?: boolean
  separator: string
  homeDir: string
  trashLabel?: string
}>()
const operationHistory = useFileOperationHistory(() => props.separator)

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

const directoryEventToken = useDirectoryEvents(() => props.path, () => {
  refresh()
})

const {
  isDragOver, dragLabel, hoveredFolderPath,
  pendingDrop, confirmDrop, cancelDrop,
  onRowDragStart,
  onDragOver, onDragEnter, onDragLeave, onDrop,
} = useDragDrop(() => props.path, {
  onOperationComplete: (action, results) => {
    if (action === 'move') {
      operationHistory.recordMove(results)
    } else {
      operationHistory.recordCopy(results)
    }
  },
})

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
  closeOthers: []
  focusNext: []
  selectionStatus: [status: { multiSelectMode: boolean, selectedCount: number }]
}>()

const entries = ref<FileEntry[]>([])
const loading = ref(false)
const errorMsg = ref('')
const pathError = ref(false)
const multiSelectMode = ref(false)
const selectedRowKeys = ref<string[]>([])
const currentRowKey = ref('')
const selectedPathSet = computed(() => new Set(selectedRowKeys.value))
const sortState = ref<DataTableSortState | null>(null)
const trashLabel = computed(() => props.trashLabel || '回收站')
type ContextTarget = { kind: 'blank', dir: string } | { kind: 'entry', entry: FileEntry }
type ContextActionKey = 'new-folder' | 'open-terminal' | 'paste' | 'refresh' | 'open' | 'rename' | 'copy-path' | 'copy' | 'cut' | 'delete'

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
const renameModal = ref({
  show: false,
  entry: null as FileEntry | null,
  name: '',
})
const renameInputRef = ref<InstanceType<typeof NInput> | null>(null)
const deleteConfirmModal = ref({
  show: false,
  entries: [] as FileEntry[],
  fallbackIndex: -1,
  permanent: false,
})
const historyConfirmModal = ref({
  show: false,
  action: '' as 'undo' | 'redo' | '',
})
const historyConfirmActionLabel = computed(() => historyConfirmModal.value.action === 'undo' ? '撤销' : '重做')
const historyConfirmSummary = computed(() => {
  if (historyConfirmModal.value.action === 'undo') return operationHistory.undoSummary.value
  if (historyConfirmModal.value.action === 'redo') return operationHistory.redoSummary.value
  return null
})
const shortcutHelpModal = ref(false)

watch([multiSelectMode, selectedRowKeys], () => {
  emit('selectionStatus', {
    multiSelectMode: multiSelectMode.value,
    selectedCount: selectedRowKeys.value.length,
  })
}, { immediate: true })

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
      await pasteClipboardEntriesToDir(target.dir)
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
    key: 'rename',
    label: '重命名',
    targets: ['entry'],
    run: (target) => {
      if (target.kind !== 'entry') return
      openRenameModal(target.entry)
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

function compareEntriesByColumn(columnKey: string, row1: FileEntry, row2: FileEntry): number {
  switch (columnKey) {
    case 'name':
      return compareText(row1.name, row2.name)
    case 'type':
      return compareText(fileTypeLabel(row1), fileTypeLabel(row2))
    case 'size':
      return row1.size - row2.size
    case 'modTime':
      return compareTime(row1.modTime, row2.modTime)
    default:
      return 0
  }
}

function sortEntries(source: FileEntry[]): FileEntry[] {
  const state = sortState.value
  if (!state?.order) return source
  const direction = state.order === 'ascend' ? 1 : -1
  return [...source].sort((row1, row2) => direction * compareEntriesByColumn(String(state.columnKey), row1, row2))
}

function sortOrderFor(columnKey: string) {
  return sortState.value?.columnKey === columnKey ? sortState.value.order : false
}

function onUpdateSorter(state: DataTableSortState | DataTableSortState[] | null) {
  sortState.value = Array.isArray(state) ? state[0] || null : state
}

function isParentEntry(row: FileEntry): boolean {
  return row.path.startsWith(parentEntryPathPrefix)
}

function actualEntryPath(row: FileEntry): string {
  return isParentEntry(row) ? row.path.slice(parentEntryPathPrefix.length) : row.path
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
watch(() => props.path, (p) => {
  pathInput.value = p
  pathAutocompleteOptions.value = []
})
const pathInputRef = ref<AutoCompleteInst | null>(null)
const pathAutocompleteOptions = ref<AutoCompleteOption[]>([])
const pathAutocompleteLoading = ref(false)
let pathAutocompleteRequestId = 0
const searchVisible = ref(false)
const searchQuery = ref('')
const searchInputRef = ref<InstanceType<typeof NInput> | null>(null)
type PinyinFn = typeof import('pinyin-pro')['pinyin']
let pinyinFn: PinyinFn | null = null
let pinyinLoadPromise: Promise<void> | null = null
const pinyinReady = ref(false)
type ShortcutCategory = 'navigation' | 'file' | 'selection' | 'search' | 'panel' | 'dialog'
type CategorizedShortcutAction = ShortcutAction & { category: ShortcutCategory }

function onPathSubmit() {
  const trimmed = pathInput.value.trim()
  if (trimmed && trimmed !== props.path) {
    emit('navigate', trimmed)
  }
  focusFileTable()
}

function onPathInput(value: string) {
  pathInput.value = value
  pathError.value = false
  void updatePathAutocompleteOptions(value)
}

async function updatePathAutocompleteOptions(value: string) {
  const requestId = ++pathAutocompleteRequestId
  const input = value.trim()
  const query = pathCompletionQuery(input)
  if (!query) {
    pathAutocompleteOptions.value = []
    return
  }

  pathAutocompleteLoading.value = true
  try {
    const result = await FileService.ListDir(query.dir)
    if (requestId !== pathAutocompleteRequestId) return
    const normalizedPart = query.part.toLowerCase()
    pathAutocompleteOptions.value = (result || [])
      .filter(entry => entry.isDir)
      .filter(entry => settings.showHiddenFiles || !entry.isHidden)
      .filter(entry => entry.name.toLowerCase().startsWith(normalizedPart))
      .slice(0, 50)
      .map(entry => {
        const value = joinCompletionPath(query.dir, entry.name)
        return {
          label: value + props.separator,
          value,
        }
      })
  } catch {
    if (requestId === pathAutocompleteRequestId) {
      pathAutocompleteOptions.value = []
    }
  } finally {
    if (requestId === pathAutocompleteRequestId) {
      pathAutocompleteLoading.value = false
    }
  }
}

function pathCompletionQuery(input: string): { dir: string, part: string } | null {
  if (!input) return null
  const separators = props.separator === '\\' ? ['\\', '/'] : ['/']
  let index = -1
  for (const separator of separators) {
    index = Math.max(index, input.lastIndexOf(separator))
  }
  if (index < 0) return { dir: props.path, part: input }

  const dir = input.slice(0, index + 1)
  if (!dir) return null
  return { dir: trimCompletionDir(dir), part: input.slice(index + 1) }
}

function trimCompletionDir(dir: string): string {
  if (props.separator !== '\\') return dir === '/' ? dir : dir.replace(/\/+$/, '')
  const normalized = dir.replace(/\//g, '\\')
  const root = normalized.match(/^[A-Za-z]:\\$/) || normalized.match(/^\\\\[^\\]+\\[^\\]+\\$/)
  return root ? normalized : normalized.replace(/\\+$/, '')
}

function joinCompletionPath(dir: string, name: string): string {
  return joinPath(dir, name, props.separator)
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

function blurPathInput() {
  pathInputRef.value?.blur()
  focusFileTable()
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

function createBaseColumns(): DataTableColumns<FileEntry> {
  return [
    {
      title: 'Name',
      key: 'name',
      sorter: true,
      sortOrder: sortOrderFor('name'),
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
      sorter: true,
      sortOrder: sortOrderFor('type'),
      render(row) {
        return fileTypeLabel(row)
      },
    },
    {
      title: 'Size',
      key: 'size',
      width: '85px',
      sorter: true,
      sortOrder: sortOrderFor('size'),
      render(row) {
        return row.isDir ? '-' : formatSize(row.size)
      },
    },
    {
      title: 'Modified',
      key: 'modTime',
      width: '165px',
      sorter: true,
      sortOrder: sortOrderFor('modTime'),
      render(row) {
        return formatTime(row.modTime)
      },
    },
  ]
}

const columns = computed<DataTableColumns<FileEntry>>(() => {
  const baseColumns = createBaseColumns()
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
  if (isParentEntry(row)) return
  const anchorKey = selectionAnchorKey()
  if (e.shiftKey && anchorKey) {
    selectRowRange(anchorKey, row.path, e.ctrlKey || e.metaKey)
    currentRowKey.value = row.path
    return
  }
  if (e.ctrlKey || e.metaKey) {
    multiSelectMode.value = true
    if (selectedRowKeys.value.length === 0 && anchorKey && anchorKey !== row.path) {
      selectedRowKeys.value = [anchorKey]
    }
    toggleSelectedRow(row.path)
    currentRowKey.value = row.path
    return
  }
  currentRowKey.value = row.path
  if (!multiSelectMode.value) {
    selectedRowKeys.value = [row.path]
    return
  }
  toggleSelectedRow(row.path)
}

function selectionAnchorKey(): string {
  if (currentRowKey.value && isSelectableEntryPath(currentRowKey.value)) return currentRowKey.value
  const lastSelectedKey = selectedRowKeys.value[selectedRowKeys.value.length - 1]
  return lastSelectedKey && isSelectableEntryPath(lastSelectedKey) ? lastSelectedKey : ''
}

function isSelectableEntryPath(path: string): boolean {
  return visibleEntries.value.some(entry => entry.path === path && !isParentEntry(entry))
}

function selectRowRange(anchorPath: string, targetPath: string, preserveExisting: boolean) {
  const selectableEntries = visibleEntries.value.filter(entry => !isParentEntry(entry))
  const anchorIndex = selectableEntries.findIndex(entry => entry.path === anchorPath)
  const targetIndex = selectableEntries.findIndex(entry => entry.path === targetPath)
  if (anchorIndex < 0 || targetIndex < 0) return
  const [start, end] = anchorIndex < targetIndex ? [anchorIndex, targetIndex] : [targetIndex, anchorIndex]
  const rangeKeys = selectableEntries.slice(start, end + 1).map(entry => entry.path)
  multiSelectMode.value = true
  selectedRowKeys.value = preserveExisting
    ? uniqueKeys([...selectedRowKeys.value, ...rangeKeys])
    : rangeKeys
}

function uniqueKeys(keys: string[]): string[] {
  return Array.from(new Set(keys))
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
  await pasteClipboardEntriesToDir(props.path)
}

async function pasteClipboardEntriesToDir(targetDir: string) {
  const clipboard = fileClipboard.clipboard.value
  if (!clipboard) return
  const { paths, mode } = clipboard
  if (mode === 'cut') {
    const results = await FileService.MoveEntries(paths, targetDir, 'rename')
    operationHistory.recordMove(results)
    fileClipboard.clearClipboard()
  } else {
    const results = await FileService.CopyEntries(paths, targetDir, 'rename')
    operationHistory.recordCopy(results)
  }
  notifyDirectoriesChanged([targetDir, ...sourceDirsForPaths(paths)])
}

function deleteCurrentEntries() {
  const entries = fileOperationEntriesForCurrent()
  if (entries.length === 0) return
  openDeleteConfirmModal(entries)
}

function permanentlyDeleteCurrentEntries() {
  const entries = fileOperationEntriesForCurrent()
  if (entries.length === 0) return
  openDeleteConfirmModal(entries, true)
}

function renameCurrentEntry() {
  const entry = currentEntry()
  if (!entry || isParentEntry(entry)) return
  openRenameModal(entry)
}

function toggleCurrentEntrySelection() {
  if (!multiSelectMode.value) return
  const entry = currentEntry()
  if (!entry || isParentEntry(entry)) return
  toggleSelectedRow(entry.path)
}

function selectAllEntries() {
  const selectableKeys = visibleEntries.value
    .filter(entry => !isParentEntry(entry))
    .map(entry => entry.path)
  if (selectableKeys.length === 0) return
  multiSelectMode.value = true
  selectedRowKeys.value = selectableKeys
  currentRowKey.value = selectableKeys[selectableKeys.length - 1]
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

function sourceDirsForPaths(paths: string[]): string[] {
  return paths
    .map(path => getParentPath(path, props.separator))
    .filter((path): path is string => !!path)
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
    const path = await FileService.CreateFolder(dir, name.trim() || '新建文件夹')
    operationHistory.recordCreateFolder(path)
    closeCreateFolderModal()
    notifyDirectoriesChanged([dir])
  } catch (err) {
    console.error('Create folder failed:', err)
    message.error(friendlyActionError(err))
  }
}

function openRenameModal(entry: FileEntry) {
  renameModal.value = {
    show: true,
    entry,
    name: entry.name,
  }
  nextTick(() => {
    selectRenameText(entry)
  })
}

function closeRenameModal() {
  renameModal.value.show = false
}

async function confirmRenameEntry() {
  const { entry, name } = renameModal.value
  const trimmed = name.trim()
  if (!entry || !trimmed) return
  try {
    const renamed = await FileService.RenameEntry(entry.path, trimmed)
    operationHistory.recordRename(entry.path, renamed.path)
    closeRenameModal()
    updateRenamedEntry(entry.path, renamed)
    notifyDirectoriesChanged([props.path], { exclude: directoryEventToken })
  } catch (err) {
    console.error('Rename entry failed:', err)
    message.error(friendlyActionError(err))
  }
}

function updateRenamedEntry(oldPath: string, renamed: FileEntry) {
  entries.value = entries.value.map(entry => entry.path === oldPath ? renamed : entry)
  selectedRowKeys.value = selectedRowKeys.value.map(path => path === oldPath ? renamed.path : path)
  if (currentRowKey.value === oldPath) {
    setCurrentEntry(renamed)
  }
}

function selectRenameText(entry: FileEntry) {
  const input = renameInputRef.value?.inputElRef
  if (!input) return
  const selectionEnd = renameSelectionEnd(entry)
  input.focus()
  input.setSelectionRange(0, selectionEnd)
}

function renameSelectionEnd(entry: FileEntry): number {
  if (entry.isDir) return entry.name.length
  const dotIndex = entry.name.lastIndexOf('.')
  if (dotIndex <= 0) return entry.name.length
  return dotIndex
}

function openDeleteConfirmModal(entries: FileEntry[], permanent = false) {
  const deletePathSet = new Set(entries.map(entry => entry.path))
  const fallbackIndex = visibleEntries.value
    .filter(entry => !isParentEntry(entry))
    .findIndex(entry => deletePathSet.has(entry.path))
  deleteConfirmModal.value = {
    show: true,
    entries,
    fallbackIndex,
    permanent,
  }
}

function closeDeleteConfirmModal() {
  deleteConfirmModal.value.show = false
}

async function confirmDeleteEntry() {
  const { entries, fallbackIndex, permanent } = deleteConfirmModal.value
  if (entries.length === 0) return
  try {
    const paths = entries.map(entry => entry.path)
    let deletedPaths: string[]
    if (permanent) {
      deletedPaths = await FileService.DeleteEntries(paths)
    } else {
      const results = await FileService.TrashEntries(paths)
      operationHistory.recordTrash(results)
      deletedPaths = results.map(result => result.sourcePath)
    }
    closeDeleteConfirmModal()
    removeEntries(deletedPaths, fallbackIndex)
    notifyDirectoriesChanged([props.path], { exclude: directoryEventToken })
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

onMounted(() => {
  if (props.focused || activeFileTableShortcutScopeId === 0) {
    activateShortcutScope()
    if (props.focused) {
      nextTick(() => focusFileTable())
    }
  }
})

watch(() => props.focused, (focused) => {
  if (focused) {
    activateShortcutScope()
    nextTick(() => focusFileTable())
  }
})

function isShortcutScopeActive() {
  return props.focused === true
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
  if (historyConfirmModal.value.show) {
    closeHistoryConfirmModal()
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
  if (renameModal.value.show) {
    closeRenameModal()
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
  if (renameModal.value.show) {
    await confirmRenameEntry()
    return
  }
  if (deleteConfirmModal.value.show) {
    await confirmDeleteEntry()
    return
  }
  if (historyConfirmModal.value.show) {
    await confirmHistoryOperation()
  }
}

function requestUndoOperation() {
  if (!operationHistory.canUndo.value) return
  historyConfirmModal.value = {
    show: true,
    action: 'undo',
  }
}

function requestRedoOperation() {
  if (!operationHistory.canRedo.value) return
  historyConfirmModal.value = {
    show: true,
    action: 'redo',
  }
}

function closeHistoryConfirmModal() {
  historyConfirmModal.value.show = false
}

async function confirmHistoryOperation() {
  const action = historyConfirmModal.value.action
  if (!action) return
  closeHistoryConfirmModal()
  if (action === 'undo') {
    await undoLastOperation()
  } else {
    await redoLastOperation()
  }
}

async function undoLastOperation() {
  try {
    await operationHistory.undo()
  } catch (err) {
    console.error('Undo failed:', err)
    message.error(friendlyActionError(err))
  }
}

async function redoLastOperation() {
  try {
    await operationHistory.redo()
  } catch (err) {
    console.error('Redo failed:', err)
    message.error(friendlyActionError(err))
  }
}

const shortcutActions: CategorizedShortcutAction[] = [
  { id: 'select-next', category: 'selection', label: '选择下一项', keys: [{ key: 'j' }], run: () => selectEntryByOffset(1) },
  { id: 'select-prev', category: 'selection', label: '选择上一项', keys: [{ key: 'k' }], run: () => selectEntryByOffset(-1) },
  { id: 'go-up', category: 'navigation', label: '返回上级目录', keys: [{ key: 'h' }], run: () => goUp(), disabled: () => !canGoUp.value },
  { id: 'open', category: 'file', label: '打开当前项', keys: [{ key: 'l' }], run: () => openCurrentEntry() },
  { id: 'toggle-search', category: 'search', label: '切换搜索栏', keys: [{ key: '/' }, { key: 'f', ctrlOrMeta: true, allowInEditable: true }], run: () => toggleSearch() },
  { id: 'escape', category: 'dialog', label: '退出搜索/多选/弹窗', keys: [{ key: 'escape' }], run: () => handleEscapeShortcut(), allowInEditable: true },
  { id: 'toggle-multi-select', category: 'selection', label: '切换多选模式', keys: [{ key: 'm' }], run: () => toggleMultiSelectMode() },
  { id: 'select-all', category: 'selection', label: '选择全部', keys: [{ key: 'a', ctrlOrMeta: true }], run: () => selectAllEntries() },
  { id: 'toggle-current-selection', category: 'selection', label: '切换当前项选择', keys: [{ key: 'space' }], run: () => toggleCurrentEntrySelection(), disabled: () => !multiSelectMode.value },
  { id: 'help', category: 'dialog', label: '显示热键速查表', keys: [{ key: '?', shift: true }], run: () => { shortcutHelpModal.value = true } },
  { id: 'copy', category: 'file', label: '复制当前项', keys: [{ key: 'y' }, { key: 'c', ctrlOrMeta: true }], run: () => copyCurrentEntries() },
  { id: 'paste', category: 'file', label: '粘贴到当前目录', keys: [{ key: 'p' }, { key: 'v', ctrlOrMeta: true }], run: () => pasteClipboardEntries(), disabled: () => !fileClipboard.hasClipboard.value },
  { id: 'undo', category: 'file', label: '撤销', keys: [{ key: 'u' }, { key: 'z', ctrlOrMeta: true }], run: () => requestUndoOperation(), disabled: () => !operationHistory.canUndo.value },
  { id: 'redo', category: 'file', label: '重做', keys: [{ key: 'r', ctrl: true }, { key: 'z', ctrlOrMeta: true, shift: true }, { key: 'y', ctrlOrMeta: true }], run: () => requestRedoOperation(), disabled: () => !operationHistory.canRedo.value },
  { id: 'cut', category: 'file', label: '剪切当前项', keys: [{ key: 'x' }, { key: 'x', ctrlOrMeta: true }], run: () => cutCurrentEntries() },
  { id: 'rename', category: 'file', label: '重命名当前项', keys: [{ key: 'f2' }, { key: 'r' }], run: () => renameCurrentEntry() },
  { id: 'select-first', category: 'selection', label: '选择第一项', keys: [[{ key: 'g' }, { key: 'g' }]], run: () => selectFirstEntry() },
  { id: 'select-last', category: 'selection', label: '选择最后一项', keys: [{ key: 'g', shift: true }], run: () => selectLastEntry() },
  { id: 'delete', category: 'file', label: `移到${trashLabel.value}`, keys: [{ key: 'd' }, { key: 'delete' }], run: () => deleteCurrentEntries() },
  { id: 'permanent-delete', category: 'file', label: '永久删除当前项', keys: [{ key: 'd', shift: true }, { key: 'delete', shift: true }], run: () => permanentlyDeleteCurrentEntries() },
  { id: 'confirm', category: 'dialog', label: '确认当前弹窗', keys: [{ key: 'enter' }], run: () => handleEnterShortcut(), allowInEditable: true, disabled: () => !deleteConfirmModal.value.show && !renameModal.value.show },
  { id: 'focus-path', category: 'navigation', label: '聚焦路径栏', keys: [{ key: 'o' }, { key: 'l', ctrlOrMeta: true }], run: () => focusPathInput() },
  { id: 'refresh', category: 'navigation', label: '刷新', keys: [{ key: 'f5' }], run: () => refresh() },
  { id: 'split-vertical', category: 'panel', label: '竖直分屏', keys: [{ key: 'd', ctrlOrMeta: true }], run: () => emit('splitV') },
  { id: 'split-horizontal', category: 'panel', label: '水平分屏', keys: [{ key: 'd', ctrlOrMeta: true, shift: true }], run: () => emit('splitH') },
  { id: 'close-panel', category: 'panel', label: '关闭当前面板', keys: [{ key: 'w', ctrlOrMeta: true }], run: () => emit('close'), disabled: () => !props.closable },
  { id: 'close-other-panels', category: 'panel', label: '关闭其它面板', keys: [{ key: 'w', ctrlOrMeta: true, shift: true }], run: () => emit('closeOthers'), disabled: () => !props.closable },
  { id: 'focus-next-panel', category: 'panel', label: '切换到下一个面板', keys: [{ key: 'tab' }], run: () => {}, disabled: () => true },
]

const shortcutCategoryLabels: Record<ShortcutCategory, string> = {
  navigation: '导航',
  file: '文件操作',
  selection: '选择',
  search: '搜索',
  panel: '面板',
  dialog: '弹窗',
}

const shortcutHelpGroups = computed(() => {
  const categories: ShortcutCategory[] = ['navigation', 'selection', 'file', 'search', 'panel', 'dialog']
  return categories
    .map(category => ({
      category,
      label: shortcutCategoryLabels[category],
      rows: shortcutActions
        .filter(action => action.category === category)
        .map(action => ({
          id: action.id,
          label: action.label,
          keys: action.keys.map(formatShortcutBinding),
        })),
    }))
    .filter(group => group.rows.length > 0)
})

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
        <NButton
            text
            :disabled="!operationHistory.canUndo.value"
            title="撤销"
            @click="requestUndoOperation"
        >
          <template #icon>
            <n-icon><UndoSharp/></n-icon>
          </template>
        </NButton>
        <NButton
            text
            :disabled="!operationHistory.canRedo.value"
            title="重做"
            @click="requestRedoOperation"
        >
          <template #icon>
            <n-icon><RedoSharp/></n-icon>
          </template>
        </NButton>
        <NAutoComplete
          ref="pathInputRef"
          class="path-input"
          :value="pathInput"
          :options="pathAutocompleteOptions"
          :loading="pathAutocompleteLoading"
          size="small"
          placeholder="输入路径后回车"
          :status="pathError ? 'error' : undefined"
          clearable
          @update:value="onPathInput"
          @keyup.enter="onPathSubmit"
          @keydown.esc.stop.prevent="blurPathInput"
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
        :on-update:sorter="onUpdateSorter"
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
        remote
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
      v-model:show="renameModal.show"
      preset="card"
      title="重命名"
      style="width: 360px"
    >
      <div class="modal-body">
        <NInput
          ref="renameInputRef"
          v-model:value="renameModal.name"
          placeholder="新名称"
          autofocus
          @keyup.enter="confirmRenameEntry"
        />
      </div>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="closeRenameModal">取消</NButton>
          <NButton type="primary" @click="confirmRenameEntry">重命名</NButton>
        </NSpace>
      </template>
    </NModal>
    <NModal
      v-model:show="deleteConfirmModal.show"
      preset="card"
      :title="deleteConfirmModal.permanent ? '确认永久删除' : `确认移到${trashLabel}`"
      style="width: 360px"
    >
      <div class="modal-body">
        <template v-if="deleteConfirmModal.entries.length === 1">
          确定{{ deleteConfirmModal.permanent ? '永久删除' : `移到${trashLabel}` }}「{{ deleteConfirmModal.entries[0]?.name }}」吗？
        </template>
        <template v-else>
          确定{{ deleteConfirmModal.permanent ? '永久删除' : `移到${trashLabel}` }}选中的 {{ deleteConfirmModal.entries.length }} 项吗？
        </template>
      </div>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="closeDeleteConfirmModal">取消</NButton>
          <NButton type="error" @click="confirmDeleteEntry">
            {{ deleteConfirmModal.permanent ? '永久删除' : `移到${trashLabel}` }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
    <NModal
      v-model:show="historyConfirmModal.show"
      preset="card"
      :title="`确认${historyConfirmActionLabel}`"
      style="width: 360px"
    >
      <div class="modal-body">
        <div>确定{{ historyConfirmActionLabel }}以下操作吗？</div>
        <div class="history-operation-summary">{{ historyConfirmSummary?.sentence || '文件操作' }}</div>
      </div>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="closeHistoryConfirmModal">取消</NButton>
          <NButton type="primary" @click="confirmHistoryOperation">
            {{ historyConfirmActionLabel }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
    <NModal
      v-model:show="shortcutHelpModal"
      preset="card"
      title="快捷键"
      style="width: min(900px, 92vw)"
    >
      <div class="shortcut-help">
        <div
          v-for="group in shortcutHelpGroups"
          :key="group.category"
          class="shortcut-help-group"
        >
          <div class="shortcut-help-title">{{ group.label }}</div>
          <div
            v-for="shortcut in group.rows"
            :key="shortcut.id"
            class="shortcut-help-row"
          >
            <span class="shortcut-help-keys">
              <NTag
                v-for="key in shortcut.keys"
                :key="key"
                size="small"
                :bordered="false"
              >
                {{ key }}
              </NTag>
            </span>
            <span class="shortcut-help-label">{{ shortcut.label }}</span>
          </div>
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
  --file-table-scroll-end-space: min(30vh, 240px);
  height: 100%;
}

:deep(.data-table .v-vl-items) {
  padding-bottom: var(--file-table-scroll-end-space) !important;
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

.history-operation-summary {
  margin-top: 8px;
  font-weight: 600;
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
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 14px;
}

.shortcut-help-group {
  display: grid;
  gap: 6px;
}

.shortcut-help-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--n-text-color-3);
}

.shortcut-help-row {
  display: grid;
  grid-template-columns: 210px 1fr;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.shortcut-help-keys {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}

.shortcut-help-keys :deep(.n-tag__content) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}

.shortcut-help-label {
  color: var(--n-text-color);
}
</style>
