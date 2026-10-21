import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Settings } from './useSettings'

const backend = vi.hoisted(() => ({
  GetSettings: vi.fn(),
  SaveSettings: vi.fn(async (_value: Settings) => {}),
}))
vi.mock('../../bindings/zashiki/internal/settings', () => ({ SettingsService: backend }))

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
})

describe('排序设置', () => {
  it('启动时恢复排序，保存新选择后下次启动继续生效', async () => {
    let stored: Partial<Settings> = { fileSort: { columnKey: 'size', order: 'descend' } }
    backend.GetSettings.mockImplementation(async () => stored)
    backend.SaveSettings.mockImplementation(async (value) => { stored = JSON.parse(JSON.stringify(value)) })
    const first = await import('./useSettings')
    await first.initSettings()
    expect(first.useSettings().settings.fileSort).toEqual(stored.fileSort)
    first.useSettings().updateSetting('fileSort', { columnKey: 'modTime', order: 'ascend' })
    vi.resetModules()
    const restarted = await import('./useSettings')
    await restarted.initSettings()
    expect(restarted.useSettings().settings.fileSort).toEqual({ columnKey: 'modTime', order: 'ascend' })
  })

  it.each([undefined, null, {}, { columnKey: 'unknown', order: 'ascend' }, { columnKey: 'name', order: false }])('兼容旧配置和无效排序：%j', async (fileSort) => {
    backend.GetSettings.mockResolvedValue({ fileSort })
    const { initSettings, useSettings } = await import('./useSettings')
    await initSettings()
    expect(useSettings().settings.fileSort).toEqual({ columnKey: '', order: '' })
  })
})
