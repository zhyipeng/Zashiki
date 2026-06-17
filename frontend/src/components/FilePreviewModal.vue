<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NAlert, NButton, NEmpty, NInput, NModal, NSpin, NSpace, NSwitch, NTag } from 'naive-ui'
import type { FileEntry, FilePreview } from '../../bindings/zashiki/internal/filemanager'
import { formatPreviewSize, isFormattedJsonPreview, isHtmlPreview, isMarkdownPreview, previewTextContent, resolvePreviewRenderer } from './preview'
import HtmlPreview from './HtmlPreview.vue'
import OfficePreview from './OfficePreview.vue'
import PdfPreview from './PdfPreview.vue'

const props = defineProps<{
  show: boolean
  entry: FileEntry | null
  preview: FilePreview | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  'update:show': [show: boolean]
  save: [content: string]
}>()

const renderer = computed(() => resolvePreviewRenderer(props.preview))
const title = computed(() => props.entry?.name || '预览')
const sizeText = computed(() => formatPreviewSize(props.preview?.size ?? props.entry?.size ?? 0))
const editMode = ref(false)
const draftContent = ref('')
const mdViewMode = ref<'preview' | 'source'>('preview')
const htmlViewMode = ref<'preview' | 'source'>('preview')
const canEdit = computed(() => props.preview?.kind === 'text' && !props.preview.truncated && !props.loading && !props.error)
const hasChanges = computed(() => draftContent.value !== (props.preview?.content || ''))
const displayedTextContent = computed(() => previewTextContent(props.preview))
const jsonFormatted = computed(() => isFormattedJsonPreview(props.preview))
const isMarkdown = computed(() => isMarkdownPreview(props.preview))
const isHtml = computed(() => isHtmlPreview(props.preview))
const markdownModule = ref<typeof import('marked') | null>(null)
const markdownLoaded = ref(false)

const renderedMarkdown = computed(() => {
  if (!isMarkdown.value || !props.preview?.content || !markdownModule.value) return ''
  return markdownModule.value.marked.parse(props.preview.content, { async: false }) as string
})

watch(isMarkdown, async (val) => {
  if (val && !markdownLoaded.value) {
    markdownModule.value = await import('marked')
    markdownLoaded.value = true
  }
}, { immediate: true })

watch(() => props.preview, (preview) => {
  editMode.value = false
  mdViewMode.value = 'preview'
  htmlViewMode.value = 'preview'
  draftContent.value = preview?.kind === 'text' ? preview.content : ''
})

watch(() => props.show, (show) => {
  if (!show) {
    editMode.value = false
    mdViewMode.value = 'preview'
    htmlViewMode.value = 'preview'
  }
})

function onUpdateShow(show: boolean) {
  emit('update:show', show)
}

function enterEditMode() {
  if (editMode.value || !canEdit.value || !props.preview) return
  if (isMarkdown.value) mdViewMode.value = 'source'
  draftContent.value = props.preview.content
  editMode.value = true
}

function onPreviewStageDblclick(event: MouseEvent) {
  const target = event.target as HTMLElement | null
  if (editMode.value || target?.closest('textarea, input, button')) return
  enterEditMode()
}

function cancelEditMode() {
  draftContent.value = props.preview?.content || ''
  editMode.value = false
}

function saveEdit() {
  if (!canEdit.value || !hasChanges.value) return
  emit('save', draftContent.value)
}

function saveEditFromKeyboard(event: KeyboardEvent) {
  event.preventDefault()
  event.stopPropagation()
  saveEdit()
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
        <span v-if="jsonFormatted">已格式化预览</span>
        <NSpace v-if="isMarkdown && !editMode" class="preview-actions" size="small" align="center">
          <span>预览</span>
          <NSwitch size="small" :value="mdViewMode === 'source'" @update:value="(v: boolean) => mdViewMode = v ? 'source' : 'preview'" />
          <span>源码</span>
        </NSpace>
        <NSpace v-if="isHtml && !editMode" class="preview-actions" size="small" align="center">
          <span>预览</span>
          <NSwitch size="small" :value="htmlViewMode === 'source'" @update:value="(v: boolean) => htmlViewMode = v ? 'source' : 'preview'" />
          <span>源码</span>
        </NSpace>
        <NSpace v-if="canEdit" class="preview-actions" size="small">
          <NButton v-if="!editMode" size="tiny" @click="enterEditMode">编辑</NButton>
          <template v-else>
            <NButton size="tiny" :disabled="loading" @click="cancelEditMode">取消</NButton>
            <NButton size="tiny" type="primary" :loading="loading" :disabled="!hasChanges" @click="saveEdit">保存</NButton>
          </template>
        </NSpace>
      </div>
      <div
        class="preview-stage"
        :class="{
          'text-preview-stage': renderer.kind === 'text',
          'is-office': renderer.kind === 'office',
          'is-pdf': renderer.kind === 'pdf',
          'is-html': renderer.kind === 'html',
        }"
        @dblclick="onPreviewStageDblclick"
      >
        <NSpin v-if="loading" class="preview-spin" />
        <NAlert v-else-if="error" type="error" :title="error" />
        <template v-else-if="preview">
          <img
            v-if="renderer.kind === 'image' && preview.dataUrl"
            class="preview-image"
            :src="preview.dataUrl"
            :alt="preview.name"
          >
          <NInput
            v-else-if="renderer.kind === 'text' && editMode"
            v-model:value="draftContent"
            class="preview-editor"
            type="textarea"
            :autosize="false"
            @keydown.ctrl.s="saveEditFromKeyboard"
            @keydown.meta.s="saveEditFromKeyboard"
          />
          <div v-else-if="renderer.kind === 'text' && isMarkdown && mdViewMode === 'preview'" class="preview-markdown" v-html="renderedMarkdown" />
          <pre v-else-if="renderer.kind === 'text'" class="preview-text">{{ displayedTextContent }}</pre>
          <pre v-else-if="renderer.kind === 'html' && htmlViewMode === 'source'" class="preview-text">{{ displayedTextContent }}</pre>
          <HtmlPreview v-else-if="renderer.kind === 'html' && preview" :preview="preview" />
          <OfficePreview v-else-if="renderer.kind === 'office' && preview" :preview="preview" />
          <PdfPreview v-else-if="renderer.kind === 'pdf' && preview" :preview="preview" />
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

.preview-actions {
  margin-left: auto;
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

.preview-stage.is-office {
  height: min(68vh, 700px);
  max-height: none;
  align-items: stretch;
  justify-content: stretch;
  overflow: hidden;
  padding: 0;
}

.preview-stage.is-pdf {
  height: min(68vh, 700px);
  max-height: none;
  align-items: stretch;
  justify-content: stretch;
  overflow: hidden;
  padding: 0;
}

.preview-stage.is-html {
  height: min(68vh, 700px);
  max-height: none;
  align-items: stretch;
  justify-content: stretch;
  overflow: hidden;
  padding: 0;
}

.text-preview-stage {
  height: min(68vh, 700px);
  align-items: stretch;
  justify-content: stretch;
  overflow: hidden;
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
  height: 100%;
  box-sizing: border-box;
  margin: 0;
  padding: 12px;
  align-self: stretch;
  overflow: auto;
  color: var(--n-text-color);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  cursor: text;
  user-select: text;
  -webkit-user-select: text;
}

.preview-editor {
  align-self: stretch;
  width: 100%;
  height: 100%;
}

.preview-editor :deep(textarea) {
  height: 100% !important;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.55;
}

.preview-editor :deep(.n-input-wrapper),
.preview-editor :deep(.n-input__textarea) {
  height: 100%;
}

.preview-markdown {
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  padding: 16px;
  align-self: stretch;
  overflow: auto;
  color: var(--n-text-color);
  font-size: 14px;
  line-height: 1.6;
}

.preview-markdown :deep(h1),
.preview-markdown :deep(h2),
.preview-markdown :deep(h3),
.preview-markdown :deep(h4),
.preview-markdown :deep(h5),
.preview-markdown :deep(h6) {
  margin: 16px 0 8px;
  line-height: 1.4;
}

.preview-markdown :deep(p) {
  margin: 0 0 12px;
}

.preview-markdown :deep(ul),
.preview-markdown :deep(ol) {
  margin: 0 0 12px;
  padding-left: 24px;
}

.preview-markdown :deep(code) {
  padding: 2px 6px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.9em;
}

.preview-markdown :deep(pre) {
  margin: 0 0 12px;
  padding: 12px;
  border-radius: 6px;
  overflow: auto;
}

.preview-markdown :deep(pre code) {
  padding: 0;
  font-size: 13px;
  line-height: 1.55;
}

.preview-markdown :deep(blockquote) {
  margin: 0 0 12px;
  padding: 4px 16px;
  border-left: 4px solid var(--n-border-color);
}

.preview-markdown :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 12px;
}

.preview-markdown :deep(th),
.preview-markdown :deep(td) {
  padding: 6px 12px;
  border: 1px solid var(--n-border-color);
}

.preview-markdown :deep(img) {
  max-width: 100%;
}

.preview-markdown :deep(a) {
  color: var(--n-primary-color);
}

.preview-markdown :deep(hr) {
  border: none;
  border-top: 1px solid var(--n-border-color);
  margin: 16px 0;
}
</style>
