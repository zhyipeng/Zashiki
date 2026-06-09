<script setup lang="ts">
import {computed, ref, onMounted, onUnmounted, watch} from 'vue'
import {NSplit, NMessageProvider, NSpin, NConfigProvider, NDivider, NFlex, NButton, NIcon, darkTheme} from 'naive-ui'
import Sidebar from './components/Sidebar.vue'
import SplitNode from './components/SplitNode.vue'
import {createLeaf, splitLeaf, closeLeaf, keepOnlyLeaf, navigateLeaf, getFirstLeafId, getLeafIds, findLeafById} from './components/tree'
import type {TreeNode} from './components/tree'
import {FileService} from '../bindings/zashiki/internal/filemanager'
import { Settings28Regular } from '@vicons/fluent'
import SettingsModal from './components/SettingsModal.vue'
import { useTheme } from './composables/useTheme'

const showSettings = ref(false)
const { isDarkTheme, mountTheme } = useTheme()
const naiveTheme = computed(() => isDarkTheme.value ? darkTheme : null)

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
let cleanupTheme: (() => void) | null = null

onMounted(async () => {
  cleanupTheme = mountTheme()
  window.addEventListener('keydown', onGlobalKeydown)
  try {
    const [home, sep, rootDirs, trash] = await Promise.all([
      FileService.GetHomeDir(),
      FileService.GetSeparator(),
      FileService.GetRoots(),
      FileService.GetTrashInfo(),
    ])
    homeDir.value = home
    separator.value = sep || '/'
    roots.value = rootDirs || []
    trashInfo.value = trash || { label: '', path: '', available: false }
    currentPath.value = homeDir.value
  } catch (err) {
    console.error('GetHomeDir failed:', err)
    error.value = String(err)
    currentPath.value = '/'
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
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

function onGlobalKeydown(event: KeyboardEvent) {
  if (event.key !== 'Tab' || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return
  if (isEditableTarget(event.target)) return
  event.preventDefault()
  handleFocusNextPanel(focusedId.value)
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return tag === 'input' || tag === 'textarea' || target.isContentEditable
}
</script>

<template>
  <NConfigProvider :theme="naiveTheme">
    <NMessageProvider>
      <div v-if="loading" class="app-loading">
        <NSpin/>
      </div>
      <div v-else class="app-layout">
        <n-flex class="app-header" justify="end">
          <n-button text style="font-size: 24px" @click="showSettings = true">
            <n-icon><Settings28Regular/></n-icon>
          </n-button>
        </n-flex>
        <NDivider style="margin: 0" />
        <NSplit
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
              />
            </div>
          </template>
        </NSplit>
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
}

.app-main-split {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
</style>
