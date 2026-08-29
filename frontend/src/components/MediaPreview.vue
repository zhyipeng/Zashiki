<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NEmpty } from 'naive-ui'
import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'
import { mediaAssetUrl } from './assetUrl'

const props = defineProps<{
  preview: FilePreview
}>()

const isVideo = computed(() => props.preview.kind === 'video')
const src = computed(() => mediaAssetUrl(props.preview.path))
const loadFailed = ref(false)

watch(() => props.preview.path, () => {
  loadFailed.value = false
})

function onMediaError() {
  loadFailed.value = true
}
</script>

<template>
  <div class="media-preview" :class="{ 'is-audio': !isVideo }">
    <NEmpty
      v-if="loadFailed"
      class="media-fallback"
      description="浏览器不支持此媒体编码格式，无法预览"
    />
    <video
      v-else-if="isVideo"
      :key="src"
      class="media-video"
      :src="src"
      :title="preview.name"
      controls
      autoplay
      preload="metadata"
      @error="onMediaError"
    />
    <audio
      v-else
      :key="src"
      :src="src"
      :title="preview.name"
      controls
      autoplay
      preload="metadata"
      @error="onMediaError"
    />
  </div>
</template>

<style scoped>
.media-preview {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.media-video {
  display: block;
  width: 100%;
  max-height: min(62vh, 620px);
  background: #000;
  border-radius: 4px;
}

.media-preview.is-audio audio {
  width: min(560px, 90%);
}

.media-fallback {
  padding: 24px;
}
</style>
