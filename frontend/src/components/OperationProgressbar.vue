<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon, NProgress } from 'naive-ui'
import { CloseRound, ContentCopyOutlined, DriveFileMoveOutlined, DeleteOutlineRound, DeleteSweepOutlined } from '@vicons/material'
import {
  isTerminalPhase,
  operationLabel,
  progressByteText,
  progressPercent,
  useOperationProgress,
} from '../composables/useOperationProgress'
import type { OperationProgressState } from '../composables/useOperationProgress'

const { states, cancelOperation, dismissOperation } = useOperationProgress()

const kindIcons = {
  copy: ContentCopyOutlined,
  move: DriveFileMoveOutlined,
  delete: DeleteOutlineRound,
  trash: DeleteSweepOutlined,
} as const

function iconFor(state: OperationProgressState) {
  return kindIcons[state.kind] ?? ContentCopyOutlined
}

function statusText(state: OperationProgressState): string {
  switch (state.phase) {
    case 'scan':
      return `${operationLabel(state.kind)}中 · 计算大小…`
    case 'done':
      return `${operationLabel(state.kind)}完成`
    case 'cancelled':
      return `已取消${operationLabel(state.kind)}`
    case 'error':
      return state.error || `${operationLabel(state.kind)}失败`
    default:
      return `${operationLabel(state.kind)}中 ${state.doneItems}/${state.totalItems} 项`
  }
}

function itemText(state: OperationProgressState): string {
  if (isTerminalPhase(state.phase)) return ''
  if (!state.currentName) return ''
  return state.currentName
}

function percentFor(state: OperationProgressState): number {
  return progressPercent(state) ?? 0
}

function progressStatus(state: OperationProgressState): 'default' | 'success' | 'error' | 'warning' {
  if (state.phase === 'done') return 'success'
  if (state.phase === 'error') return 'error'
  if (state.phase === 'cancelled') return 'warning'
  return 'default'
}

const hasStates = computed(() => states.value.length > 0)
</script>

<template>
  <div
    v-if="hasStates"
    class="operation-progressbar"
  >
    <div
      v-for="state in states"
      :key="state.id"
      class="operation-progressbar-row"
      :class="{ terminal: isTerminalPhase(state.phase) }"
    >
      <NIcon
        class="operation-icon"
        :size="14"
      >
        <component :is="iconFor(state)" />
      </NIcon>
      <span class="operation-status">{{ statusText(state) }}</span>
      <span
        v-if="itemText(state)"
        class="operation-current"
        :title="state.currentName"
      >{{ itemText(state) }}</span>
      <span
        v-if="progressByteText(state)"
        class="operation-bytes"
      >{{ progressByteText(state) }}</span>
      <NProgress
        class="operation-progress"
        type="line"
        :percentage="percentFor(state)"
        :height="6"
        :border-radius="3"
        :show-indicator="false"
        :status="progressStatus(state)"
      />
      <NButton
        v-if="!isTerminalPhase(state.phase)"
        text
        size="tiny"
        title="取消操作"
        @click="cancelOperation(state.id)"
      >
        <template #icon>
          <n-icon><CloseRound /></n-icon>
        </template>
      </NButton>
      <NButton
        v-else
        text
        size="tiny"
        title="关闭"
        @click="dismissOperation(state.id)"
      >
        <template #icon>
          <n-icon><CloseRound /></n-icon>
        </template>
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.operation-progressbar {
  border-top: 1px solid var(--border-color, #e0e0e6);
  padding: 2px 10px;
  flex-shrink: 0;
  font-size: 12px;
}

.operation-progressbar-row {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 26px;
}

.operation-icon {
  flex-shrink: 0;
  opacity: 0.8;
}

.operation-progressbar-row.terminal .operation-icon {
  opacity: 0.5;
}

.operation-status {
  flex-shrink: 0;
  white-space: nowrap;
}

.operation-current {
  flex-shrink: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  opacity: 0.65;
  min-width: 0;
}

.operation-bytes {
  flex-shrink: 0;
  white-space: nowrap;
  opacity: 0.65;
  font-variant-numeric: tabular-nums;
}

.operation-progress {
  flex: 1 1 60px;
  min-width: 60px;
}
</style>
