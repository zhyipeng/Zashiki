<script setup lang="ts">
import { NModal, NRadioButton, NRadioGroup, NSwitch } from 'naive-ui'
import { useSettings } from '../composables/useSettings'
import type { ThemeMode } from '../composables/useSettings'

defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { settings, updateSetting } = useSettings()

const themeOptions: { label: string, value: ThemeMode }[] = [
  { label: '浅色', value: 'light' },
  { label: '深色', value: 'dark' },
  { label: '跟随系统', value: 'system' },
]
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
      <span class="setting-label">主题</span>
      <NRadioGroup :value="settings.themeMode" @update:value="(val: ThemeMode) => updateSetting('themeMode', val)">
        <NRadioButton
          v-for="option in themeOptions"
          :key="option.value"
          :value="option.value"
          :label="option.label"
        />
      </NRadioGroup>
    </div>
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
  flex-shrink: 0;
}
</style>
