<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { NDataTable, NButton, NText, NSpin, NIcon, NEmpty, NAlert, NInput } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { FileService } from '../../bindings/zashiki'
import type { FileEntry } from '../../bindings/zashiki'
import { CloseSharp, ArrowBackRound, ArrowForwardRound } from '@vicons/material'
import { SplitVertical28Regular, SplitHorizontal28Regular, FolderArrowUp24Regular } from '@vicons/fluent'

const props = defineProps<{
  path: string
  closable?: boolean
}>()

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
    loading.value = true
    errorMsg.value = ''
    pathError.value = false
    entries.value = []
    FileService.ListDir(newPath).then((result) => {
      console.log('ListDir', newPath, '→', result?.length, 'entries')
      entries.value = result || []
    }).catch((err) => {
      console.error('ListDir failed:', newPath, err)
      errorMsg.value = friendlyError(err, newPath)
      pathError.value = true
    }).finally(() => {
      loading.value = false
    })
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

const parentPath = computed(() => {
  const p = props.path
  if (!p || p === '/') return null
  return p.split('/').slice(0, -1).join('/') || '/'
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
    <div class="table-area">
      <NAlert v-if="errorMsg" type="error" :title="errorMsg" class="error-alert" />
      <NSpin v-else-if="loading" class="spin-fill" />
      <NDataTable
        v-else-if="entries.length > 0"
        :columns="columns"
        :data="entries"
        :row-key="(row: FileEntry) => row.path"
        :row-props="(row: FileEntry) => ({
          style: 'cursor: pointer',
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
</style>
