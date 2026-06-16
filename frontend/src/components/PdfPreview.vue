<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { NButton, NButtonGroup, NSpin, NAlert, NText } from 'naive-ui'
import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'

const VuePdfEmbed = defineAsyncComponent(() => import('vue-pdf-embed'))

const props = defineProps<{
  preview: FilePreview
}>()

const loadingState = ref<'loading' | 'ready' | 'error'>('loading')
const errorMsg = ref('')
const pageIndex = ref(1)
const pageTotal = ref(1)

const pageLabel = computed(() => `${pageIndex.value} / ${pageTotal.value} 页`)

const pdfSource = computed(() => {
  if (!props.preview.dataUrl) return null
  return {
    url: props.preview.dataUrl,
    cMapUrl: 'https://unpkg.com/pdfjs-dist/cmaps/',
    cMapPacked: true,
  }
})

function onPdfLoaded(pdf: any) {
  pageTotal.value = pdf.numPages
  loadingState.value = 'ready'
}

function onPdfLoadingFailed(err: Error) {
  loadingState.value = 'error'
  errorMsg.value = err.message || 'PDF 加载失败'
}

function goPrev() {
  if (pageIndex.value > 1) pageIndex.value--
}

function goNext() {
  if (pageIndex.value < pageTotal.value) pageIndex.value++
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
    event.preventDefault()
    goPrev()
  } else if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
    event.preventDefault()
    goNext()
  }
}

onMounted(() => {
  if (!props.preview.dataUrl) {
    loadingState.value = 'error'
    errorMsg.value = '没有文件数据'
  }
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="pdf-preview-root">
    <div
      v-if="loadingState === 'ready' && pageTotal > 1"
      class="pdf-nav"
    >
      <NButtonGroup size="tiny">
        <NButton :disabled="pageIndex <= 1" @click="goPrev">上一页</NButton>
        <NButton disabled class="page-indicator">
          <NText depth="3">{{ pageLabel }}</NText>
        </NButton>
        <NButton :disabled="pageIndex >= pageTotal" @click="goNext">下一页</NButton>
      </NButtonGroup>
    </div>

    <div class="pdf-stage">
      <NSpin v-if="loadingState === 'loading'" class="pdf-spin" />
      <NAlert
        v-else-if="loadingState === 'error'"
        type="error"
        :title="errorMsg"
      />
      <div
        v-show="loadingState === 'ready'"
        class="pdf-container"
      >
        <VuePdfEmbed
          v-if="pdfSource"
          :source="pdfSource"
          :page="pageIndex"
          @loaded="onPdfLoaded"
          @loading-failed="onPdfLoadingFailed"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.pdf-preview-root {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  width: 100%;
  min-height: 0;
}

.pdf-nav {
  display: flex;
  justify-content: center;
  flex-shrink: 0;
}

.page-indicator {
  cursor: default !important;
  min-width: 100px;
  justify-content: center;
}

.pdf-stage {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  position: relative;
  overflow: auto;
}

.pdf-spin {
  min-height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.pdf-container {
  width: 100%;
  overflow: auto;
}
</style>