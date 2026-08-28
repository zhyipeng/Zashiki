<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NButton,
  NEmpty,
  NIcon,
  NInput,
  NProgress,
  NQrCode,
  NTag,
  NTooltip,
  useMessage,
} from 'naive-ui'
import { FolderOpen20Regular, ClipboardPaste20Regular } from '@vicons/fluent'
import { Dialogs, Clipboard } from '@wailsio/runtime'
import { useLanShare, transferPercent } from '../../composables/useLanShare'
import { formatBytes } from '../../composables/useOperationProgress'
import type { ShareItem } from '../../../bindings/zashiki/internal/lanshare/models'
import type { TransferState } from '../../composables/useLanShare'

const message = useMessage()
const {
  status,
  items,
  transfers,
  receivedTexts,
  pendingShareRequest,
  primaryUrl,
  refreshStatus,
  ensureServer,
  addFiles,
  addText,
  removeItem,
  clearItems,
  stopServer,
  consumeShareRequest,
} = useLanShare()

const draftText = ref('')
const sharingText = ref(false)
const starting = ref(false)

const running = computed(() => status.value?.running ?? false)
const primaryUrlValue = computed(() => primaryUrl())
const fileCount = computed(() => items.value.filter(item => item.kind === 'file').length)

onMounted(() => {
  if (status.value === null) refreshStatus()
})

// FileTable 快速入口：消费排队的分享请求并加入分享列表。
watch(pendingShareRequest, async (request) => {
  if (!request) return
  const queued = consumeShareRequest()
  if (!queued || queued.paths.length === 0) return
  try {
    const added = await addFiles(queued.paths)
    message.success(`已加入 ${added?.length ?? 0} 项到快传`)
  } catch (err) {
    message.error(`分享失败：${friendlyError(err)}`)
  }
})

async function startServer() {
  if (starting.value) return
  starting.value = true
  try {
    await ensureServer()
  } catch (err) {
    message.error(`启动快传服务失败：${friendlyError(err)}`)
  } finally {
    starting.value = false
  }
}

async function pickFiles() {
  try {
    const result = await Dialogs.OpenFile({
      Title: '选择要分享的文件',
      CanChooseFiles: true,
      CanChooseDirectories: true,
      AllowsMultipleSelection: true,
      TreatsFilePackagesAsDirectories: false,
    })
    const paths = Array.isArray(result) ? result : (result ? [result] : [])
    if (paths.length === 0) return
    const added = await addFiles(paths)
    message.success(`已加入 ${added?.length ?? 0} 个文件`)
  } catch (err) {
    message.error(`添加文件失败：${friendlyError(err)}`)
  }
}

async function shareDraftText() {
  const text = draftText.value
  if (!text.trim()) return
  sharingText.value = true
  try {
    await addText(text)
    draftText.value = ''
    message.success('文本已加入分享')
  } catch (err) {
    message.error(`分享文本失败：${friendlyError(err)}`)
  } finally {
    sharingText.value = false
  }
}

async function shareClipboard() {
  try {
    const text = await Clipboard.Text()
    if (!text || !text.trim()) {
      message.warning('剪贴板中没有文本内容')
      return
    }
    await addText(text)
    message.success('剪贴板内容已加入分享')
  } catch (err) {
    message.error(`读取剪贴板失败：${friendlyError(err)}`)
  }
}

async function copyText(text: string, tip: string) {
  try {
    await Clipboard.SetText(text)
    message.success(tip)
  } catch (err) {
    message.error(`复制失败：${friendlyError(err)}`)
  }
}

async function onClear() {
  try {
    await clearItems()
  } catch (err) {
    message.error(`清空失败：${friendlyError(err)}`)
  }
}

async function onStop() {
  try {
    await stopServer()
    message.success('快传服务已停止')
  } catch (err) {
    message.error(`停止失败：${friendlyError(err)}`)
  }
}

function itemSizeText(item: ShareItem): string {
  return item.kind === 'text' ? `${formatBytes(item.size)} 文本` : formatBytes(item.size)
}

function transferStatusText(state: TransferState): string {
  if (state.phase === 'error') return state.error || '传输失败'
  if (state.phase === 'done') return '完成'
  const percent = transferPercent(state)
  return percent === null ? '传输中…' : `${percent}%`
}

function transferProgressText(state: TransferState): string {
  if (state.totalBytes > 0) {
    return `${formatBytes(state.doneBytes)} / ${formatBytes(state.totalBytes)}`
  }
  return ''
}

function friendlyError(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}
</script>

<template>
  <div class="transfer-page">
    <div class="transfer-columns">
      <div class="transfer-col">
        <div class="transfer-card">
          <div class="transfer-card-title">
            <span>连接</span>
            <NButton v-if="running" size="tiny" quaternary type="error" @click="onStop">
              停止服务
            </NButton>
          </div>
          <template v-if="running && primaryUrlValue">
            <div class="connect-body">
              <NQrCode :value="primaryUrlValue" :size="148" error-correction-level="M" />
              <div class="connect-info">
                <div class="connect-input-row">
                  <NInput :value="primaryUrlValue" size="small" readonly />
                  <NButton size="small" @click="copyText(primaryUrlValue, '已复制链接')">
                    复制
                  </NButton>
                </div>
                <div v-if="(status?.urls?.length ?? 0) > 1" class="connect-alt">
                  <div v-for="url in status?.urls" :key="url" class="connect-alt-item">{{ url }}</div>
                </div>
                <div class="connect-meta">
                  端口 {{ status?.port }} · 已共享 {{ items.length }} 项（{{ fileCount }} 个文件）
                </div>
                <div class="connect-warn">同一局域网内任何人拿到此链接即可访问，用完请停止服务</div>
              </div>
            </div>
          </template>
          <NEmpty v-else description="快传服务未启动" class="connect-empty">
            <template #extra>
              <NButton type="primary" size="small" :loading="starting" @click="startServer">
                启动快传服务
              </NButton>
            </template>
          </NEmpty>
        </div>

        <div class="transfer-card">
          <div class="transfer-card-title">分享内容</div>
          <div class="share-actions">
            <NButton size="small" secondary type="primary" @click="pickFiles">
              <template #icon>
                <NIcon><FolderOpen20Regular /></NIcon>
              </template>
              添加文件
            </NButton>
            <NButton size="small" secondary @click="shareClipboard">
              <template #icon>
                <NIcon><ClipboardPaste20Regular /></NIcon>
              </template>
              分享剪贴板文本
            </NButton>
          </div>
          <div class="share-text-row">
            <NInput
              v-model:value="draftText"
              type="textarea"
              size="small"
              placeholder="输入要分享的文本…"
              :autosize="{ minRows: 2, maxRows: 5 }"
            />
            <NButton size="small" type="primary" :disabled="!draftText.trim()" :loading="sharingText" @click="shareDraftText">
              分享
            </NButton>
          </div>
          <div v-if="items.length > 0" class="share-list">
            <div v-for="item in items" :key="item.id" class="share-item">
              <NTag size="small" :bordered="false" :type="item.kind === 'text' ? 'info' : 'default'">
                {{ item.kind === 'text' ? '文本' : (item.isImage ? '图片' : '文件') }}
              </NTag>
              <NTooltip trigger="hover" :disabled="item.kind !== 'text'">
                <template #trigger>
                  <span class="share-item-name">{{ item.name }}</span>
                </template>
                <span class="share-item-text">{{ item.text }}</span>
              </NTooltip>
              <span class="share-item-size">{{ itemSizeText(item) }}</span>
              <NButton size="tiny" quaternary @click="removeItem(item.id)">移除</NButton>
            </div>
            <NButton size="tiny" quaternary class="share-clear" @click="onClear">全部清空</NButton>
          </div>
          <NEmpty v-else description="暂无分享内容" size="small" class="share-empty" />
        </div>
      </div>

      <div class="transfer-col">
        <div v-if="receivedTexts.length > 0" class="transfer-card">
          <div class="transfer-card-title">收到的文本</div>
          <div v-for="text in receivedTexts" :key="text.id" class="received-item">
            <div class="received-meta">{{ text.remoteAddr }} 发来</div>
            <div class="received-body">{{ text.text }}</div>
            <div class="received-actions">
              <NButton size="tiny" quaternary type="primary" @click="copyText(text.text, '已复制到剪贴板')">
                复制
              </NButton>
            </div>
          </div>
        </div>

        <div class="transfer-card">
          <div class="transfer-card-title">传输动态</div>
          <div v-if="transfers.length > 0" class="transfer-list">
            <div v-for="state in transfers" :key="state.id" class="transfer-row">
              <NTag
                size="small"
                :bordered="false"
                :type="state.direction === 'download' ? 'info' : 'success'"
              >
                {{ state.direction === 'download' ? '浏览器下载' : '浏览器上传' }}
              </NTag>
              <div class="transfer-row-main">
                <div class="transfer-row-name">
                  {{ state.name }}
                  <span class="transfer-row-remote">来自 {{ state.remoteAddr }}</span>
                </div>
                <NProgress
                  v-if="state.phase !== 'error'"
                  type="line"
                  :show-indicator="false"
                  :height="6"
                  :percentage="transferPercent(state) ?? 0"
                  :status="state.phase === 'done' ? 'success' : 'default'"
                />
                <div class="transfer-row-status" :class="{ error: state.phase === 'error' }">
                  {{ transferStatusText(state) }}
                  <span v-if="transferProgressText(state)" class="transfer-row-bytes">
                    {{ transferProgressText(state) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
          <NEmpty v-else description="暂无传输" size="small" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.transfer-page {
  height: 100%;
  overflow: auto;
  box-sizing: border-box;
  padding: 16px 20px;
}

.transfer-columns {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.transfer-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.transfer-card {
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  padding: 14px 16px;
  background: var(--n-color);
}

.transfer-card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 12px;
}

.connect-body {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.connect-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.connect-input-row {
  display: flex;
  gap: 8px;
}

.connect-input-row .n-input {
  flex: 1;
  min-width: 0;
}

.connect-alt {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.connect-alt-item {
  font-size: 12px;
  color: var(--n-text-color-3);
  word-break: break-all;
}

.connect-meta {
  font-size: 12px;
  color: var(--n-text-color-2);
}

.connect-warn {
  font-size: 12px;
  color: var(--n-warning-color);
}

.connect-empty {
  padding: 12px 0;
}

.share-actions {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
}

.share-text-row {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  margin-bottom: 10px;
}

.share-text-row .n-input {
  flex: 1;
}

.share-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.share-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
}

.share-item-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.share-item-text {
  white-space: pre-wrap;
  word-break: break-all;
}

.share-item-size {
  font-size: 12px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}

.share-clear {
  align-self: flex-start;
}

.share-empty {
  padding: 8px 0;
}

.received-item {
  padding: 8px 0;
  border-bottom: 1px solid var(--n-border-color);
}

.received-item:last-child {
  border-bottom: none;
}

.received-meta {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-bottom: 4px;
}

.received-body {
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 120px;
  overflow: auto;
  background: var(--n-action-color);
  border-radius: 6px;
  padding: 8px 10px;
}

.received-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 2px;
}

.transfer-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.transfer-row {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}

.transfer-row-main {
  flex: 1;
  min-width: 0;
}

.transfer-row-name {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.transfer-row-remote {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-left: 6px;
}

.transfer-row-status {
  font-size: 12px;
  color: var(--n-text-color-3);
  margin-top: 2px;
}

.transfer-row-status.error {
  color: var(--n-error-color);
}

.transfer-row-bytes {
  margin-left: 6px;
}
</style>
