<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NTree, NDivider, NText, NSplit } from 'naive-ui'
import type { TreeOption } from 'naive-ui'
import { FileService } from '../../bindings/zashiki'
import { useSettings } from '../composables/useSettings'
import { ancestorPaths, baseName, joinPath, pathRoot } from './path'

const { settings } = useSettings()

const props = defineProps<{
  currentPath: string
  homeDir: string
  separator: string
}>()

const emit = defineEmits<{
  navigate: [path: string]
}>()

const treeData = ref<TreeOption[]>([])
const expandedKeys = ref<string[]>([])

watch(() => props.homeDir, (home) => {
  if (!home) return
  const root = pathRoot(home, props.separator) || props.separator
  treeData.value = [
    { label: 'Computer', key: root, isLeaf: false },
    { label: baseName(home, props.separator) || 'Home', key: home, isLeaf: false },
  ]
}, { immediate: true })

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
    const entries = await FileService.ListDir(node.key as string)
    node.children = entries
      .filter(e => e.isDir && (settings.showHiddenFiles || !e.isHidden))
      .map(e => ({
        label: e.name,
        key: e.path,
        isLeaf: false,
      }))
  } catch {
    node.children = []
    node.isLeaf = true
  }
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
            <span class="quick-icon">📁</span>
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
  font-size: 14px;
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
</style>
