<script setup lang="ts">
import {ref, onMounted, watch} from 'vue'
import {NSplit, NMessageProvider, NSpin, NConfigProvider, NDivider} from 'naive-ui'
import Sidebar from './components/Sidebar.vue'
import SplitNode from './components/SplitNode.vue'
import {createLeaf, splitLeaf, closeLeaf, navigateLeaf, getFirstLeafId, findLeafById} from './components/tree'
import type {TreeNode} from './components/tree'
import {FileService} from '../bindings/zashiki'

const currentPath = ref('')
const homeDir = ref('')
const loading = ref(true)
const error = ref('')

let nextId = 1
const rootNode = ref<TreeNode>(createLeaf(nextId++, ''))
const focusedId = ref(1)

onMounted(async () => {
  try {
    homeDir.value = await FileService.GetHomeDir()
    currentPath.value = homeDir.value
  } catch (err) {
    console.error('GetHomeDir failed:', err)
    error.value = String(err)
    currentPath.value = '/'
  } finally {
    loading.value = false
  }
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
  rootNode.value = splitLeaf(rootNode.value, leafId, nextId++, nextId++, direction)
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

function handleFocus(leafId: number, path: string) {
  if (focusedId.value === leafId) return
  focusedId.value = leafId
  currentPath.value = path
}
</script>

<template>
  <NConfigProvider>
    <NMessageProvider>
      <div v-if="loading" class="app-loading">
        <NSpin/>
      </div>
      <div v-else class="app-layout">
        <div class="app-header"></div>
        <NDivider style="margin: 0" />
        <NSplit
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
                @navigate="onNavigate"
            />
          </template>
          <template #[2]>
            <div class="main-content">
              <SplitNode
                  :node="rootNode"
                  :focused-id="focusedId"
                  :closable="false"
                  @navigate="handleNavigate"
                  @split="handleSplit"
                  @close="handleClose"
                  @focus="handleFocus"
              />
            </div>
          </template>
        </NSplit>
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
  background: rgb(255, 255, 255);
}

.app-layout {
  width: 100vw;
  height: 100vh;
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
}
</style>
