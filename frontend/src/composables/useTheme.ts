import { computed, ref, watch } from 'vue'
import { Events, System } from '@wailsio/runtime'
import { useSettings } from './useSettings'

const systemDark = ref(false)
let initialized = false
let stopThemeWatch: (() => void) | null = null
let offWailsThemeChanged: (() => void) | null = null
let mediaQuery: MediaQueryList | null = null

const { settings } = useSettings()

const isDarkTheme = computed(() => {
  if (settings.themeMode === 'dark') return true
  if (settings.themeMode === 'light') return false
  return systemDark.value
})

export function useTheme() {
  return {
    isDarkTheme,
    mountTheme,
  }
}

function mountTheme() {
  if (initialized) {
    return cleanupTheme
  }
  initialized = true

  refreshSystemTheme()
  setupBrowserThemeListener()
  offWailsThemeChanged = Events.On('common:ThemeChanged', () => {
    refreshSystemTheme()
  })

  stopThemeWatch = watch(isDarkTheme, applyDocumentTheme, { immediate: true })

  return cleanupTheme
}

async function refreshSystemTheme() {
  try {
    systemDark.value = await System.IsDarkMode()
  } catch (err) {
    systemDark.value = getBrowserDarkMode()
  }
}

function setupBrowserThemeListener() {
  if (typeof window === 'undefined' || !window.matchMedia) return

  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  systemDark.value = mediaQuery.matches
  mediaQuery.addEventListener('change', onBrowserThemeChanged)
}

function onBrowserThemeChanged(event: MediaQueryListEvent) {
  systemDark.value = event.matches
}

function getBrowserDarkMode() {
  return typeof window !== 'undefined'
    && !!window.matchMedia
    && window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyDocumentTheme(isDark: boolean) {
  if (typeof document === 'undefined') return

  const theme = isDark ? 'dark' : 'light'
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

function cleanupTheme() {
  stopThemeWatch?.()
  stopThemeWatch = null
  offWailsThemeChanged?.()
  offWailsThemeChanged = null
  mediaQuery?.removeEventListener('change', onBrowserThemeChanged)
  mediaQuery = null
  initialized = false
}
