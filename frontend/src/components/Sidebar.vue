<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { NTree, NDivider, NText, NSplit, NIcon } from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { FolderOutlined } from '@vicons/material'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import { useSettings } from '../composables/useSettings'
import { useDirectoryChangeListener } from '../composables/useDirectoryEvents'
import { ancestorPaths, joinPath, pathRoot } from './path'

type RootInfo = { name: string, path: string, freeSpace: number, totalSpace: number }

const { settings } = useSettings()

const props = defineProps<{
  currentPath: string
  homeDir: string
  separator: string
  roots: RootInfo[]
}>()

const emit = defineEmits<{
  navigate: [path: string]
}>()

const treeData = ref<TreeOption[]>([])
const expandedKeys = ref<string[]>([])
const rootByPath = computed(() => new Map(props.roots.map(root => [root.path, root])))

watch(() => [props.homeDir, props.separator, props.roots] as const, ([home, separator, roots]) => {
  const homeRoot = home ? pathRoot(home, separator) : ''
  const rootEntries = roots.length > 0
    ? roots
    : [{ name: homeRoot || separator, path: homeRoot || separator, freeSpace: 0, totalSpace: 0 }]
  const seen = new Set<string>()
  treeData.value = rootEntries
    .filter(root => {
      if (!root.path || seen.has(root.path)) return false
      seen.add(root.path)
      return true
    })
    .map(root => ({
      label: root.name || root.path,
      key: root.path,
      isLeaf: false,
    }))
}, { immediate: true })

function renderTreeLabel({ option }: { option: TreeOption }) {
  const key = typeof option.key === 'string' ? option.key : String(option.key)
  const root = rootByPath.value.get(key)
  if (!root || !root.totalSpace) return option.label as string
  const usage = formatRootUsage(root)

  const usedPercent = Math.min(
    100,
    Math.max(0, Math.round(((root.totalSpace - root.freeSpace) / root.totalSpace) * 100)),
  )
  return h('div', {
    class: 'root-label',
    style: { '--used-percent': `${usedPercent}%` },
  }, [
    h('span', { class: 'root-title' }, root.name || root.path),
    h('span', { class: 'root-space' }, usage),
  ])
}

function formatRootUsage(root: RootInfo): string {
  const totalG = Math.max(1, bytesToG(root.totalSpace))
  const usedG = Math.min(totalG, Math.max(0, bytesToG(root.totalSpace - root.freeSpace)))
  return `${usedG}/${totalG}G`
}

function bytesToG(bytes: number): number {
  return Math.round(bytes / 1024 ** 3)
}

// 当通过外部方式（Quick Access、FileTable）导航时，展开祖先路径
watch(() => props.currentPath, (path) => {
  if (!path) return
  const ancestors = ancestorPaths(path, props.separator)
  // 合并已有的 expandedKeys 和新的祖先路径
  const merged = new Set([...expandedKeys.value, ...ancestors])
  // 同时确保当前路径也在 expandedKeys 中（这样它的子节点可以展开）
  if (path !== pathRoot(path, props.separator)) {
    merged.add(path)
  }
  expandedKeys.value = Array.from(merged)
})

function onUpdateExpandedKeys(keys: string[]) {
  expandedKeys.value = keys
}

async function onLoad(node: TreeOption) {
  try {
    node.children = await loadDirectoryChildren(node.key as string)
    node.isLeaf = node.children.length === 0
  } catch {
    node.children = []
    node.isLeaf = true
  }
}

async function loadDirectoryChildren(path: string): Promise<TreeOption[]> {
  const entries = await FileService.ListDir(path)
  return entries
    .filter(e => e.isDir && (settings.showHiddenFiles || !e.isHidden))
    .map(e => ({
      label: e.name,
      key: e.path,
      isLeaf: false,
    }))
}

useDirectoryChangeListener((dirs) => {
  for (const dir of dirs) {
    refreshTreeNode(dir)
  }
})

async function refreshTreeNode(path: string) {
  const node = findTreeNode(treeData.value, path)
  if (!node) return
  try {
    node.children = await loadDirectoryChildren(path)
    node.isLeaf = node.children.length === 0
  } catch {
    node.children = []
    node.isLeaf = true
  }
}

function findTreeNode(nodes: TreeOption[] | undefined, key: string): TreeOption | null {
  if (!nodes) return null
  for (const node of nodes) {
    if (String(node.key) === key) return node
    const found = findTreeNode(node.children, key)
    if (found) return found
  }
  return null
}

function onUpdateSelectedKeys(keys: string[]) {
  if (keys.length > 0) {
    emit('navigate', keys[0])
  }
}

const quickAccess = computed(() => {
  const h = props.homeDir
  if (!h) return []
  return [
    { label: 'Home', path: h },
    { label: 'Desktop', path: joinPath(h, 'Desktop', props.separator) },
    { label: 'Documents', path: joinPath(h, 'Documents', props.separator) },
    { label: 'Downloads', path: joinPath(h, 'Downloads', props.separator) },
  ]
})
</script>

<template>
  <div class="sidebar">
    <NSplit
      class="sidebar-split"
      direction="vertical"
      :default-size="'170px'"
      :min="'80px'"
      :max="'300px'"
      :resize-trigger-size="3"
    >
      <template #[1]>
        <div class="quick-access">
          <NText depth="3" class="section-title">快速访问</NText>
          <div
            v-for="item in quickAccess"
            :key="item.path"
            class="quick-item"
            :class="{ active: currentPath === item.path }"
            @click="emit('navigate', item.path)"
          >
            <NIcon class="quick-icon" :size="16" color="#D99A22">
              <FolderOutlined/>
            </NIcon>
            <span class="quick-label">{{ item.label }}</span>
          </div>
        </div>
      </template>
      <template #[2]>
        <div class="tree-wrapper">
          <NTree
            :data="treeData"
            :selected-keys="[currentPath]"
            :expanded-keys="expandedKeys"
            :remote="true"
            :on-load="onLoad"
            :render-label="renderTreeLabel"
            :on-update:expanded-keys="onUpdateExpandedKeys"
            :on-update:selected-keys="onUpdateSelectedKeys"
            block-line
          />
        </div>
      </template>
    </NSplit>
  </div>
</template>

<style scoped>
.sidebar {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  user-select: none;
}

.section-title {
  display: block;
  padding: 8px 16px 4px;
  font-size: 12px;
}

.quick-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 16px;
  cursor: pointer;
  border-radius: 0;
  margin: 0;
}

.quick-item:hover {
  background-color: var(--n-color-hover);
}

.quick-item.active {
  background-color: var(--n-color-selected);
}

.quick-icon {
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  line-height: 1;
}

.quick-label {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-split {
  flex: 1;
  min-height: 0;
}

.tree-wrapper {
  height: 100%;
  overflow: auto;
  padding: 4px 0;
}

:deep(.root-label) {
  width: 100%;
  min-width: 120px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  padding: 2px 6px;
  margin: 1px 0;
  border-radius: 4px;
  box-sizing: border-box;
  background:
    linear-gradient(
      to right,
      #E7F5EE 0,
      #E7F5EE var(--used-percent),
      transparent var(--used-percent),
      transparent 100%
    );
}

:deep(.root-title) {
  display: block;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 18px;
}

:deep(.root-space) {
  flex-shrink: 0;
  color: var(--n-text-color-3);
  font-size: 8px;
  line-height: 18px;
  white-space: nowrap;
}
</style>
