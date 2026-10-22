import { mkdtemp, readdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { CancellablePromise } from '@wailsio/runtime'

vi.mock('@wailsio/runtime', async (importOriginal) => ({
  ...await importOriginal<typeof import('@wailsio/runtime')>(),
  Clipboard: {},
  Events: { On: vi.fn(() => () => {}) },
}))
vi.mock('../../bindings/zashiki/internal/filemanager', () => ({
  FileService: {
    CreateArchive: vi.fn(),
    ListDirPage: async (path: string) => {
      const names = await readdir(path)
      return {
        entries: names.map(name => ({
          name, path: join(path, name), size: 0, modTime: '',
          isDir: false, isHidden: false, isSymlink: false, linkTarget: '',
        })),
        total: names.length,
      }
    },
  },
}))
vi.mock('../../bindings/zashiki/internal/nativefs', () => ({
  FileTransferService: { StartDrag: vi.fn(async () => 0) },
  ClipboardOperation: {},
}))
vi.mock('../../bindings/zashiki/internal/settings', () => ({
  SettingsService: { SaveSettings: vi.fn(async () => {}) },
}))
vi.mock('./FilePreviewModal.vue', () => ({ default: { render: () => null } }))
vi.mock('naive-ui', async (importOriginal) => ({
  ...await importOriginal<typeof import('naive-ui')>(),
  useMessage: () => ({ error: vi.fn() }),
}))

import FileTable from './FileTable.vue'
import ArchiveBrowser from './ArchiveBrowser.vue'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import { FileTransferService } from '../../bindings/zashiki/internal/nativefs'
import { SettingsService } from '../../bindings/zashiki/internal/settings'
import { useSettings } from '../composables/useSettings'

// 保留行事件及受控选择接口，避免虚拟列表依赖浏览器布局。
const DataTable = defineComponent({
  props: ['data', 'rowProps', 'checkedRowKeys', 'columns', 'onUpdate:checkedRowKeys', 'onUpdate:sorter'],
  setup(props) {
    return () => h('table', props.data.map((row: FileEntry) => h('tr', props.rowProps(row), [
      h('td', row.name),
      props.columns.some((column: { type?: string }) => column.type === 'selection')
        ? h('td', [h('input', {
          type: 'checkbox',
          class: 'n-checkbox',
          checked: props.checkedRowKeys.includes(row.path),
          onClick: () => props['onUpdate:checkedRowKeys'](
            props.checkedRowKeys.includes(row.path)
              ? props.checkedRowKeys.filter((path: string) => path !== row.path)
              : [...props.checkedRowKeys, row.path],
          ),
        })])
        : null,
    ])))
  },
})

const originalWails = (window as any)._wails
let dir = ''
let wrapper: VueWrapper | undefined

afterEach(async () => {
  wrapper?.unmount()
  wrapper = undefined
  if (dir) await rm(dir, { recursive: true, force: true })
  dir = ''
  useSettings().updateSetting('fileSort', { columnKey: '', order: '' })
  ;(window as any)._wails = originalWails
  vi.clearAllMocks()
})

async function setup(os: string, names = ['a.txt', 'b.txt'], stubs: Record<string, any> = {}) {
  ;(window as any)._wails = { environment: { OS: os } }
  dir = await mkdtemp(join(tmpdir(), 'file-table-selection-'))
  await Promise.all(names.map(name => writeFile(join(dir, name), '')))
  wrapper = mount(FileTable, {
    props: { path: dir, separator: '/', homeDir: dir },
    global: { stubs: { NDataTable: DataTable, DataTable, ...stubs } },
  })
  await flushPromises()
  await vi.waitFor(() => expect(wrapper!.find('table').exists()).toBe(true))
  await wrapper.get('[title="多选"]').trigger('click')
  return wrapper
}

it('双击压缩包打开包内浏览器', async () => {
  const view = await setup('darwin', ['archive.zip'], { ArchiveBrowser: true })
  await view.get('[title="多选"]').trigger('click')
  await view.get(`[data-entry-key="${join(dir, 'archive.zip')}"]`).trigger('dblclick')
  const browser = view.findComponent(ArchiveBrowser)
  expect(browser.props('show')).toBe(true)
  expect(browser.props('archivePath')).toBe(join(dir, 'archive.zip'))
})

it('右键压缩保留所选文件，并为输出追加 ZIP 扩展名', async () => {
  const Modal = defineComponent({
    props: ['show'],
    setup: (props, { slots }) => () => props.show ? h('section', [slots.default?.(), slots.footer?.()]) : null,
  })
  const view = await setup('darwin', ['a.txt', 'b.txt'], { NModal: Modal, Modal })
  for (const name of ['a.txt', 'b.txt']) await view.get(`[data-entry-key="${join(dir, name)}"]`).trigger('click')
  await view.get(`[data-entry-key="${join(dir, 'b.txt')}"]`).trigger('contextmenu')
  await nextTick()
  const dropdown = view.findComponent({ name: 'Dropdown' })
  const options = dropdown.props('options') as Array<{ key: string }>
  expect(options.at(-1)?.key).toBe('create-archive')
  dropdown.vm.$emit('select', 'create-archive')
  await nextTick()
  await view.get('input[placeholder="压缩包文件名（自动追加扩展名）"]').setValue('backup')
  vi.mocked(FileService.CreateArchive).mockImplementation(() => CancellablePromise.resolve(join(dir, 'backup.zip')))
  await view.findAll('button').find(button => button.text() === '开始压缩')!.trigger('click')
  await flushPromises()
  expect(FileService.CreateArchive).toHaveBeenCalledWith([join(dir, 'a.txt'), join(dir, 'b.txt')], join(dir, 'backup.zip'), 'zip')
})

async function pointer(target: { element: Element }, type: string, x: number) {
  target.element.dispatchEvent(new MouseEvent(type, { bubbles: true, clientX: x, clientY: 0 }))
  await nextTick()
}

function status() {
  return wrapper!.emitted('selectionStatus')!.at(-1)![0]
}

it('恢复排序，在切换目录和重新挂载后保留，并可持久化取消排序', async () => {
  useSettings().updateSetting('fileSort', { columnKey: 'name', order: 'descend' })
  const view = await setup('darwin')
  const table = () => view.findComponent(DataTable)
  const names = () => table().props('data').filter((row: FileEntry) => !row.path.startsWith('__zashiki_parent__:')).map((row: FileEntry) => row.name)
  expect(names()).toEqual(['b.txt', 'a.txt'])
  expect(table().props('columns').find((column: { key: string }) => column.key === 'name').sortOrder).toBe('descend')

  table().props('onUpdate:sorter')({ columnKey: 'name', order: 'ascend' })
  await nextTick()
  expect(names()).toEqual(['a.txt', 'b.txt'])
  expect(SettingsService.SaveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ fileSort: { columnKey: 'name', order: 'ascend' } }))

  const otherDir = await mkdtemp(join(dir, 'other-'))
  await Promise.all(['d.txt', 'c.txt'].map(name => writeFile(join(otherDir, name), '')))
  await view.setProps({ path: otherDir })
  await vi.waitFor(() => expect(names()).toEqual(['c.txt', 'd.txt']))
  view.unmount()
  wrapper = mount(FileTable, {
    props: { path: otherDir, separator: '/', homeDir: dir },
    global: { stubs: { NDataTable: DataTable, DataTable } },
  })
  await vi.waitFor(() => expect(wrapper!.findComponent(DataTable).props('data').some((row: FileEntry) => row.name === 'c.txt')).toBe(true))
  expect(wrapper.findComponent(DataTable).props('columns').find((column: { key: string }) => column.key === 'name').sortOrder).toBe('ascend')
  wrapper.findComponent(DataTable).props('onUpdate:sorter')({ columnKey: 'name', order: false })
  await nextTick()
  expect(SettingsService.SaveSettings).toHaveBeenLastCalledWith(expect.objectContaining({ fileSort: { columnKey: '', order: '' } }))
})

describe.each(['darwin', 'windows', 'linux'])('列表多选（%s）', (os) => {
  it('点击未选中的行可累计选择，再次点击可取消选择', async () => {
    const view = await setup(os)
    for (const name of ['a.txt', 'b.txt']) {
      const row = view.get(`[data-entry-key="${join(dir, name)}"]`)
      await pointer(row, 'pointerdown', 0)
      expect(status()).toEqual({ multiSelectMode: true, selectedCount: name === 'a.txt' ? 0 : 1 })
      await row.trigger('pointerup')
      await row.trigger('click')
    }
    expect(status()).toEqual({ multiSelectMode: true, selectedCount: 2 })
    const row = view.get(`[data-entry-key="${join(dir, 'a.txt')}"]`)
    await row.trigger('pointerdown')
    await row.trigger('pointerup')
    await row.trigger('click')
    expect(status()).toEqual({ multiSelectMode: true, selectedCount: 1 })
  })

  it('复选框按下时保持多选，点击后更新选择', async () => {
    const view = await setup(os)
    const checkbox = view.get(`[data-entry-key="${join(dir, 'a.txt')}"] input`)
    await checkbox.trigger('pointerdown')
    expect(status()).toEqual({ multiSelectMode: true, selectedCount: 0 })
    await checkbox.trigger('pointerup')
    await checkbox.trigger('click')
    expect(status()).toEqual({ multiSelectMode: true, selectedCount: 1 })
  })
})

it.each(['darwin', 'windows'])('原生拖拽路径快照保持选择状态（%s）', async (os) => {
  const view = await setup(os)
  const first = view.get(`[data-entry-key="${join(dir, 'a.txt')}"]`)
  const second = view.get(`[data-entry-key="${join(dir, 'b.txt')}"]`)
  await first.trigger('click')
  await pointer(second, 'pointerdown', 0)
  await pointer(second, 'pointermove', 10)
  expect(FileTransferService.StartDrag).toHaveBeenLastCalledWith([join(dir, 'b.txt')], 10, 0)
  expect(status()).toEqual({ multiSelectMode: true, selectedCount: 1 })
  await second.trigger('pointerup')
  await second.trigger('click')
  await pointer(first, 'pointerdown', 0)
  await pointer(first, 'pointermove', 10)
  expect(FileTransferService.StartDrag).toHaveBeenLastCalledWith([join(dir, 'a.txt'), join(dir, 'b.txt')], 10, 0)
  expect(status()).toEqual({ multiSelectMode: true, selectedCount: 2 })
})
