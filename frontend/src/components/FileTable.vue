<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { NDataTable, NButton, NText, NSpin, NIcon, NEmpty, NAlert, NInput } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { FileService } from '../../bindings/zashiki'
import type { FileEntry } from '../../bindings/zashiki'
import { CloseSharp, ArrowBackRound, ArrowForwardRound, HomeRound } from '@vicons/material'
import { SplitVertical28Regular, SplitHorizontal28Regular, FolderArrowUp24Regular } from '@vicons/fluent'
import { useSettings } from '../composables/useSettings'
import { useDragDrop, clearDrag } from '../composables/useDragDrop'
import DropConfirmModal from './DropConfirmModal.vue'
import { parentPath as getParentPath } from './path'

const { settings } = useSettings()
const visibleEntries = computed(() => {
  if (settings.showHiddenFiles) return entries.value
  return entries.value.filter((e: FileEntry) => !e.isHidden)
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

const pathInput = ref(props.path)
watch(() => props.path, (p) => { pathInput.value = p })

function onPathSubmit() {
  const trimmed = pathInput.value.trim()
  if (trimmed && trimmed !== props.path) {
    emit('navigate', trimmed)
  }
}

const columns: DataTableColumns<FileEntry> = [
  {
    title: 'Name',
    key: 'name',
    render(row) {
      return [row.isDir ? '📁' : '📄', ' ', row.name].join('')
    },
  },
  {
    title: 'Size',
    key: 'size',
    width: '85px',
    render(row) {
      return row.isDir ? '-' : formatSize(row.size)
    },
  },
  {
    title: 'Modified',
    key: 'modTime',
    width: '165px',
    render(row) {
      return formatTime(row.modTime)
    },
  },
]

async function onRowDblclick(row: FileEntry) {
  if (row.isDir) {
    emit('navigate', row.path)
  } else {
    try {
      await FileService.OpenFile(row.path)
    } catch (err) {
      console.error('OpenFile failed:', row.path, err)
      errorMsg.value = friendlyError(err, row.path)
    }
  }
}
</script>

<template>
  <div class="file-table">
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
          :disabled="!canGoHome"
          title="主页"
          @click="goHome"
        >
          <template #icon>
            <n-icon><HomeRound/></n-icon>
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
        <NInput
          class="path-input"
          v-model:value="pathInput"
          size="tiny"
          placeholder="输入路径后回车"
          :status="pathError ? 'error' : undefined"
          @keyup.enter="onPathSubmit"
          @input="pathError = false"
        />
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
    <div
      class="table-area"
      @dragover="onDragOver"
      @dragenter="onDragEnter"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <div v-if="isDragOver" class="drag-overlay">
        <span class="drag-label">{{ dragLabel }}</span>
      </div>
      <NAlert v-if="errorMsg" type="error" :title="errorMsg" class="error-alert" />
      <NSpin v-else-if="loading" class="spin-fill" />
      <NDataTable
        v-else-if="visibleEntries.length > 0"
        :columns="columns"
        :data="visibleEntries"
        :row-key="(row: FileEntry) => row.path"
        :row-props="(row: FileEntry) => ({
          style: 'cursor: pointer',
          class: hoveredFolderPath === row.path ? 'drag-target-folder' : '',
          'data-folder-path': row.isDir ? row.path : undefined,
          draggable: true,
          onDragstart: (e: DragEvent) => onRowDragStart(e, row),
          onDragend: () => clearDrag(),
          onDblclick: () => onRowDblclick(row),
        })"
        :bordered="false"
        :single-line="false"
        size="small"
        flex-height
        :virtual-scroll="true"
        class="data-table"
      />
      <NEmpty v-else description="Empty directory" class="empty-fill" />
    </div>
    <DropConfirmModal
      :show="showConfirm"
      :sources="pendingDrop?.paths || []"
      :target-dir="pendingDrop?.targetDir || ''"
      @confirm="onConfirm"
      @update:show="(v: boolean) => !v && onCancel()"
    />
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
</style>
