<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NModal, NRadioButton, NRadioGroup, NSwitch, NInput, NButton, useMessage } from 'naive-ui'
import { useSettings } from '../composables/useSettings'
import type { ThemeMode } from '../composables/useSettings'
import { Dialogs, System } from '@wailsio/runtime'
import { SettingsService } from '../../bindings/zashiki/internal/settings'

defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const { settings, updateSetting } = useSettings()
const message = useMessage()
const addingToPath = ref(false)
const addingToContextMenu = ref(false)
const isWindows = ref(false)

onMounted(() => {
  isWindows.value = System.IsWindows()
})

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

async function browseDefaultEditor() {
  const result = await Dialogs.OpenFile({
    Title: '选择默认编辑器',
    CanChooseFiles: true,
    CanChooseDirectories: false,
    TreatsFilePackagesAsDirectories: false,
  })
  const path = Array.isArray(result) ? result[0] : result
  if (path) {
    updateSetting('defaultEditor', path)
  }
}

async function addToPath() {
  if (addingToPath.value) return

  addingToPath.value = true
  try {
    await SettingsService.AddToPath()
    message.success('已加入 PATH，请重新打开终端后使用')
  } catch (err) {
    const detail = err instanceof Error ? err.message : String(err)
    message.error(`加入 PATH 失败：${detail}`)
  } finally {
    addingToPath.value = false
  }
}

async function addToContextMenu() {
  if (addingToContextMenu.value) return

  addingToContextMenu.value = true
  try {
    await SettingsService.AddToContextMenu()
    message.success('已加入资源管理器右键菜单')
  } catch (err) {
    const detail = err instanceof Error ? err.message : String(err)
    message.error(`加入右键菜单失败：${detail}`)
  } finally {
    addingToContextMenu.value = false
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
      <div class="program-input-group">
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
    <div class="setting-item">
      <span class="setting-label">默认编辑器</span>
      <div class="program-input-group">
        <NInput
          :value="settings.defaultEditor"
          @update:value="(val: string) => updateSetting('defaultEditor', val)"
          placeholder="Visual Studio Code"
          style="width: 280px"
          size="small"
        />
        <NButton size="small" @click="browseDefaultEditor">浏览</NButton>
      </div>
    </div>
    <div class="setting-item">
      <span class="setting-label">命令行</span>
      <NButton size="small" type="primary" :loading="addingToPath" @click="addToPath">
        加入到 PATH
      </NButton>
    </div>
    <div v-if="isWindows" class="setting-item">
      <span class="setting-label">右键菜单</span>
      <NButton size="small" type="primary" :loading="addingToContextMenu" @click="addToContextMenu">
        加入到右键菜单
      </NButton>
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

.program-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
