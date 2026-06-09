import { reactive, ref } from 'vue'
import { SettingsService } from '../../bindings/zashiki/internal/settings'

export interface Settings {
  showHiddenFiles: boolean
}

const state = reactive<Settings>({
  showHiddenFiles: false,
})

const loaded = ref(false)
let initPromise: Promise<void> | null = null

export function initSettings(): Promise<void> {
  if (initPromise) return initPromise

  initPromise = SettingsService.GetSettings().then((settings) => {
    state.showHiddenFiles = settings.showHiddenFiles
    loaded.value = true
  }).catch((err) => {
    console.error('Failed to load settings:', err)
    loaded.value = true
  })

  return initPromise
}

function persist() {
  SettingsService.SaveSettings({ ...state } as any).catch((err) => {
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
