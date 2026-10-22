import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import { NButton, NModal } from 'naive-ui'
import AppUpdate from './AppUpdate.vue'

const mocks = vi.hoisted(() => ({ state: null as any, install: vi.fn(), dismiss: vi.fn(), start: vi.fn(), cleanup: vi.fn() }))
vi.mock('../composables/useAppUpdate', () => ({ useAppUpdate: () => mocks.state }))
let wrapper: VueWrapper | undefined

beforeEach(() => {
  vi.resetAllMocks()
  mocks.start.mockReturnValue(mocks.cleanup)
  mocks.state = {
    status: ref({ currentVersion: '1.0.0', version: '1.1.0', notes: '<img src=x onerror=alert(1)>', size: 1024 }),
    visible: ref(true), busy: ref(false), error: ref(''), percent: ref(null), ready: ref(false),
    install: mocks.install, dismiss: mocks.dismiss, start: mocks.start,
  }
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = undefined
})

describe('AppUpdate', () => {
  it('renders release notes as text and waits for the confirmation button', async () => {
    wrapper = mount(AppUpdate, { attachTo: document.body })
    await flushPromises()
    expect(mocks.start).toHaveBeenCalledOnce()
    expect(mocks.install).not.toHaveBeenCalled()
    expect(document.querySelector('.release-notes')?.textContent).toContain('<img src=x onerror=alert(1)>')
    expect(document.querySelector('.release-notes img')).toBeNull()
    const confirm = wrapper.findAllComponents(NButton).find(button => button.text() === '下载并重启安装')
    expect(confirm).toBeDefined()
    await confirm!.trigger('click')
    expect(mocks.install).toHaveBeenCalledOnce()
    wrapper.unmount()
    wrapper = undefined
    expect(mocks.cleanup).toHaveBeenCalledOnce()
  })

  it('prevents closing during download and offers restart retry after a failure', async () => {
    mocks.state.busy.value = true
    wrapper = mount(AppUpdate, { attachTo: document.body })
    await flushPromises()
    expect(wrapper.findComponent(NModal).props('closable')).toBe(false)
    const later = wrapper.findAllComponents(NButton).find(button => button.text() === '稍后提醒')
    expect(later?.attributes('disabled')).toBeDefined()
    mocks.state.busy.value = false
    mocks.state.ready.value = true
    mocks.state.error.value = '更新助手启动失败'
    await flushPromises()
    expect(document.body.textContent).toContain('更新助手启动失败')
    const retry = wrapper.findAllComponents(NButton).find(button => button.text() === '重试重启安装')
    await retry!.trigger('click')
    expect(mocks.install).toHaveBeenCalledOnce()
  })
})
