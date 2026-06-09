import { reactive, ref } from 'vue'
import { SettingsService } from '../../bindings/zashiki/internal/settings'

export type ThemeMode = 'light' | 'dark' | 'system'

export interface Settings {
  showHiddenFiles: boolean
  themeMode: ThemeMode
}

const state = reactive<Settings>({
  showHiddenFiles: false,
  themeMode: 'system',
})

const loaded = ref(false)
let initPromise: Promise<void> | null = null

export function initSettings(): Promise<void> {
  if (initPromise) return initPromise

  initPromise = SettingsService.GetSettings().then((settings) => {
    state.showHiddenFiles = settings.showHiddenFiles
    state.themeMode = normalizeThemeMode(settings.themeMode)
    loaded.value = true
  }).catch((err) => {
    console.error('Failed to load settings:', err)
    loaded.value = true
  })

  return initPromise
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
