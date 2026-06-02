<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { NDataTable, NButton, NText, NSpin, NSpace, NEmpty, NAlert } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { FileService } from '../../bindings/zashiki'
import type { FileEntry } from '../../bindings/zashiki'

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

watch(() => props.path, loadDir, { immediate: true })

async function loadDir() {
  if (!props.path) return
  loading.value = true
  errorMsg.value = ''
  entries.value = []
  try {
    const result = await FileService.ListDir(props.path)
    console.log('ListDir', props.path, '→', result?.length, 'entries')
    entries.value = result || []
  } catch (err) {
    console.error('ListDir failed:', props.path, err)
    errorMsg.value = String(err)
  } finally {
    loading.value = false
  }
}

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

const parentPath = computed(() => {
  const p = props.path
  if (!p || p === '/') return null
  return p.split('/').slice(0, -1).join('/') || '/'
})

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
      errorMsg.value = String(err)
    }
  }
}
</script>

<template>
  <div class="file-table">
    <div class="toolbar">
      <div class="toolbar-left">
        <NButton
          v-if="closable"
          text
          size="tiny"
          @click="emit('close')"
        >
          &times;
        </NButton>
        <NButton
          v-if="parentPath"
          text
          @click="emit('navigate', parentPath)"
        >
          &larr;
        </NButton>
        <NText class="path-text">{{ path }}</NText>
      </div>
      <div class="toolbar-right">
        <NButton
          text
          size="tiny"
          title="竖直分屏"
          @click="emit('splitV')"
        >
          ⬌
        </NButton>
        <NButton
          text
          size="tiny"
          title="水平分屏"
          @click="emit('splitH')"
        >
          ⬍
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
  gap: 2px;
  flex-shrink: 0;
}

.path-text {
  font-size: 13px;
  font-family: monospace;
  word-break: break-all;
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
