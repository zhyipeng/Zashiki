<script setup lang="ts">
import { NModal, NRadioButton, NRadioGroup, NSwitch, NInput, NButton } from 'naive-ui'
import { useSettings } from '../composables/useSettings'
import type { ThemeMode } from '../composables/useSettings'
import { Dialogs } from '@wailsio/runtime'

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

async function browseTerminalProgram() {
  const result = await Dialogs.OpenFile({
    Title: '选择终端程序',
    CanChooseFiles: true,
    CanChooseDirectories: false,
    TreatsFilePackagesAsDirectories: false,
  })
  const path = Array.isArray(result) ? result[0] : result
  if (path) {
    updateSetting('terminalProgram', path)
  }
}
</script>

<template>
  <NModal
    :show="show"
    :on-update:show="(val: boolean) => emit('update:show', val)"
    preset="card"
    title="设置"
    style="width: 620px"
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
    <div class="setting-item">
      <span class="setting-label">默认终端程序</span>
      <div class="terminal-input-group">
        <NInput
          :value="settings.terminalProgram"
          @update:value="(val: string) => updateSetting('terminalProgram', val)"
          placeholder="Terminal"
          style="width: 280px"
          size="small"
        />
        <NButton size="small" @click="browseTerminalProgram">浏览</NButton>
      </div>
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

.terminal-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
