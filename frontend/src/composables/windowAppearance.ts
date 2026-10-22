import { System } from '@wailsio/runtime'
import type { GlobalThemeOverrides } from 'naive-ui'

export function configureWindowAppearance(isDark = false): GlobalThemeOverrides | undefined {
  if (!System.IsMac()) {
    delete document.documentElement.dataset.windowMaterial
    return undefined
  }

  document.documentElement.dataset.windowMaterial = 'glass'
  return {
    DataTable: {
      tdColor: 'transparent',
      tdColorSorting: 'var(--app-surface-background)',
      tdColorStriped: 'var(--app-surface-background)',
      thColor: 'var(--app-surface-background)',
      thColorSorting: 'var(--app-surface-background)',
      ...(!isDark ? {
        borderColor: 'rgba(31, 41, 55, 0.07)',
        thColor: 'rgba(243, 245, 248, 0.62)',
        thColorHover: 'rgba(235, 239, 244, 0.72)',
        thColorSorting: 'rgba(235, 239, 244, 0.72)',
        tdColorHover: 'rgba(31, 41, 55, 0.035)',
      } : {}),
    },
    ...(!isDark ? {
      Input: {
        color: 'rgba(255, 255, 255, 0.62)',
        colorFocus: 'rgba(255, 255, 255, 0.9)',
        border: '1px solid rgba(31, 41, 55, 0.12)',
      },
      Divider: { color: 'rgba(31, 41, 55, 0.08)' },
    } : {}),
  }
}
