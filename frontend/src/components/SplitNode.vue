<script setup lang="ts">
import FileTable from './FileTable.vue'
import type { TreeNode } from './tree'
import { isLeaf } from './tree'

defineProps<{
  node: TreeNode
  focusedId: number
  closable: boolean
}>()

const emit = defineEmits<{
  navigate: [id: number, path: string]
  split: [id: number, direction: 'horizontal' | 'vertical']
  close: [id: number]
  focus: [id: number, path: string]
}>()
</script>

<template>
  <div class="split-node-root">
    <div
      v-if="isLeaf(node)"
      class="leaf-wrapper"
      :class="{ dimmed: !closable ? false : node.id !== focusedId }"
      @mousedown="emit('focus', node.id, node.path)"
    >
      <FileTable
        :path="node.path"
        :closable="closable"
        @navigate="(path: string) => emit('navigate', node.id, path)"
        @split-h="emit('split', node.id, 'horizontal')"
        @split-v="emit('split', node.id, 'vertical')"
        @close="emit('close', node.id)"
      />
    </div>
    <div
      v-else
      class="split-wrapper"
      :class="node.direction"
    >
      <SplitNode
        v-for="(child, index) in node.children"
        :key="child.id"
        :node="child"
        :focused-id="focusedId"
        :closable="true"
        :class="{
          'split-child': true,
          'split-child-first': index === 0,
          'split-child-last': index === node.children.length - 1,
        }"
        @navigate="(id, path) => emit('navigate', id, path)"
        @split="(id, dir) => emit('split', id, dir)"
        @close="(id) => emit('close', id)"
        @focus="(id, path) => emit('focus', id, path)"
      />
    </div>
  </div>
</template>

<script lang="ts">
// recursive self-reference
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

.split-wrapper {
  flex: 1;
  display: flex;
  min-height: 0;
  min-width: 0;
}

.split-wrapper.horizontal {
  flex-direction: column;
}

.split-wrapper.vertical {
  flex-direction: row;
}

.split-child {
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
}

.split-wrapper.horizontal > .split-child + .split-child {
  border-top: 2px solid var(--n-border-color);
}

.split-wrapper.vertical > .split-child + .split-child {
  border-left: 2px solid var(--n-border-color);
}
</style>
