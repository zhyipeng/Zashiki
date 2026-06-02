<script setup lang="ts">
import { ref } from 'vue'
import { NModal, NRadioGroup, NRadio, NButton, NSpace } from 'naive-ui'

defineProps<{
  show: boolean
  sources: string[]
  targetDir: string
}>()

const emit = defineEmits<{
  'update:show': [boolean]
  confirm: [action: 'move' | 'copy', conflict: 'overwrite' | 'skip' | 'rename']
}>()

const conflict = ref<'overwrite' | 'skip' | 'rename'>('overwrite')
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="确认拖放操作"
    style="width: 420px"
    :on-update:show="(v: boolean) => emit('update:show', v)"
  >
    <div class="confirm-body">
      <div class="summary">
        已选择 {{ sources.length }} 项 → {{ targetDir }}
      </div>

      <div class="section">
        <div class="section-label">目标已有同名文件时</div>
        <NRadioGroup v-model:value="conflict">
          <NSpace vertical>
            <NRadio value="overwrite">覆盖</NRadio>
            <NRadio value="skip">跳过</NRadio>
            <NRadio value="rename">重命名（追加编号）</NRadio>
          </NSpace>
        </NRadioGroup>
      </div>
    </div>

    <template #footer>
      <NSpace justify="end">
        <NButton @click="emit('update:show', false)">取消</NButton>
        <NButton type="primary" @click="emit('confirm', 'copy', conflict)">复制</NButton>
        <NButton type="primary" @click="emit('confirm', 'move', conflict)">移动</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.confirm-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.section-label {
  font-size: 13px;
  color: var(--n-text-color-2);
}
.summary {
  font-size: 13px;
  color: var(--n-text-color-3);
}
</style>
