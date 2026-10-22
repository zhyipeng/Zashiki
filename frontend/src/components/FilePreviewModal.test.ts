import { describe, expect, it } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'
import FilePreviewModal from './FilePreviewModal.vue'

const Modal = defineComponent({ setup: (_, { slots }) => () => h('section', slots.default?.()) })
const preview: FilePreview = {
  name: 'readme.txt', path: 'readme.txt', kind: 'text', mimeType: 'text/plain', size: 5,
  version: '', content: 'hello', dataUrl: '', truncated: false, message: '',
}

describe('文件预览编辑权限', () => {
  it('普通文本可以编辑', async () => {
    const wrapper = mount(FilePreviewModal, { props: { show: true, entry: null, preview }, global: { stubs: { NModal: Modal, Modal } } })
    expect(wrapper.text()).toContain('编辑')
    await wrapper.find('.preview-stage').trigger('dblclick')
    expect(wrapper.find('textarea').exists()).toBe(true)
    wrapper.unmount()
  })
  it('包内只读文本不显示编辑入口，双击也不能进入编辑', async () => {
    const wrapper = mount(FilePreviewModal, { props: { show: true, entry: null, preview, readonly: true }, global: { stubs: { NModal: Modal, Modal } } })
    expect(wrapper.text()).not.toContain('编辑')
    await wrapper.find('.preview-stage').trigger('dblclick')
    expect(wrapper.find('textarea').exists()).toBe(false)
    expect(wrapper.emitted('save')).toBeUndefined()
    wrapper.unmount()
  })
})
