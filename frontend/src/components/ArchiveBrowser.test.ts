import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { CancellablePromise } from '@wailsio/runtime'
import type { ArchiveEntry, ArchivePage, FilePreview } from '../../bindings/zashiki/internal/filemanager'

vi.mock('../../bindings/zashiki/internal/filemanager', () => ({
  FileService: { ListArchive: vi.fn(), GetArchiveEntryPreview: vi.fn(), ExtractArchive: vi.fn() },
}))
vi.mock('../composables/useOperationProgress', () => ({
  trackOperationPromise: (promise: Promise<unknown>) => promise,
  isOperationCancelledError: (err: unknown) => String(err).includes('cancelled'),
}))
vi.mock('naive-ui', async importOriginal => ({
  ...await importOriginal<typeof import('naive-ui')>(),
  useMessage: () => ({ success: vi.fn() }),
}))

import { FileService } from '../../bindings/zashiki/internal/filemanager'
import ArchiveBrowser from './ArchiveBrowser.vue'
import FilePreviewModal from './FilePreviewModal.vue'

const Modal = defineComponent({
  props: ['show'],
  setup: (props, { slots }) => () => props.show ? h('section', slots.default?.()) : null,
})
const Table = defineComponent({
  props: ['data', 'rowProps', 'checkedRowKeys'],
  emits: ['update:checked-row-keys'],
  setup: (props, { emit }) => () => h('table', props.data.map((row: ArchiveEntry) => h('tr', props.rowProps(row), [
    h('td', row.name), h('td', [h('input', { type: 'checkbox', onChange: () => emit('update:checked-row-keys', [row.path]) })]),
  ]))),
})
function cancellable<T>(value: T) { return CancellablePromise.resolve(value) }
function entry(path: string, isDir = false): ArchiveEntry { return { path, name: path.split('/').pop()!, size: 12, isDir, isLink: false } }
let wrapper: VueWrapper | undefined

function render() {
  wrapper = mount(ArchiveBrowser, {
    props: { show: true, archivePath: '/tmp/archive.zip', name: 'archive.zip', separator: '/' },
    global: { stubs: { NModal: Modal, Modal, NDataTable: Table, DataTable: Table, FilePreviewModal: true } },
  })
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(FileService.ListArchive).mockImplementation(() => cancellable({ entries: [entry('docs', true), entry('readme.txt')], total: 2 }))
  vi.mocked(FileService.ExtractArchive).mockImplementation(() => cancellable('/tmp/archive'))
  vi.mocked(FileService.GetArchiveEntryPreview).mockImplementation(() => cancellable({ name: 'readme.txt', path: 'readme.txt', kind: 'text', content: 'hello', mimeType: 'text/plain', size: 5, version: '', dataUrl: '', truncated: false, message: '' } as FilePreview))
})
afterEach(() => wrapper?.unmount())

describe('ArchiveBrowser', () => {
  it('浏览目录并返回根目录', async () => {
    const view = render()
    await flushPromises()
    expect(FileService.ListArchive).toHaveBeenLastCalledWith('/tmp/archive.zip', '', 0, 200)
    await view.findAll('tr')[0].trigger('dblclick')
    await flushPromises()
    expect(FileService.ListArchive).toHaveBeenLastCalledWith('/tmp/archive.zip', 'docs', 0, 200)
    await view.findAll('button').find(button => button.text() === '上一级')!.trigger('click')
    await flushPromises()
    expect(FileService.ListArchive).toHaveBeenLastCalledWith('/tmp/archive.zip', '', 0, 200)
  })
  it('按需打开只读预览', async () => {
    const view = render()
    await flushPromises()
    expect(FileService.GetArchiveEntryPreview).not.toHaveBeenCalled()
    await view.findAll('tr')[1].trigger('dblclick')
    await flushPromises()
    expect(FileService.GetArchiveEntryPreview).toHaveBeenCalledWith('/tmp/archive.zip', 'readme.txt')
    const modal = view.findComponent(FilePreviewModal)
    expect(modal.props('readonly')).toBe(true)
    expect(modal.props('preview')?.content).toBe('hello')
  })
  it('解压选中目录并通知目标目录刷新', async () => {
    const view = render()
    await flushPromises()
    await view.find('input[type="checkbox"]').trigger('change')
    await view.findAll('button').find(button => button.text().startsWith('解压选中项'))!.trigger('click')
    await flushPromises()
    expect(FileService.ExtractArchive).toHaveBeenCalledWith('/tmp/archive.zip', '/tmp/archive', ['docs'])
    expect(view.emitted('extracted')).toEqual([['/tmp']])
  })
  it('将空选择传给全部解压，并展示后端错误', async () => {
    vi.mocked(FileService.ExtractArchive).mockImplementation(() => CancellablePromise.reject(new Error('目标文件夹已存在')))
    const view = render()
    await flushPromises()
    await view.findAll('button').find(button => button.text() === '全部解压')!.trigger('click')
    await flushPromises()
    expect(FileService.ExtractArchive).toHaveBeenCalledWith('/tmp/archive.zip', '/tmp/archive', [])
    expect(view.text()).toContain('目标文件夹已存在')
    expect(view.emitted('extracted')).toBeUndefined()
  })
  it('切换压缩包时取消旧请求并忽略过期响应', async () => {
    let resolveOld!: (value: ArchivePage) => void
    const oldRequest = new CancellablePromise<ArchivePage>(resolve => { resolveOld = resolve })
    vi.spyOn(oldRequest, 'cancel').mockReturnValue(CancellablePromise.resolve())
    vi.mocked(FileService.ListArchive).mockReturnValueOnce(oldRequest)
    const view = render()
    await view.setProps({ archivePath: '/tmp/new.zip', name: 'new.zip' })
    await flushPromises()
    resolveOld({ entries: [entry('stale.txt')], total: 1 })
    await flushPromises()
    expect(oldRequest.cancel).toHaveBeenCalled()
    expect(view.text()).not.toContain('stale.txt')
    expect(view.text()).toContain('readme.txt')
  })
})
