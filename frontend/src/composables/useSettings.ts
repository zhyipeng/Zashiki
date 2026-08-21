import { reactive, ref } from 'vue'
import { SettingsService } from '../../bindings/zashiki/internal/settings'

export type ThemeMode = 'light' | 'dark' | 'system'

export interface SyncToolSettings {
  sourceDir: string
  targetDir: string
  mode: 'mirror' | 'incremental'
  compareSize: boolean
  compareModTime: boolean
  compareHash: boolean
  ignoreHidden: boolean
  ignorePatterns: string[]
}

export interface Settings {
  showHiddenFiles: boolean
  themeMode: ThemeMode
  terminalProgram: string
  defaultEditor: string
  pinnedQuickAccessPaths: string[]
  syncTool: SyncToolSettings
}

function defaultSyncToolSettings(): SyncToolSettings {
  return {
    sourceDir: '',
    targetDir: '',
    mode: 'incremental',
    compareSize: true,
    compareModTime: true,
    compareHash: false,
    ignoreHidden: true,
    ignorePatterns: [],
  }
}

function normalizeSyncToolSettings(value: unknown): SyncToolSettings {
  const fallback = defaultSyncToolSettings()
  if (!value || typeof value !== 'object') return fallback
  const raw = value as Record<string, unknown>
  const mode = raw.mode === 'mirror' ? 'mirror' : 'incremental'
  const compareSize = raw.compareSize === true
  const compareModTime = raw.compareModTime === true
  const compareHash = raw.compareHash === true
  const patterns = Array.isArray(raw.ignorePatterns)
    ? raw.ignorePatterns.filter((pattern): pattern is string => typeof pattern === 'string' && pattern.length > 0)
    : []
  return {
    sourceDir: typeof raw.sourceDir === 'string' ? raw.sourceDir : '',
    targetDir: typeof raw.targetDir === 'string' ? raw.targetDir : '',
    mode,
    // The backend rejects configs without any dimension; re-enable size as a safe default.
    compareSize: compareSize || (!compareModTime && !compareHash),
    compareModTime,
    compareHash,
    ignoreHidden: raw.ignoreHidden !== false,
    ignorePatterns: patterns,
  }
}

const state = reactive<Settings>({
  showHiddenFiles: false,
  themeMode: 'system',
  terminalProgram: '',
  defaultEditor: '',
  pinnedQuickAccessPaths: [],
  syncTool: defaultSyncToolSettings(),
})

const loaded = ref(false)
let initPromise: Promise<void> | null = null

export function initSettings(): Promise<void> {
  if (initPromise) return initPromise

  const promise = SettingsService.GetSettings().then((settings) => {
    state.showHiddenFiles = settings.showHiddenFiles
    state.themeMode = normalizeThemeMode(settings.themeMode)
    state.terminalProgram = settings.terminalProgram || ''
    state.defaultEditor = settings.defaultEditor || ''
    state.pinnedQuickAccessPaths = Array.isArray(settings.pinnedQuickAccessPaths)
      ? settings.pinnedQuickAccessPaths.filter((path): path is string => typeof path === 'string' && path.length > 0)
      : []
    state.syncTool = normalizeSyncToolSettings(settings.syncTool)
    loaded.value = true
  }).catch((err) => {
    console.error('Failed to load settings:', err)
    loaded.value = true
  })

  initPromise = promise
  return promise
}

function normalizeThemeMode(value: unknown): ThemeMode {
  return value === 'light' || value === 'dark' || value === 'system' ? value : 'system'
}

function persist() {
  SettingsService.SaveSettings({ ...state }).catch((err) => {
    console.error('Failed to save settings:', err)
  })
}

export function useSettings() {
  return {
    settings: state,
    loaded,
    updateSetting: <K extends keyof Settings>(key: K, value: Settings[K]) => {
      state[key] = value
      persist()
    },
  }
}
