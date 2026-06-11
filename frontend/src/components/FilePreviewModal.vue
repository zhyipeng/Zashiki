<script setup lang="ts">
import { computed } from 'vue'
import { NAlert, NEmpty, NModal, NSpin, NTag } from 'naive-ui'
import type { FileEntry, FilePreview } from '../../bindings/zashiki/internal/filemanager'
import { formatPreviewSize, resolvePreviewRenderer } from './preview'

const props = defineProps<{
  show: boolean
  entry: FileEntry | null
  preview: FilePreview | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  'update:show': [show: boolean]
}>()

const renderer = computed(() => resolvePreviewRenderer(props.preview))
const title = computed(() => props.entry?.name || '预览')
const sizeText = computed(() => formatPreviewSize(props.preview?.size ?? props.entry?.size ?? 0))

function onUpdateShow(show: boolean) {
  emit('update:show', show)
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    :title="title"
    class="file-preview-modal"
    style="width: min(980px, 92vw)"
    :on-update:show="onUpdateShow"
  >
    <div class="preview-shell">
      <div class="preview-meta">
        <NTag size="small" :bordered="false">{{ renderer.label }}</NTag>
        <span>{{ sizeText }}</span>
        <span v-if="preview?.mimeType">{{ preview.mimeType }}</span>
        <span v-if="preview?.truncated">已截断</span>
      </div>
      <div class="preview-stage">
        <NSpin v-if="loading" class="preview-spin" />
        <NAlert v-else-if="error" type="error" :title="error" />
        <template v-else-if="preview">
          <img
            v-if="renderer.kind === 'image' && preview.dataUrl"
            class="preview-image"
            :src="preview.dataUrl"
            :alt="preview.name"
          >
          <pre v-else-if="renderer.kind === 'text'" class="preview-text">{{ preview.content }}</pre>
          <NEmpty v-else :description="preview.message || '暂不支持此文件类型预览'" />
        </template>
        <NEmpty v-else description="暂无预览" />
      </div>
    </div>
  </NModal>
</template>

<style scoped>
.preview-shell {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
}

.preview-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--n-text-color-3);
  font-size: 12px;
  line-height: 18px;
  flex-wrap: wrap;
}

.preview-stage {
  min-height: 320px;
  max-height: min(70vh, 720px);
  overflow: auto;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  background: var(--n-color);
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-spin {
  min-height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-image {
  display: block;
  max-width: 100%;
  max-height: min(68vh, 700px);
  object-fit: contain;
}

.preview-text {
  width: 100%;
  min-height: 320px;
  box-sizing: border-box;
  margin: 0;
  padding: 12px;
  align-self: stretch;
  color: var(--n-text-color);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
