<script setup lang="ts">
import { NSplit } from 'naive-ui'
import FileTable from './FileTable.vue'
import type { TreeNode } from './tree'
import { isLeaf } from './tree'

interface SelectionStatus {
  multiSelectMode: boolean
  selectedCount: number
}

defineProps<{
  node: TreeNode
  focusedId: number
  closable: boolean
  separator: string
  homeDir: string
  trashLabel: string
}>()

const emit = defineEmits<{
  navigate: [id: number, path: string]
  split: [id: number, direction: 'horizontal' | 'vertical']
  close: [id: number]
  closeOthers: [id: number]
  focus: [id: number, path: string]
  focusNext: [id: number]
  focusPrev: [id: number]
  selectionStatus: [id: number, status: SelectionStatus]
}>()

// Our direction → NSplit direction:
//   horizontal (stacked top/bottom) → NSplit vertical
//   vertical (side by side)        → NSplit horizontal
function nsDir(dir: 'horizontal' | 'vertical') {
  return dir === 'horizontal' ? 'vertical' : 'horizontal'
}
</script>

<template>
  <div class="split-node-root">
    <div
      v-if="isLeaf(node)"
      class="leaf-wrapper"
      :class="{ dimmed: closable && node.id !== focusedId }"
      @mousedown="emit('focus', node.id, node.path)"
    >
      <FileTable
        :path="node.path"
        :closable="closable"
        :focused="node.id === focusedId"
        :separator="separator"
        :home-dir="homeDir"
        :trash-label="trashLabel"
        @navigate="(path: string) => emit('navigate', node.id, path)"
        @split-h="emit('split', node.id, 'horizontal')"
        @split-v="emit('split', node.id, 'vertical')"
        @close="emit('close', node.id)"
        @close-others="emit('closeOthers', node.id)"
        @focus-next="emit('focusNext', node.id)"
        @focus-prev="emit('focusPrev', node.id)"
        @selection-status="(status: SelectionStatus) => emit('selectionStatus', node.id, status)"
      />
    </div>
    <NSplit
      v-else
      :direction="nsDir(node.direction)"
      :default-size="0.5"
      :resize-trigger-size="4"
      :pane-1-style="{ overflow: 'hidden' }"
      :pane-2-style="{ overflow: 'hidden' }"
    >
      <template #[1]>
        <SplitNode
          :node="node.children[0]"
          :focused-id="focusedId"
          :closable="true"
          :separator="separator"
          :home-dir="homeDir"
          :trash-label="trashLabel"
          @navigate="(id, path) => emit('navigate', id, path)"
          @split="(id, dir) => emit('split', id, dir)"
          @close="(id) => emit('close', id)"
          @close-others="(id) => emit('closeOthers', id)"
          @focus="(id, path) => emit('focus', id, path)"
          @focus-next="(id) => emit('focusNext', id)"
          @focus-prev="(id) => emit('focusPrev', id)"
          @selection-status="(id, status) => emit('selectionStatus', id, status)"
        />
      </template>
      <template #[2]>
        <SplitNode
          :node="node.children[1]"
          :focused-id="focusedId"
          :closable="true"
          :separator="separator"
          :home-dir="homeDir"
          :trash-label="trashLabel"
          @navigate="(id, path) => emit('navigate', id, path)"
          @split="(id, dir) => emit('split', id, dir)"
          @close="(id) => emit('close', id)"
          @close-others="(id) => emit('closeOthers', id)"
          @focus="(id, path) => emit('focus', id, path)"
          @focus-next="(id) => emit('focusNext', id)"
          @focus-prev="(id) => emit('focusPrev', id)"
          @selection-status="(id, status) => emit('selectionStatus', id, status)"
        />
      </template>
    </NSplit>
  </div>
</template>

<script lang="ts">
export default { name: 'SplitNode' }
</script>

<style scoped>
.split-node-root {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.leaf-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  transition: opacity 0.15s;
}

.leaf-wrapper.dimmed {
  opacity: 0.5;
}
</style>
