import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { NMessageProvider } from 'naive-ui'
import { defineComponent, h } from 'vue'
import AccessCodeSettings from './AccessCodeSettings.vue'

const { setAccessCode } = vi.hoisted(() => ({ setAccessCode: vi.fn() }))
vi.mock('../../../bindings/zashiki/internal/lanshare', () => ({
  LanShareService: { SetAccessCode: setAccessCode },
}))

function mountSettings(accessCode = '') {
  const saved = vi.fn()
  const wrapper = mount(defineComponent({
    setup: () => () => h(NMessageProvider, null, {
      default: () => h(AccessCodeSettings, { accessCode, onSaved: saved }),
    }),
  }), { attachTo: document.body })
  return { wrapper, saved }
}

beforeEach(() => {
  vi.resetAllMocks()
  setAccessCode.mockResolvedValue(undefined)
})

describe('AccessCodeSettings', () => {
  it('saves a custom short code and notifies the parent to refresh URLs', async () => {
    const { wrapper, saved } = mountSettings()
    try {
      expect(wrapper.get('button').attributes('disabled')).toBeDefined()
      await wrapper.get('input').setValue('abc')
      await wrapper.get('button').trigger('click')
      await flushPromises()
      expect(setAccessCode).toHaveBeenCalledWith('abc')
      expect(saved).toHaveBeenCalledOnce()
    } finally { wrapper.unmount() }
  })

  it('clears the saved code to restore random mode using Enter', async () => {
    const { wrapper, saved } = mountSettings('abc')
    try {
      expect(wrapper.get('input').element.value).toBe('abc')
      await wrapper.get('input').setValue('')
      await wrapper.get('input').trigger('keydown', { key: 'Enter' })
      await flushPromises()
      expect(setAccessCode).toHaveBeenCalledWith('')
      expect(saved).toHaveBeenCalledOnce()
    } finally { wrapper.unmount() }
  })

  it.each(['a/b', 'a b', '中文', 'a'.repeat(33)])('rejects invalid code %j', async code => {
    const { wrapper } = mountSettings()
    try {
      await wrapper.get('input').setValue(code)
      expect(wrapper.get('button').attributes('disabled')).toBeDefined()
      await wrapper.get('input').trigger('keydown', { key: 'Enter' })
      expect(setAccessCode).not.toHaveBeenCalled()
      expect(wrapper.text()).toContain('访问码只能包含')
    } finally { wrapper.unmount() }
  })

  it('keeps the draft and allows retry after a save error', async () => {
    setAccessCode.mockRejectedValueOnce(new Error('写入失败'))
    const { wrapper, saved } = mountSettings('abc')
    try {
      await wrapper.get('input').setValue('new')
      await wrapper.get('button').trigger('click')
      await flushPromises()
      expect(saved).not.toHaveBeenCalled()
      expect(wrapper.get('input').element.value).toBe('new')
      expect(wrapper.get('button').attributes('disabled')).toBeUndefined()
      await wrapper.get('button').trigger('click')
      await flushPromises()
      expect(saved).toHaveBeenCalledOnce()
    } finally { wrapper.unmount() }
  })
})
