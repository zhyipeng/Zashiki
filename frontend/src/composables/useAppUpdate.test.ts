import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(), check: vi.fn(), download: vi.fn(), restart: vi.fn(), on: vi.fn(), off: vi.fn(),
}))
vi.mock('../../bindings/zashiki/internal/appupdate', () => ({
  UpdateService: { GetStatus: mocks.getStatus, Check: mocks.check, Download: mocks.download, Restart: mocks.restart },
}))
vi.mock('@wailsio/runtime', () => ({ Events: { On: mocks.on } }))

const idle = { currentVersion: '1.0.0', enabled: true, reason: '', state: 'idle', version: '', notes: '', size: 0 }
const available = { ...idle, state: 'available', version: '1.1.0', notes: '新版说明', size: 1024 }
let cleanup: (() => void) | undefined

beforeEach(() => {
  vi.resetModules()
  vi.resetAllMocks()
  vi.useFakeTimers()
  mocks.getStatus.mockResolvedValue(idle)
  mocks.check.mockResolvedValue(available)
  mocks.download.mockResolvedValue(undefined)
  mocks.restart.mockResolvedValue(undefined)
  mocks.on.mockReturnValue(mocks.off)
})

afterEach(() => {
  cleanup?.()
  cleanup = undefined
  vi.useRealTimers()
})

async function start() {
  const { useAppUpdate } = await import('./useAppUpdate')
  const update = useAppUpdate()
  cleanup = update.start()
  await flushPromises()
  return update
}

describe('application updates', () => {
  it('checks on startup and asks for confirmation before downloading', async () => {
    const update = await start()
    expect(mocks.check).toHaveBeenCalledOnce()
    expect(update.visible.value).toBe(true)
    expect(update.status.value.notes).toBe('新版说明')
    expect(mocks.download).not.toHaveBeenCalled()
    expect(mocks.restart).not.toHaveBeenCalled()
    mocks.getStatus.mockResolvedValue({ ...available, state: 'ready' })
    await update.install()
    expect(mocks.download).toHaveBeenCalledOnce()
    expect(mocks.restart).toHaveBeenCalledOnce()
  })

  it('keeps network failures silent until the user checks manually', async () => {
    mocks.check.mockRejectedValue(new Error('网络不可用'))
    const update = await start()
    expect(update.error.value).toBe('')
    expect(update.visible.value).toBe(false)
    await update.check(true)
    expect(update.error.value).toContain('网络不可用')
  })

  it('defers a dismissed release but allows reopening it manually', async () => {
    const update = await start()
    update.dismiss()
    expect(update.visible.value).toBe(false)
    await vi.advanceTimersByTimeAsync(6 * 60 * 60 * 1000)
    expect(update.visible.value).toBe(false)
    await update.check(true)
    expect(update.visible.value).toBe(true)
  })

  it('does not restart if verification or download fails', async () => {
    const update = await start()
    mocks.download.mockRejectedValueOnce(new Error('校验失败'))
    await update.install()
    expect(update.error.value).toContain('校验失败')
    expect(mocks.restart).not.toHaveBeenCalled()
    expect(update.busy.value).toBe(false)
  })

  it('retains the ready update when restart fails, without checking again', async () => {
    const update = await start()
    mocks.getStatus.mockResolvedValue({ ...available, state: 'ready' })
    mocks.restart.mockRejectedValueOnce(new Error('助手未就绪'))
    await update.install()
    expect(update.ready.value).toBe(true)
    expect(update.error.value).toContain('助手未就绪')
    await update.check(true)
    expect(mocks.check).toHaveBeenCalledOnce()
    await update.install()
    expect(mocks.restart).toHaveBeenCalledTimes(2)
  })

  it('prevents duplicate installs and dismissal while a download is active', async () => {
    const update = await start()
    let finish: (() => void) | undefined
    mocks.download.mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    const downloading = update.install()
    await update.install()
    update.dismiss()
    expect(mocks.download).toHaveBeenCalledOnce()
    expect(update.visible.value).toBe(true)
    finish?.()
    await downloading
  })

  it('reports current version on manual check and skips disabled builds', async () => {
    mocks.check.mockResolvedValue({ ...idle, state: 'up-to-date' })
    const update = await start()
    expect(update.visible.value).toBe(false)
    await update.check(true)
    expect(update.feedback.value).toBe('当前已是最新版本')
    update.status.value = { ...idle, enabled: false, reason: '开发模式不执行自动更新' }
    await update.check(true)
    expect(mocks.check).toHaveBeenCalledTimes(2)
  })

  it('shows progress and removes listeners and periodic checks on unmount', async () => {
    const update = await start()
    const onProgress = mocks.on.mock.calls[0][1]
    onProgress({ data: { written: 20, total: 80 } })
    expect(update.percent.value).toBe(25)
    onProgress({ data: { written: 20, total: 0 } })
    expect(update.percent.value).toBeNull()
    cleanup?.()
    cleanup = undefined
    expect(mocks.off).toHaveBeenCalledOnce()
    await vi.advanceTimersByTimeAsync(6 * 60 * 60 * 1000)
    expect(mocks.check).toHaveBeenCalledOnce()
  })
})
