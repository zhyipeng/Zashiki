import { afterEach, describe, expect, it, vi } from 'vitest'
import { h } from 'vue'
import { mount } from '@vue/test-utils'
import { darkTheme, NConfigProvider, NDataTable, NInput } from 'naive-ui'
import { System } from '@wailsio/runtime'
import { configureWindowAppearance } from './windowAppearance'

vi.mock('@wailsio/runtime', () => ({ System: { IsMac: vi.fn() } }))

afterEach(() => {
  delete document.documentElement.dataset.windowMaterial
  vi.clearAllMocks()
})

describe('窗口毛玻璃', () => {
  it('非 macOS 环境保留默认主题并清除透明背景标记', () => {
    vi.mocked(System.IsMac).mockReturnValue(false)
    document.documentElement.dataset.windowMaterial = 'glass'

    expect(configureWindowAppearance()).toBeUndefined()
    expect(document.documentElement.dataset.windowMaterial).toBeUndefined()
  })

  it.each([['浅色', null], ['深色', darkTheme]] as const)('%s主题的表格透出原生材质，保留悬停反馈', (_name, theme) => {
    vi.mocked(System.IsMac).mockReturnValue(true)
    const overrides = configureWindowAppearance(theme !== null)
    const wrapper = mount(NConfigProvider, {
      props: { theme, themeOverrides: overrides },
      slots: {
        default: () => h(NDataTable, {
          columns: [{ title: '名称', key: 'name' }],
          data: [{ name: 'example.txt' }],
        }),
      },
    })
    try {
      expect(document.documentElement.dataset.windowMaterial).toBe('glass')
      const table = wrapper.find('.n-data-table').element as HTMLElement
      expect(table.style.getPropertyValue('--n-td-color')).toBe('transparent')
      expect(table.style.getPropertyValue('--n-th-color')).toBe(
        theme ? 'var(--app-surface-background)' : 'rgba(243, 245, 248, 0.62)',
      )
      if (!theme) {
        expect(table.style.getPropertyValue('--n-border-color')).toBe('rgba(31, 41, 55, 0.07)')
      }
      expect(table.style.getPropertyValue('--n-td-color-hover')).not.toBe('transparent')
      expect(wrapper.text()).toContain('example.txt')
    } finally {
      wrapper.unmount()
    }
  })

  it('切换到深色时恢复输入框和分隔线的深色主题', async () => {
    vi.mocked(System.IsMac).mockReturnValue(true)
    const wrapper = mount(NConfigProvider, {
      props: { theme: null as typeof darkTheme | null, themeOverrides: configureWindowAppearance(false) },
      slots: { default: () => h(NInput, { value: '/Users/example' }) },
    })
    try {
      const input = () => wrapper.find('.n-input').element as HTMLElement
      expect(input().style.getPropertyValue('--n-color')).toBe('rgba(255, 255, 255, 0.62)')
      const darkOverrides = configureWindowAppearance(true)
      await wrapper.setProps({ theme: darkTheme, themeOverrides: darkOverrides })
      expect(darkOverrides?.Input).toBeUndefined()
      expect(darkOverrides?.Divider).toBeUndefined()
      expect(input().style.getPropertyValue('--n-color')).not.toBe('rgba(255, 255, 255, 0.62)')
    } finally {
      wrapper.unmount()
    }
  })
})
