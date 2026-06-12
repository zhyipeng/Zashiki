<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, ref, watch } from 'vue'
import { NTree, NText, NSplit, NIcon, NDropdown, useMessage } from 'naive-ui'
import type { DropdownOption, TreeOption } from 'naive-ui'
import { DeleteOutlined, FolderOutlined, FolderSpecialOutlined } from '@vicons/material'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import { useSettings } from '../composables/useSettings'
import { FILE_EXPLORER_DRAG_MIME, activeDragPaths, clearDrag, finishDragDrop, hasActiveDragPayload, startFileExplorerDrag } from '../composables/useDragDrop'
import { useDirectoryChangeListener } from '../composables/useDirectoryEvents'
import { ancestorPaths, baseName, joinPath, pathRoot } from './path'

type RootInfo = { name: string, path: string, freeSpace: number, totalSpace: number }
type TrashInfo = { label: string, path: string, available: boolean }
type QuickAccessItem = { label: string, path: string, isTrash?: boolean, isPinned?: boolean }

const { settings, updateSetting } = useSettings()
const message = useMessage()
const quickAccessRef = ref<HTMLElement | null>(null)
const quickAccessDragOver = ref(false)
const lastDragPoint = ref({ x: 0, y: 0 })
const quickAccessContextMenu = ref({
  show: false,
  x: 0,
  y: 0,
  item: null as QuickAccessItem | null,
})
const quickAccessContextOptions: DropdownOption[] = [
  {
    label: '从快速访问删除',
    key: 'remove-pinned',
  },
]

const props = defineProps<{
  currentPath: string
  homeDir: string
  separator: string
  roots: RootInfo[]
  trashInfo: TrashInfo
}>()

const emit = defineEmits<{
  navigate: [path: string]
}>()

const treeData = ref<TreeOption[]>([])
const expandedKeys = ref<string[]>([])
const rootByPath = computed(() => new Map(props.roots.map(root => [root.path, root])))

onMounted(() => {
  window.addEventListener('dragover', onWindowDragOver)
  window.addEventListener('dragend', onWindowDragEnd)
})

onUnmounted(() => {
  window.removeEventListener('dragover', onWindowDragOver)
  window.removeEventListener('dragend', onWindowDragEnd)
})

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

function treeNodeProps({ option }: { option: TreeOption }) {
  const path = typeof option.key === 'string' ? option.key : String(option.key)
  return {
    draggable: true,
    onDragstart: (e: DragEvent) => startFileExplorerDrag(e, [path], path),
    onDragend: () => clearDrag(),
  }
}

const quickAccess = computed(() => {
  const h = props.homeDir
  if (!h) return []
  const items: QuickAccessItem[] = [
    { label: 'Home', path: h },
    { label: 'Desktop', path: joinPath(h, 'Desktop', props.separator) },
    { label: 'Documents', path: joinPath(h, 'Documents', props.separator) },
    { label: 'Downloads', path: joinPath(h, 'Downloads', props.separator) },
  ]
  if (props.trashInfo.available) {
    items.push({ label: props.trashInfo.label || '回收站', path: props.trashInfo.path, isTrash: true })
  }
  for (const path of settings.pinnedQuickAccessPaths) {
    items.push({ label: baseName(path, props.separator), path, isPinned: true })
  }
  return items
})

const quickAccessHeight = computed(() => `${38 * quickAccess.value.length}px`)

async function onQuickAccessClick(item: QuickAccessItem) {
  if (item.isTrash) {
    try {
      await FileService.OpenTrash()
      return
    } catch (err) {
      message.error(`打开${item.label}失败：${err}`)
    }
  }
  if (item.path) {
    emit('navigate', item.path)
    return
  }
}

function rememberDragPoint(e: DragEvent) {
  lastDragPoint.value = { x: e.clientX, y: e.clientY }
}

function onQuickAccessDragOver(e: DragEvent) {
  if (!hasQuickAccessDropPayload(e)) return
  e.preventDefault()
  rememberDragPoint(e)
  quickAccessDragOver.value = true
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'link'
  }
}

function onQuickAccessDragEnter(e: DragEvent) {
  if (!hasQuickAccessDropPayload(e)) return
  e.preventDefault()
  rememberDragPoint(e)
  quickAccessDragOver.value = true
}

function onQuickAccessDragLeave(e: DragEvent) {
  if ((e.currentTarget as HTMLElement)?.contains(e.relatedTarget as HTMLElement)) return
  quickAccessDragOver.value = false
}

async function onQuickAccessDrop(e: DragEvent) {
  e.preventDefault()
  e.stopPropagation()
  rememberDragPoint(e)
  quickAccessDragOver.value = false
  const paths = quickAccessDropPaths(e)
  if (paths.length === 0) {
    finishDragDrop()
    return
  }

  await pinQuickAccessPaths(paths)
}

async function onWindowDragEnd() {
  if (!quickAccessDragOver.value && !isLastDragPointInQuickAccess()) return
  quickAccessDragOver.value = false
  const paths = activeDragPaths()
  if (paths.length === 0) {
    finishDragDrop()
    return
  }
  await pinQuickAccessPaths(paths)
}

function onWindowDragOver(e: DragEvent) {
  if (!hasActiveDragPayload()) return
  rememberDragPoint(e)
}

function isLastDragPointInQuickAccess(): boolean {
  const el = quickAccessRef.value
  if (!el) return false
  const rect = el.getBoundingClientRect()
  const { x, y } = lastDragPoint.value
  return x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom
}

async function pinQuickAccessPaths(paths: string[]) {
  const nextPaths = [...settings.pinnedQuickAccessPaths]
  let added = 0
  for (const path of paths) {
    if (nextPaths.includes(path) || isBuiltInQuickAccessPath(path)) continue
    try {
      const info = await FileService.GetFileInfo(path)
      if (!info.isDir) continue
      nextPaths.push(path)
      added++
    } catch (err) {
      console.error('Pin quick access failed:', path, err)
    }
  }

  if (added === 0) {
    finishDragDrop()
    return
  }
  persistPinnedQuickAccess(nextPaths)
  finishDragDrop()
  message.success(added > 1 ? `已固定 ${added} 个文件夹` : '已固定到快速访问')
}

function hasQuickAccessDropPayload(e: DragEvent): boolean {
  const types = Array.from(e.dataTransfer?.types || [])
  return hasActiveDragPayload() || types.includes(FILE_EXPLORER_DRAG_MIME) || types.includes('Files')
}

function quickAccessDropPaths(e: DragEvent): string[] {
  const activePaths = activeDragPaths()
  if (activePaths.length > 0) return activePaths

  const payload = e.dataTransfer?.getData(FILE_EXPLORER_DRAG_MIME) || e.dataTransfer?.getData('text/plain')
  if (payload) {
    try {
      const parsed = JSON.parse(payload)
      if (Array.isArray(parsed?.paths)) {
        return parsed.paths.filter((path: unknown): path is string => typeof path === 'string' && path.length > 0)
      }
    } catch {
      return []
    }
  }

  const paths: string[] = []
  const files = e.dataTransfer?.files
  if (!files) return paths
  for (let i = 0; i < files.length; i++) {
    const file = files[i] as any
    if (file.path) paths.push(file.path)
  }
  return paths
}

function isBuiltInQuickAccessPath(path: string): boolean {
  return quickAccess.value.some(item => !item.isPinned && item.path === path)
}

function showQuickAccessContextMenu(e: MouseEvent, item: QuickAccessItem) {
  if (!item.isPinned) return
  e.preventDefault()
  quickAccessContextMenu.value = {
    show: false,
    x: e.clientX,
    y: e.clientY,
    item,
  }
  requestAnimationFrame(() => {
    quickAccessContextMenu.value.show = true
  })
}

function hideQuickAccessContextMenu() {
  quickAccessContextMenu.value.show = false
}

function onQuickAccessContextSelect(key: string | number) {
  const item = quickAccessContextMenu.value.item
  hideQuickAccessContextMenu()
  if (key !== 'remove-pinned' || !item?.isPinned) return
  persistPinnedQuickAccess(settings.pinnedQuickAccessPaths.filter(path => path !== item.path))
}

function persistPinnedQuickAccess(paths: string[]) {
  updateSetting('pinnedQuickAccessPaths', [...paths])
}
</script>

<template>
  <div class="sidebar">
    <NSplit
      class="sidebar-split"
      direction="vertical"
      :size="quickAccessHeight"
      :resize-trigger-size="3"
    >
      <template #[1]>
        <div
          ref="quickAccessRef"
          class="quick-access"
          :class="{ 'drag-over': quickAccessDragOver }"
          @dragover="onQuickAccessDragOver"
          @dragenter="onQuickAccessDragEnter"
          @dragleave="onQuickAccessDragLeave"
          @drop="onQuickAccessDrop"
        >
          <NText depth="3" class="section-title">快速访问</NText>
          <div
            v-for="item in quickAccess"
            :key="item.isTrash ? 'trash' : item.path"
            class="quick-item"
            :class="{ active: item.path && currentPath === item.path, pinned: item.isPinned }"
            @click="onQuickAccessClick(item)"
            @contextmenu.stop="showQuickAccessContextMenu($event, item)"
          >
            <NIcon class="quick-icon" :size="16" :color="item.isPinned ? '#4B7BEC' : '#D99A22'">
              <component :is="item.isTrash ? DeleteOutlined : item.isPinned ? FolderSpecialOutlined : FolderOutlined"/>
            </NIcon>
            <span class="quick-label">{{ item.label }}</span>
          </div>
          <NDropdown
            trigger="manual"
            placement="bottom-start"
            :show="quickAccessContextMenu.show"
            :x="quickAccessContextMenu.x"
            :y="quickAccessContextMenu.y"
            :options="quickAccessContextOptions"
            @select="onQuickAccessContextSelect"
            @clickoutside="hideQuickAccessContextMenu"
          />
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
            :node-props="treeNodeProps"
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

.quick-access {
  height: 100%;
}

.quick-access.drag-over {
  background: rgba(var(--n-primary-color-rgb, 24, 160, 88), 0.08);
  outline: 1px dashed var(--n-primary-color, #18a058);
  outline-offset: -3px;
}

.quick-item.pinned .quick-label {
  font-weight: 500;
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
      var(--sidebar-root-used-color) 0,
      var(--sidebar-root-used-color) var(--used-percent),
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
  color: var(--sidebar-root-space-color);
  font-size: 8px;
  line-height: 18px;
  white-space: nowrap;
}
</style>
