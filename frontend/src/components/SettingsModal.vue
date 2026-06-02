<script setup lang="ts">
import { NModal, NSwitch } from 'naive-ui'
import { useSettings } from '../composables/useSettings'

defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { settings, updateSetting } = useSettings()
</script>

<template>
  <NModal
    :show="show"
    :on-update:show="(val: boolean) => emit('update:show', val)"
    preset="card"
    title="设置"
    style="width: 420px"
  >
    <div class="setting-item">
      <span class="setting-label">显示隐藏文件</span>
      <NSwitch :value="settings.showHiddenFiles" @update:value="(val: boolean) => updateSetting('showHiddenFiles', val)" />
    </div>
  </NModal>
</template>

<style scoped>
.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
}

.setting-label {
  font-size: 14px;
}
</style>
