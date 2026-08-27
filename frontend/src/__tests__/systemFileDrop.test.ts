import { describe, expect, it } from 'vitest'
import {
  hasSidebarDropTarget,
  isQuickAccessDropTarget,
  sidebarTargetDir,
  panelIdFromPoint,
  resolveTargetPanel,
  targetDirFromDropDetails,
} from '../composables/useSystemFileDrop'

describe('sidebarTargetDir', () => {
  it('从 data-drop-dir 属性解析侧边栏目录', () => {
    const dir = sidebarTargetDir({ x: 10, y: 10, attributes: { 'data-drop-dir': '/home/user/docs' } })
    expect(dir).toBe('/home/user/docs')
  })

  it('__quick_access__ 特殊标记返回空（由坐标定位处理）', () => {
    const dir = sidebarTargetDir({ x: 10, y: 10, attributes: { 'data-drop-dir': '__quick_access__' } })
    expect(dir).toBe('')
  })

  it('无属性或空属性返回空', () => {
    expect(sidebarTargetDir(undefined)).toBe('')
    expect(sidebarTargetDir(null)).toBe('')
    expect(sidebarTargetDir({ x: 0, y: 0 })).toBe('')
  })
})

describe('hasSidebarDropTarget', () => {
  it('带 data-drop-dir 属性视为侧边栏落点', () => {
    expect(hasSidebarDropTarget({ x: 0, y: 0, attributes: { 'data-drop-dir': '/x' } })).toBe(true)
  })

  it('无属性或空属性不是侧边栏落点', () => {
    expect(hasSidebarDropTarget({ x: 0, y: 0 })).toBe(false)
    expect(hasSidebarDropTarget(null)).toBe(false)
  })
})

describe('isQuickAccessDropTarget', () => {
  it('识别快速访问区域本身', () => {
    expect(isQuickAccessDropTarget({ x: 0, y: 0, attributes: { 'data-drop-dir': '__quick_access__' } })).toBe(true)
  })

  it('识别快速访问条目', () => {
    expect(isQuickAccessDropTarget({ x: 0, y: 0, attributes: { 'data-quick-access-target': 'true' } })).toBe(true)
  })

  it('普通目录落点不是快速访问目标', () => {
    expect(isQuickAccessDropTarget({ x: 0, y: 0, attributes: { 'data-drop-dir': '/tmp/docs' } })).toBe(false)
    expect(isQuickAccessDropTarget(undefined)).toBe(false)
  })
})

describe('targetDirFromDropDetails', () => {
  it('目录行属性优先于面板当前目录', () => {
    expect(targetDirFromDropDetails(
      { x: 0, y: 0, attributes: { 'data-folder-path': '/tmp/target' } },
      '/tmp/panel',
    )).toBe('/tmp/target')
  })

  it('未命中目录行时回退面板当前目录', () => {
    expect(targetDirFromDropDetails({ x: 0, y: 0, attributes: {} }, '/tmp/panel')).toBe('/tmp/panel')
    expect(targetDirFromDropDetails(undefined, '/tmp/panel')).toBe('/tmp/panel')
  })
})

describe('resolveTargetPanel', () => {
  it('坐标命中面板时返回该面板 id', () => {
    const details = { x: 100, y: 100 }
    const pointToPanel = (x: number, y: number) => (x === 100 && y === 100 ? 3 : 0)
    expect(resolveTargetPanel(details, pointToPanel, 1)).toBe(3)
  })

  it('坐标未命中时回退当前激活面板', () => {
    const details = { x: 100, y: 100 }
    const pointToPanel = () => 0
    expect(resolveTargetPanel(details, pointToPanel, 5)).toBe(5)
  })

  it('无坐标时回退当前激活面板', () => {
    expect(resolveTargetPanel(null, () => 7, 2)).toBe(2)
    expect(resolveTargetPanel(undefined, () => 7, 2)).toBe(2)
  })

  it('坐标命中与激活面板都缺失时返回 0', () => {
    expect(resolveTargetPanel({ x: 1, y: 1 }, () => 0, 0)).toBe(0)
  })
})

describe('panelIdFromPoint', () => {
  it('jsdom 无 elementFromPoint 时返回 0（回退激活面板）', () => {
    expect(panelIdFromPoint(100, 100)).toBe(0)
  })
})
