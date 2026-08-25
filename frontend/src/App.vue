<script setup lang="ts">
import {computed, ref, onMounted, onUnmounted, watch} from 'vue'
import {NSplit, NMessageProvider, NSpin, NConfigProvider, NDivider, NButton, NIcon, NTooltip, darkTheme} from 'naive-ui'
import Sidebar from './components/Sidebar.vue'
import SplitNode from './components/SplitNode.vue'
import {createLeaf, splitLeaf, closeLeaf, keepOnlyLeaf, navigateLeaf, getFirstLeafId, getLeafIds, findLeafById} from './components/tree'
import type {TreeNode} from './components/tree'
import {FileService} from '../bindings/zashiki/internal/filemanager'
import { Settings28Regular, Folder28Regular, ArrowSync24Regular } from '@vicons/fluent'
import SettingsModal from './components/SettingsModal.vue'
import OperationProgressbar from './components/OperationProgressbar.vue'
import SyncToolPage from './components/sync/SyncToolPage.vue'
import { useTheme } from './composables/useTheme'
import { useFileClipboard } from './composables/useFileClipboard'
import { bindOperationProgressEvents } from './composables/useOperationProgress'

const showSettings = ref(false)
const activeView = ref<'files' | 'sync'>('files')
const { isDarkTheme, mountTheme } = useTheme()
const fileClipboard = useFileClipboard()
const naiveTheme = computed(() => isDarkTheme.value ? darkTheme : null)

interface SelectionStatus {
  multiSelectMode: boolean
  selectedCount: number
}

const currentPath = ref('')
const homeDir = ref('')
const separator = ref('/')
const roots = ref<{ name: string, path: string, freeSpace: number, totalSpace: number }[]>([])
const trashInfo = ref<{ label: string, path: string, available: boolean }>({ label: '', path: '', available: false })
const loading = ref(true)
const error = ref('')

let nextId = 1
const rootNode = ref<TreeNode>(createLeaf(nextId++, ''))
const focusedId = ref(1)
const selectionStatusById = ref<Record<number, SelectionStatus>>({})
let cleanupTheme: (() => void) | null = null
let clipboardPollTimer: number | null = null

const clipboardStatusText = computed(() => {
  const clipboard = fileClipboard.clipboard.value
  if (!clipboard || clipboard.paths.length === 0) return ''
  const action = clipboard.mode === 'cut' ? '剪切' : '复制'
  return `已${action} ${formatStatusTarget(clipboard.paths)}`
})
const focusedSelectionStatus = computed(() => selectionStatusById.value[focusedId.value] || null)
const selectionStatusText = computed(() => {
  const status = focusedSelectionStatus.value
  if (!status?.multiSelectMode) return ''
  return `已选择 ${status.selectedCount} 项`
})
const appStatusItems = computed(() => [clipboardStatusText.value, selectionStatusText.value].filter(Boolean))

onMounted(async () => {
  cleanupTheme = mountTheme()
  bindOperationProgressEvents()
  window.addEventListener('keydown', onGlobalKeydown)
  clipboardPollTimer = window.setInterval(() => {
    void fileClipboard.refreshSequence()
  }, 2000)
  try {
    const [home, sep, rootDirs, trash, initial] = await Promise.all([
      FileService.GetHomeDir(),
      FileService.GetSeparator(),
      FileService.GetRoots(),
      FileService.GetTrashInfo(),
      FileService.GetInitialDir(),
    ])
    homeDir.value = home
    separator.value = sep || '/'
    roots.value = rootDirs || []
    trashInfo.value = trash || { label: '', path: '', available: false }
    currentPath.value = initial || homeDir.value
  } catch (err) {
    console.error('GetHomeDir failed:', err)
    error.value = String(err)
    currentPath.value = '/'
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  if (clipboardPollTimer !== null) {
    window.clearInterval(clipboardPollTimer)
    clipboardPollTimer = null
  }
  window.removeEventListener('keydown', onGlobalKeydown)
  cleanupTheme?.()
  cleanupTheme = null
})

// sidebar → focused panel
watch(currentPath, (path) => {
  if (!path) return
  const focused = findLeafById(rootNode.value, focusedId.value)
  if (focused && focused.path !== path) {
    rootNode.value = navigateLeaf(rootNode.value, focusedId.value, path)
  }
})

// initial path
watch(homeDir, (home) => {
  if (home && rootNode.value.kind === 'leaf' && !rootNode.value.path) {
    rootNode.value = navigateLeaf(rootNode.value, rootNode.value.id, home)
    focusedId.value = rootNode.value.kind === 'leaf' ? rootNode.value.id : getFirstLeafId(rootNode.value)
  }
})

function onNavigate(path: string) {
  currentPath.value = path
}

function handleNavigate(leafId: number, path: string) {
  rootNode.value = navigateLeaf(rootNode.value, leafId, path)
  if (leafId === focusedId.value) {
    currentPath.value = path
  }
}

function handleSplit(leafId: number, direction: 'horizontal' | 'vertical') {
  const newLeafId = nextId++
  rootNode.value = splitLeaf(rootNode.value, leafId, newLeafId, nextId++, direction)
  focusedId.value = newLeafId
  const leaf = findLeafById(rootNode.value, newLeafId)
  if (leaf) currentPath.value = leaf.path
}

function handleClose(leafId: number) {
  const newRoot = closeLeaf(rootNode.value, leafId)
  if (!newRoot) return
  rootNode.value = newRoot
  if (focusedId.value === leafId) {
    const newId = getFirstLeafId(rootNode.value)
    focusedId.value = newId
    const leaf = findLeafById(rootNode.value, newId)
    if (leaf) currentPath.value = leaf.path
  }
}

function handleCloseOthers(leafId: number) {
  const newRoot = keepOnlyLeaf(rootNode.value, leafId)
  if (!newRoot) return
  rootNode.value = newRoot
  focusedId.value = leafId
  const leaf = findLeafById(rootNode.value, leafId)
  if (leaf) currentPath.value = leaf.path
}

function handleFocus(leafId: number, path: string) {
  if (focusedId.value === leafId) return
  focusedId.value = leafId
  currentPath.value = path
}

function handleFocusNextPanel(fromLeafId: number) {
  const leafIds = getLeafIds(rootNode.value)
  if (leafIds.length <= 1) return
  const currentIndex = leafIds.indexOf(fromLeafId)
  const nextIndex = currentIndex < 0 ? 0 : (currentIndex + 1) % leafIds.length
  const nextId = leafIds[nextIndex]
  const leaf = findLeafById(rootNode.value, nextId)
  if (!leaf) return
  focusedId.value = nextId
  currentPath.value = leaf.path
}

function handleFocusPreviousPanel(fromLeafId: number) {
  const leafIds = getLeafIds(rootNode.value)
  if (leafIds.length <= 1) return
  const currentIndex = leafIds.indexOf(fromLeafId)
  const prevIndex = currentIndex <= 0 ? leafIds.length - 1 : currentIndex - 1
  const prevId = leafIds[prevIndex]
  const leaf = findLeafById(rootNode.value, prevId)
  if (!leaf) return
  focusedId.value = prevId
  currentPath.value = leaf.path
}

function handleSelectionStatus(leafId: number, status: SelectionStatus) {
  selectionStatusById.value = {
    ...selectionStatusById.value,
    [leafId]: status,
  }
}

function onGlobalKeydown(event: KeyboardEvent) {
  if (event.key !== 'Tab' || event.ctrlKey || event.metaKey || event.altKey) return
  if (activeView.value !== 'files') return
  if (isEditableTarget(event.target)) return
  event.preventDefault()
  if (event.shiftKey) {
    handleFocusPreviousPanel(focusedId.value)
  } else {
    handleFocusNextPanel(focusedId.value)
  }
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return tag === 'input' || tag === 'textarea' || target.isContentEditable
}

function formatStatusTarget(paths: string[]): string {
  if (paths.length === 0) return ''
  const firstName = basename(paths[0])
  if (paths.length === 1) return `「${firstName}」`
  return `「${firstName}」等 ${paths.length} 项`
}

function basename(path: string): string {
  const slashIndex = path.lastIndexOf('/')
  const backslashIndex = path.lastIndexOf('\\')
  const index = Math.max(slashIndex, backslashIndex)
  return index < 0 ? path : path.slice(index + 1)
}
</script>

<template>
  <NConfigProvider :theme="naiveTheme">
    <NMessageProvider>
      <div v-if="loading" class="app-loading">
        <NSpin/>
      </div>
      <div v-else class="app-layout">
        <div class="app-header">
          <div class="app-status-bar">
            <span
              v-for="item in appStatusItems"
              :key="item"
              class="app-status-item"
            >
              {{ item }}
            </span>
          </div>
          <div class="app-view-switch">
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button
                  text
                  class="view-switch-button"
                  :class="{ active: activeView === 'files' }"
                  @click="activeView = 'files'"
                >
                  <n-icon :size="22"><Folder28Regular /></n-icon>
                </n-button>
              </template>
              文件管理
            </n-tooltip>
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button
                  text
                  class="view-switch-button"
                  :class="{ active: activeView === 'sync' }"
                  @click="activeView = 'sync'"
                >
                  <n-icon :size="22"><ArrowSync24Regular /></n-icon>
                </n-button>
              </template>
              同步工具
            </n-tooltip>
            <n-button text style="font-size: 24px" @click="showSettings = true">
              <n-icon><Settings28Regular/></n-icon>
            </n-button>
          </div>
        </div>
        <NDivider style="margin: 0" />
        <NSplit
            v-show="activeView === 'files'"
            class="app-main-split"
            direction="horizontal"
            :default-size="'180px'"
            :min="'40px'"
            :max="'500px'"
            :resize-trigger-size="3"
            :pane-1-style="{ overflow: 'hidden' }"
            :pane-2-style="{ overflow: 'hidden' }"
        >
          <template #[1]>
            <Sidebar
                :currentPath="currentPath"
                :homeDir="homeDir"
                :separator="separator"
                :roots="roots"
                :trash-info="trashInfo"
                @navigate="onNavigate"
            />
          </template>
          <template #[2]>
            <div class="main-content">
              <SplitNode
                  :node="rootNode"
                  :focused-id="focusedId"
                  :closable="false"
                  :separator="separator"
                  :home-dir="homeDir"
                  :trash-label="trashInfo.label || '回收站'"
                  @navigate="handleNavigate"
                  @split="handleSplit"
                  @close="handleClose"
                  @close-others="handleCloseOthers"
                  @focus="handleFocus"
                  @focus-next="handleFocusNextPanel"
                  @focus-prev="handleFocusPreviousPanel"
                  @selection-status="handleSelectionStatus"
              />
            </div>
          </template>
        </NSplit>
        <div v-show="activeView === 'sync'" class="app-sync-view">
          <SyncToolPage />
        </div>
        <OperationProgressbar />
        <SettingsModal v-model:show="showSettings" />
      </div>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
html, body, #app {
  margin: 0;
  padding: 0;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  user-select: none;
  -webkit-user-select: none;
  background: var(--app-background);
}

:root {
  --app-background: #ffffff;
  --sidebar-root-used-color: #E7F5EE;
  --sidebar-root-space-color: var(--n-text-color-3);
}

:root[data-theme="dark"] {
  --app-background: #101014;
  --sidebar-root-used-color: #2f5a46;
  --sidebar-root-space-color: #d9e7de;
}

.app-layout {
  width: 100vw;
  height: 100vh;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.app-loading {
  width: 100vw;
  height: 100vh;
  display: flex;
  justify-content: center;
  align-items: center;
}

.main-content {
  height: 100%;
  overflow: hidden;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
}

.app-header {
  height: 40px;
  padding: 0 15px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  box-sizing: border-box;
}

.app-view-switch {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.view-switch-button {
  font-size: 22px;
  color: var(--n-text-color-3);
  padding: 4px 6px;
  border-radius: 4px;
}

.view-switch-button.active {
  color: var(--n-primary-color);
  background: var(--n-color-hover);
}

.app-sync-view {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.app-status-bar {
  min-width: 0;
  flex: 1;
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
  color: var(--n-text-color-2);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
}

.app-status-item {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-status-item + .app-status-item {
  padding-left: 12px;
  border-left: 1px solid var(--n-border-color);
}

.app-main-split {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
</style>
