<script setup lang="ts">
import { computed, h, ref } from 'vue'
import {
  NButton,
  NCheckbox,
  NCheckboxGroup,
  NDataTable,
  NDynamicTags,
  NIcon,
  NInput,
  NRadioButton,
  NRadioGroup,
  NSwitch,
  NTooltip,
  useMessage,
} from 'naive-ui'
import { Search20Regular, ArrowSync24Regular, FolderOpen20Regular } from '@vicons/fluent'
import { Dialogs } from '@wailsio/runtime'
import { FileService } from '../../../bindings/zashiki/internal/filemanager'
import type { SyncPlan } from '../../../bindings/zashiki/internal/filemanager/models'
import type { SyncResult } from '../../../bindings/zashiki/internal/filemanager/models'
import { useSettings } from '../../composables/useSettings'
import type { SyncToolSettings } from '../../composables/useSettings'
import { useOperationProgress, trackOperationPromise } from '../../composables/useOperationProgress'
import { isOperationCancelledError } from '../../composables/useOperationProgress'
import { notifyDirectoriesChanged } from '../../composables/useDirectoryEvents'
import {
  syncConfigIssue,
  syncConfigIssueMessage,
  syncExecuteConfirmText,
  syncModeLabel,
  syncOpLabel,
  syncPlanSummary,
  syncResultSummary,
} from './syncLabels'

const message = useMessage()
const { settings, updateSetting } = useSettings()
const { states: progressStates } = useOperationProgress()

const plan = ref<SyncPlan | null>(null)
const result = ref<SyncResult | null>(null)
const analyzing = ref(false)
const executing = ref(false)
const configDirty = ref(false)

const config = computed<SyncToolSettings>(() => settings.syncTool)
const configIssue = computed(() => syncConfigIssue(config.value))
const configIssueText = computed(() => syncConfigIssueMessage(configIssue.value))
const canAnalyze = computed(() => !configIssue.value && !analyzing.value && !executing.value)
const canExecute = computed(() => canAnalyze.value && plan.value !== null && !configDirty.value)
const hasDeleteActions = computed(() => (plan.value?.delete.length ?? 0) > 0)

const syncProgress = computed(() => progressStates.value.find(state => state.kind === 'sync'))
const running = computed(() => executing.value || (syncProgress.value !== undefined &&
  syncProgress.value.phase !== 'done' && syncProgress.value.phase !== 'error' && syncProgress.value.phase !== 'cancelled'))

function updateConfig<K extends keyof SyncToolSettings>(key: K, value: SyncToolSettings[K]) {
  updateSetting('syncTool', { ...config.value, [key]: value })
  invalidatePlan()
}

function invalidatePlan() {
  if (plan.value !== null) configDirty.value = true
}

async function browseSource() {
  const path = await pickDirectory('选择源目录')
  if (path) updateConfig('sourceDir', path)
}

async function browseTarget() {
  const path = await pickDirectory('选择同步目录')
  if (path) updateConfig('targetDir', path)
}

async function pickDirectory(title: string): Promise<string | null> {
  const result = await Dialogs.OpenFile({
    Title: title,
    CanChooseFiles: false,
    CanChooseDirectories: true,
    TreatsFilePackagesAsDirectories: false,
  })
  const path = Array.isArray(result) ? result[0] : result
  return path || null
}

async function analyze() {
  if (!canAnalyze.value) return
  analyzing.value = true
  result.value = null
  try {
    plan.value = await FileService.AnalyzeSync(toSyncConfig())
    configDirty.value = false
  } catch (err) {
    if (!isOperationCancelledError(err)) {
      message.error(`分析失败：${friendlyError(err)}`)
    }
  } finally {
    analyzing.value = false
  }
}

function toSyncConfig() {
  const c = config.value
  return {
    sourceDir: c.sourceDir,
    targetDir: c.targetDir,
    mode: c.mode,
    compareSize: c.compareSize,
    compareModTime: c.compareModTime,
    compareHash: c.compareHash,
    ignoreHidden: c.ignoreHidden,
    ignorePatterns: [...c.ignorePatterns],
  }
}

async function execute() {
  if (!canExecute.value || !plan.value) return
  executing.value = true
  result.value = null
  try {
    const syncResult = await trackOperationPromise(FileService.ExecuteSync(toSyncConfig()))
    if (syncResult) {
      result.value = syncResult
      if (syncResult.status === 'cancelled') {
        message.warning('同步已取消')
      } else if (syncResult.errors.length > 0) {
        message.warning(`同步完成，但 ${syncResult.errors.length} 项失败`)
      } else {
        message.success('同步完成')
      }
    }
    plan.value = null
    configDirty.value = false
    notifyDirectoriesChanged([config.value.targetDir])
  } catch (err) {
    if (!isOperationCancelledError(err)) {
      message.error(`同步失败：${friendlyError(err)}`)
    } else {
      message.warning('同步已取消')
    }
  } finally {
    executing.value = false
  }
}

function friendlyError(err: unknown): string {
  const text = err instanceof Error ? err.message : String(err)
  return text || '未知错误'
}

function rowKey(row: { relPath: string }): string {
  return row.relPath
}

function errorRowKey(row: { relPath: string, op: string }): string {
  return `${row.relPath}:${row.op}`
}

const copyColumns = [
  { title: '路径', key: 'relPath', ellipsis: { tooltip: true } },
  { title: '原因', key: 'reason', width: 110 },
]

const deleteColumns = [
  { title: '路径', key: 'relPath', ellipsis: { tooltip: true } },
  {
    title: '类型',
    key: 'isDir',
    width: 110,
    render: (row: { isDir: boolean }) => (row.isDir ? '目录' : '文件'),
  },
]

const errorColumns = [
  { title: '路径', key: 'relPath', ellipsis: { tooltip: true } },
  { title: '阶段', key: 'op', width: 90, render: (row: { op: string }) => syncOpLabel(row.op) },
  { title: '错误', key: 'error', ellipsis: { tooltip: true } },
]

const copyRows = computed(() => plan.value?.copy ?? [])
const deleteRows = computed(() => plan.value?.delete ?? [])
const planErrorRows = computed(() => plan.value?.errors ?? [])
const resultErrorRows = computed(() => result.value?.errors ?? [])
const errorRows = computed(() => (plan.value ? planErrorRows.value : resultErrorRows.value))

const executeConfirmText = computed(() => (plan.value ? syncExecuteConfirmText(plan.value) : ''))
const resultSummaryText = computed(() => (result.value ? syncResultSummary(result.value) : ''))
const planSummaryText = computed(() => (plan.value ? syncPlanSummary(plan.value) : ''))

const confirmButton = computed(() => h(
  NButton,
  {
    type: hasDeleteActions.value ? 'warning' : 'primary',
    size: 'small',
    loading: executing.value,
    disabled: !canExecute.value,
    onClick: () => execute(),
  },
  { default: () => '开始同步' },
))
</script>

<template>
  <div class="sync-tool-page">
    <div class="sync-config">
      <div class="sync-config-row">
        <span class="sync-config-label">源目录</span>
        <div class="sync-path-group">
          <NInput
            :value="config.sourceDir"
            size="small"
            placeholder="要同步出去的目录"
            :disabled="running"
            @update:value="(val: string) => updateConfig('sourceDir', val)"
          />
          <NButton size="small" :disabled="running" @click="browseSource">
            <template #icon>
              <NIcon><FolderOpen20Regular /></NIcon>
            </template>
            浏览
          </NButton>
        </div>
      </div>

      <div class="sync-config-row">
        <span class="sync-config-label">同步目录</span>
        <div class="sync-path-group">
          <NInput
            :value="config.targetDir"
            size="small"
            placeholder="内容同步到的目录"
            :disabled="running"
            @update:value="(val: string) => updateConfig('targetDir', val)"
          />
          <NButton size="small" :disabled="running" @click="browseTarget">
            <template #icon>
              <NIcon><FolderOpen20Regular /></NIcon>
            </template>
            浏览
          </NButton>
        </div>
      </div>

      <div class="sync-config-row">
        <span class="sync-config-label">同步策略</span>
        <NRadioGroup
          :value="config.mode"
          size="small"
          :disabled="running"
          @update:value="(val: 'mirror' | 'incremental') => updateConfig('mode', val)"
        >
          <NRadioButton value="mirror" :label="syncModeLabel('mirror')" />
          <NRadioButton value="incremental" :label="syncModeLabel('incremental')" />
        </NRadioGroup>
        <span v-if="config.mode === 'mirror'" class="sync-config-hint">将删除同步目录中源不存在的文件（移入回收站）</span>
      </div>

      <div class="sync-config-row">
        <span class="sync-config-label">判重维度</span>
        <NCheckboxGroup
          :value="[
            ...(config.compareSize ? ['size'] : []),
            ...(config.compareModTime ? ['modTime'] : []),
            ...(config.compareHash ? ['hash'] : []),
          ]"
          :disabled="running"
          @update:value="(values: Array<string | number>) => {
            const selected = new Set(values.map(String))
            updateConfig('compareSize', selected.has('size'))
            updateConfig('compareModTime', selected.has('modTime'))
            updateConfig('compareHash', selected.has('hash'))
          }"
        >
          <NCheckbox value="size" label="大小" />
          <NCheckbox value="modTime" label="修改时间" />
          <NTooltip trigger="hover">
            <template #trigger>
              <NCheckbox value="hash" label="内容哈希" />
            </template>
            读取文件内容计算 SHA-256，大文件较慢但最准确
          </NTooltip>
        </NCheckboxGroup>
        <span class="sync-config-hint">文件相对路径始终作为匹配依据</span>
      </div>

      <div class="sync-config-row">
        <span class="sync-config-label">忽略隐藏文件</span>
        <NSwitch
          :value="config.ignoreHidden"
          size="small"
          :disabled="running"
          @update:value="(val: boolean) => updateConfig('ignoreHidden', val)"
        />
      </div>

      <div class="sync-config-row">
        <span class="sync-config-label">忽略规则</span>
        <NDynamicTags
          class="sync-ignore-tags"
          :value="config.ignorePatterns"
          :disabled="running"
          @update:value="(val: Array<string | number>) => updateConfig('ignorePatterns', val.map(String).filter(Boolean))"
        />
        <span class="sync-config-hint">按名字匹配，支持通配符（如 node_modules、*.tmp）</span>
      </div>
    </div>

    <div class="sync-actions">
      <NButton
        size="small"
        type="primary"
        secondary
        :loading="analyzing"
        :disabled="!canAnalyze"
        @click="analyze"
      >
        <template #icon>
          <NIcon><Search20Regular /></NIcon>
        </template>
        分析
      </NButton>
      <NTooltip :disabled="canExecute" trigger="hover">
        <template #trigger>
          <component :is="confirmButton" />
        </template>
        {{ configDirty ? '配置已变化，请重新分析' : configIssueText || '请先执行分析' }}
      </NTooltip>
      <span v-if="configIssueText" class="sync-config-error">{{ configIssueText }}</span>
      <span v-else-if="plan && !configDirty" class="sync-plan-summary">{{ planSummaryText }}</span>
      <span v-else-if="plan && configDirty" class="sync-config-hint">配置已变化，请重新分析</span>
    </div>

    <div v-if="result" class="sync-result">
      <span class="sync-result-summary" :class="{ cancelled: result.status === 'cancelled' }">
        {{ result.status === 'cancelled' ? '已取消 · ' : '' }}{{ resultSummaryText }}
      </span>
    </div>

    <div v-if="plan" class="sync-plan">
      <div class="sync-plan-section">
        <div class="sync-plan-title">将复制（{{ copyRows.length }} 项）</div>
        <NDataTable
          class="sync-table"
          size="small"
          :columns="copyColumns"
          :data="copyRows"
          :bordered="false"
          :row-key="rowKey"
          flex-height
        />
      </div>
      <div v-if="deleteRows.length > 0" class="sync-plan-section">
        <div class="sync-plan-title sync-plan-title-delete">将删除（{{ deleteRows.length }} 项，移入回收站）</div>
        <NDataTable
          class="sync-table"
          size="small"
          :columns="deleteColumns"
          :data="deleteRows"
          :bordered="false"
          :row-key="rowKey"
          flex-height
        />
      </div>
      <div v-if="errorRows.length > 0" class="sync-plan-section">
        <div class="sync-plan-title sync-plan-title-error">问题（{{ errorRows.length }} 项）</div>
        <NDataTable
          class="sync-table"
          size="small"
          :columns="errorColumns"
          :data="errorRows"
          :bordered="false"
          :row-key="errorRowKey"
          flex-height
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.sync-tool-page {
  height: 100%;
  overflow: auto;
  box-sizing: border-box;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sync-config {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex-shrink: 0;
}

.sync-config-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 36px;
  flex-wrap: wrap;
}

.sync-config-label {
  width: 84px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--n-text-color-2);
  text-align: right;
}

.sync-path-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 320px;
  max-width: 560px;
}

.sync-path-group .n-input {
  flex: 1;
}

.sync-config-hint {
  font-size: 12px;
  color: var(--n-text-color-3);
}

.sync-config-error {
  font-size: 12px;
  color: var(--n-error-color);
}

.sync-ignore-tags {
  flex: 1;
  min-width: 260px;
  max-width: 560px;
}

.sync-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.sync-plan-summary {
  font-size: 13px;
  color: var(--n-text-color-2);
}

.sync-result {
  flex-shrink: 0;
}

.sync-result-summary {
  font-size: 13px;
  color: var(--n-text-color-2);
}

.sync-result-summary.cancelled {
  color: var(--n-warning-color);
}

.sync-plan {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sync-plan-section {
  flex: 1;
  min-height: 120px;
  display: flex;
  flex-direction: column;
}

.sync-plan-title {
  font-size: 13px;
  font-weight: 500;
  margin-bottom: 4px;
  color: var(--n-text-color-2);
}

.sync-plan-title-delete {
  color: var(--n-warning-color);
}

.sync-plan-title-error {
  color: var(--n-error-color);
}

.sync-table {
  flex: 1;
  min-height: 0;
}
</style>
