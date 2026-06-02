<script setup lang="ts">
import { NSplit } from 'naive-ui'
import FileTable from './FileTable.vue'
import type { TreeNode } from './tree'
import { isLeaf } from './tree'

defineProps<{
  node: TreeNode
  focusedId: number
  closable: boolean
  separator: string
}>()

const emit = defineEmits<{
  navigate: [id: number, path: string]
  split: [id: number, direction: 'horizontal' | 'vertical']
  close: [id: number]
  focus: [id: number, path: string]
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
        :separator="separator"
        @navigate="(path: string) => emit('navigate', node.id, path)"
        @split-h="emit('split', node.id, 'horizontal')"
        @split-v="emit('split', node.id, 'vertical')"
        @close="emit('close', node.id)"
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
          @navigate="(id, path) => emit('navigate', id, path)"
          @split="(id, dir) => emit('split', id, dir)"
          @close="(id) => emit('close', id)"
          @focus="(id, path) => emit('focus', id, path)"
        />
      </template>
      <template #[2]>
        <SplitNode
          :node="node.children[1]"
          :focused-id="focusedId"
          :closable="true"
          :separator="separator"
          @navigate="(id, path) => emit('navigate', id, path)"
          @split="(id, dir) => emit('split', id, dir)"
          @close="(id) => emit('close', id)"
          @focus="(id, path) => emit('focus', id, path)"
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
