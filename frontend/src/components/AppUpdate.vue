<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { NModal, NButton, NProgress, NAlert } from 'naive-ui'
import { useAppUpdate } from '../composables/useAppUpdate'

const { status, visible, busy, error, percent, ready, install, dismiss, start } = useAppUpdate()
let cleanup: (() => void) | undefined
onMounted(() => { cleanup = start() })
onUnmounted(() => cleanup?.())
</script>

<template>
  <NModal :show="visible" preset="card" title="发现新版本" style="width: 560px; max-width: 90vw"
    :closable="!busy" :mask-closable="!busy" :close-on-esc="!busy"
    @update:show="(show: boolean) => { if (!show) dismiss() }">
    <p>当前版本 {{ status.currentVersion }} → {{ status.version }}</p>
    <p v-if="status.size">下载大小：{{ (status.size / 1024 / 1024).toFixed(1) }} MB</p>
    <pre v-if="status.notes" class="release-notes">{{ status.notes }}</pre>
    <p>确认后下载并校验更新，完成后应用会自动重启。请先等待文件操作和传输结束。</p>
    <NProgress v-if="busy" type="line" :percentage="percent ?? 0" :processing="true" :show-indicator="percent !== null" />
    <p v-if="busy">{{ ready ? '正在重启…' : '正在下载、校验并准备更新…' }}</p>
    <NAlert v-if="error" type="error" style="margin-top: 12px">{{ error }}</NAlert>
    <template #footer>
      <div class="update-actions">
        <NButton :disabled="busy" @click="dismiss">稍后提醒</NButton>
        <NButton type="primary" :loading="busy" @click="install">{{ ready ? '重试重启安装' : '下载并重启安装' }}</NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.release-notes { max-height: 260px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; font: inherit; }
.update-actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
