<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { NAlert, NButton, NDataTable, NInput, NModal, NPagination, NSpace, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import type { CancellablePromise } from '@wailsio/runtime'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import type { ArchiveEntry, ArchivePage, FilePreview } from '../../bindings/zashiki/internal/filemanager'
import { isOperationCancelledError, trackOperationPromise } from '../composables/useOperationProgress'
import { archiveFolderName, archiveParent } from './archive'
import { joinPath, parentPath } from './path'
import { formatPreviewSize } from './preview'
import FilePreviewModal from './FilePreviewModal.vue'

const props = defineProps<{ show: boolean; archivePath: string; name: string; separator: string }>()
const emit = defineEmits<{ 'update:show': [show: boolean]; extracted: [directory: string] }>()
const message = useMessage()
const directory = ref('')
const page = ref(1)
const entries = ref<ArchiveEntry[]>([])
const total = ref(0)
const selected = ref<string[]>([])
const loading = ref(false)
const error = ref('')
const extracting = ref(false)
const destination = ref('')
const preview = ref<FilePreview | null>(null)
const previewOpen = ref(false)
const previewLoading = ref(false)
const previewError = ref('')
let requestId = 0
let previewRequestId = 0
let pendingList: CancellablePromise<ArchivePage> | null = null
let pendingPreview: CancellablePromise<FilePreview> | null = null
const pageSize = 200
const columns: DataTableColumns<ArchiveEntry> = [
  { type: 'selection', disabled: row => row.isLink },
  { title: '名称', key: 'name', render: row => `${row.isDir ? '📁 ' : ''}${row.name}${row.isLink ? '（链接）' : ''}` },
  { title: '大小', key: 'size', width: 110, render: row => row.isDir ? '—' : formatPreviewSize(row.size) },
]

function cancelReads() {
  requestId++
  previewRequestId++
  pendingList?.cancel()
  pendingPreview?.cancel()
  pendingList = null
  pendingPreview = null
}

async function load() {
  const id = ++requestId
  pendingList?.cancel()
  loading.value = true
  error.value = ''
  entries.value = []
  selected.value = []
  try {
    pendingList = FileService.ListArchive(props.archivePath, directory.value, (page.value - 1) * pageSize, pageSize)
    const result = await pendingList
    if (id !== requestId) return
    entries.value = result.entries
    total.value = result.total
  } catch (err) {
    if (id === requestId && !isOperationCancelledError(err)) error.value = String(err)
  } finally {
    if (id === requestId) { loading.value = false; pendingList = null }
  }
}

watch(() => [props.show, props.archivePath] as const, ([show]) => {
  cancelReads()
  previewOpen.value = false
  if (!show) return
  directory.value = ''
  page.value = 1
  destination.value = joinPath(parentPath(props.archivePath, props.separator) || '', archiveFolderName(props.name), props.separator)
  void load()
}, { immediate: true })
onBeforeUnmount(cancelReads)

function navigate(path: string) {
  directory.value = path
  page.value = 1
  void load()
}

async function openEntry(entry: ArchiveEntry) {
  if (entry.isLink) return
  if (entry.isDir) { navigate(entry.path); return }
  const id = ++previewRequestId
  pendingPreview?.cancel()
  previewOpen.value = true
  preview.value = null
  previewLoading.value = true
  previewError.value = ''
  try {
    pendingPreview = FileService.GetArchiveEntryPreview(props.archivePath, entry.path)
    const result = await pendingPreview
    if (id === previewRequestId) preview.value = result
  } catch (err) {
    if (id === previewRequestId && !isOperationCancelledError(err)) previewError.value = String(err)
  } finally {
    if (id === previewRequestId) { previewLoading.value = false; pendingPreview = null }
  }
}

function closePreview(show: boolean) {
  previewOpen.value = show
  if (!show) { previewRequestId++; pendingPreview?.cancel(); pendingPreview = null }
}

async function extract(all: boolean) {
  if (extracting.value || !destination.value.trim()) return
  extracting.value = true
  error.value = ''
  const archive = props.archivePath
  const output = destination.value.trim()
  const selection = all ? [] : selected.value.slice()
  try {
    const result = await trackOperationPromise(FileService.ExtractArchive(archive, output, selection))
    message.success('解压完成')
    emit('extracted', parentPath(result, props.separator) || result)
  } catch (err) {
    if (!isOperationCancelledError(err)) error.value = String(err)
  } finally {
    extracting.value = false
  }
}
</script>

<template>
  <NModal :show="show" preset="card" :title="`压缩包 · ${name}`" style="width: min(900px, 94vw)" :mask-closable="!extracting" :closable="!extracting" :close-on-esc="!extracting" @update:show="emit('update:show', $event)">
    <div class="archive-browser">
      <NSpace align="center">
        <NButton size="small" :disabled="!directory || loading" @click="navigate(archiveParent(directory))">上一级</NButton>
        <NButton size="small" :disabled="loading" @click="navigate('')">根目录</NButton>
        <span>{{ directory || '/' }}</span>
      </NSpace>
      <NAlert v-if="error" type="error">{{ error }}</NAlert>
      <NDataTable
        :columns="columns" :data="entries" :loading="loading" :row-key="(row: ArchiveEntry) => row.path"
        :row-props="(row: ArchiveEntry) => ({ onDblclick: () => openEntry(row) })"
        :checked-row-keys="selected" :max-height="390" virtual-scroll
        @update:checked-row-keys="(keys: Array<string | number>) => selected = keys.map(String)"
      />
      <NPagination :page="page" :page-size="pageSize" :item-count="total" :disabled="loading" @update:page="(value: number) => { page = value; load() }" />
      <span class="archive-hint">双击目录进入，双击文件预览文本或图片。链接与特殊文件不解压。</span>
      <label for="archive-destination">解压到新文件夹（路径必须尚不存在）</label>
      <NInput id="archive-destination" v-model:value="destination" :disabled="extracting" placeholder="新文件夹完整路径" />
      <NSpace justify="end">
        <NButton :disabled="extracting || !selected.length || !destination.trim()" @click="extract(false)">解压选中项（{{ selected.length }}）</NButton>
        <NButton type="primary" :loading="extracting" :disabled="extracting || loading || !destination.trim()" @click="extract(true)">全部解压</NButton>
      </NSpace>
    </div>
    <FilePreviewModal :show="previewOpen" :entry="null" :preview="preview" :loading="previewLoading" :error="previewError" readonly @update:show="closePreview" />
  </NModal>
</template>

<style scoped>
.archive-browser { display: flex; flex-direction: column; gap: 12px; }
.archive-hint { opacity: .65; font-size: 12px; }
</style>
