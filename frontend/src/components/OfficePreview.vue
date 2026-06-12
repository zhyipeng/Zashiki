<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { NButton, NButtonGroup, NSpin, NAlert, NText } from 'naive-ui'
import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'

const props = defineProps<{
  preview: FilePreview
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const containerRef = ref<HTMLDivElement | null>(null)
const loadingState = ref<'loading' | 'ready' | 'error'>('loading')
const errorMsg = ref('')
const pageIndex = ref(0)
const pageTotal = ref(1)

const viewerType = computed<'docx' | 'xlsx' | 'pptx'>(() => {
  const name = props.preview.name.toLowerCase()
  if (name.endsWith('.xlsx')) return 'xlsx'
  if (name.endsWith('.pptx')) return 'pptx'
  return 'docx'
})

const hasNavigation = computed(() => viewerType.value === 'docx' || viewerType.value === 'pptx')
const pageLabel = computed(() => {
  if (viewerType.value === 'docx') return `${pageIndex.value + 1} / ${pageTotal.value} 页`
  if (viewerType.value === 'pptx') return `${pageIndex.value + 1} / ${pageTotal.value} 幻灯片`
  return ''
})

let viewer: any = null
let resizeObserver: ResizeObserver | null = null

function dataUrlToArrayBuffer(dataUrl: string): ArrayBuffer {
  const commaIdx = dataUrl.indexOf(',')
  if (commaIdx === -1) throw new Error('无效的数据格式')
  const base64 = dataUrl.substring(commaIdx + 1)
  const binaryStr = atob(base64)
  const bytes = new Uint8Array(binaryStr.length)
  for (let i = 0; i < binaryStr.length; i++) {
    bytes[i] = binaryStr.charCodeAt(i)
  }
  return bytes.buffer
}

function onPageChange(index: number, total: number) {
  pageIndex.value = index
  pageTotal.value = total
}

function onError(err: Error) {
  loadingState.value = 'error'
  errorMsg.value = err.message || '加载失败'
}

async function initDocxViewer(canvas: HTMLCanvasElement, data: ArrayBuffer) {
  const { DocxViewer } = await import('@silurus/ooxml/docx')
  viewer = new DocxViewer(canvas, {
    onPageChange,
    onError,
  })
  await viewer.load(data)
  pageTotal.value = viewer.pageCount
}

async function initPptxViewer(canvas: HTMLCanvasElement, data: ArrayBuffer) {
  const { PptxViewer } = await import('@silurus/ooxml/pptx')
  viewer = new PptxViewer(canvas, {
    onSlideChange: onPageChange,
    onError,
  })
  await viewer.load(data)
  pageTotal.value = viewer.slideCount
}

async function initXlsxViewer(container: HTMLElement, data: ArrayBuffer) {
  const { XlsxViewer } = await import('@silurus/ooxml/xlsx')
  viewer = new XlsxViewer(container, {
    onReady: () => {
      loadingState.value = 'ready'
    },
    onError,
  })
  await viewer.load(data)
  if (loadingState.value === 'loading') {
    loadingState.value = 'ready'
  }
}

async function initViewer() {
  if (!props.preview.dataUrl) {
    loadingState.value = 'error'
    errorMsg.value = '没有文件数据'
    return
  }

  loadingState.value = 'loading'
  errorMsg.value = ''

  try {
    const arrayBuffer = dataUrlToArrayBuffer(props.preview.dataUrl)
    await nextTick()

    if (viewerType.value === 'xlsx') {
      if (!containerRef.value) return
      await initXlsxViewer(containerRef.value, arrayBuffer)
    } else {
      if (!canvasRef.value) return

      // Size the canvas to fill its parent
      const parent = canvasRef.value.parentElement
      if (parent) {
        canvasRef.value.width = parent.clientWidth * devicePixelRatio
        canvasRef.value.height = parent.clientHeight * devicePixelRatio
        canvasRef.value.style.width = `${parent.clientWidth}px`
        canvasRef.value.style.height = `${parent.clientHeight}px`
      }

      if (viewerType.value === 'pptx') {
        await initPptxViewer(canvasRef.value, arrayBuffer)
      } else {
        await initDocxViewer(canvasRef.value, arrayBuffer)
      }
    }

    loadingState.value = 'ready'
  } catch (e: any) {
    loadingState.value = 'error'
    errorMsg.value = e.message || '预览加载失败'
  }
}

function resizeCanvas() {
  if (!canvasRef.value) return
  const parent = canvasRef.value.parentElement
  if (!parent) return
  canvasRef.value.width = parent.clientWidth * devicePixelRatio
  canvasRef.value.height = parent.clientHeight * devicePixelRatio
  canvasRef.value.style.width = `${parent.clientWidth}px`
  canvasRef.value.style.height = `${parent.clientHeight}px`
}

function goPrev() {
  if (!viewer) return
  if (viewerType.value === 'pptx') viewer.prevSlide()
  else viewer.prevPage()
}

function goNext() {
  if (!viewer) return
  if (viewerType.value === 'pptx') viewer.nextSlide()
  else viewer.nextPage()
}

function destroyViewer() {
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (viewer) {
    viewer.destroy()
    viewer = null
  }
}

onMounted(() => {
  initViewer()

  // Observe container resize for canvas viewers
  if (canvasRef.value) {
    resizeObserver = new ResizeObserver(() => {
      resizeCanvas()
    })
    resizeObserver.observe(canvasRef.value.parentElement!)
  }
})

onUnmounted(() => {
  destroyViewer()
})
</script>

<template>
  <div class="office-preview-root" :class="{ 'is-xlsx': viewerType === 'xlsx' }">
    <div
      v-if="hasNavigation && loadingState === 'ready'"
      class="office-nav"
    >
      <NButtonGroup size="tiny">
        <NButton :disabled="pageIndex <= 0" @click="goPrev">上一{{ viewerType === 'pptx' ? '页' : '页' }}</NButton>
        <NButton disabled class="page-indicator">
          <NText depth="3">{{ pageLabel }}</NText>
        </NButton>
        <NButton :disabled="pageIndex >= pageTotal - 1" @click="goNext">下一{{ viewerType === 'pptx' ? '页' : '页' }}</NButton>
      </NButtonGroup>
    </div>

    <div class="office-stage">
      <NSpin v-if="loadingState === 'loading'" class="office-spin" />
      <NAlert
        v-else-if="loadingState === 'error'"
        type="error"
        :title="errorMsg"
      />
      <div
        v-show="loadingState === 'ready'"
        v-if="viewerType === 'xlsx'"
        ref="containerRef"
        class="office-xlsx-container"
      />
      <canvas
        v-show="loadingState === 'ready'"
        v-else
        ref="canvasRef"
        class="office-canvas"
      />
    </div>
  </div>
</template>

<style scoped>
.office-preview-root {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  width: 100%;
  min-height: 0;
}

.is-xlsx {
  overflow: hidden;
}

.office-nav {
  display: flex;
  justify-content: center;
  flex-shrink: 0;
}

.page-indicator {
  cursor: default !important;
  min-width: 100px;
  justify-content: center;
}

.office-stage {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
}

.office-spin {
  min-height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.office-xlsx-container {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.office-canvas {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
</style>
