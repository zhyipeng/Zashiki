import { describe, expect, it } from 'vitest'
import {
  DeleteOutlined,
  DescriptionOutlined,
  DesktopWindowsOutlined,
  DownloadOutlined,
  FolderOutlined,
  FolderSpecialOutlined,
  HomeOutlined,
} from '@vicons/material'
import { quickAccessIcons, resolveQuickAccessIcon } from './quickAccessIcons'

describe('quickAccessIcons', () => {
  it('gives every built-in directory a dedicated icon and color', () => {
    expect(quickAccessIcons.home).toEqual({ icon: HomeOutlined, color: '#4B7BEC' })
    expect(quickAccessIcons.desktop).toEqual({ icon: DesktopWindowsOutlined, color: '#7048E8' })
    expect(quickAccessIcons.documents).toEqual({ icon: DescriptionOutlined, color: '#2F9E44' })
    expect(quickAccessIcons.downloads).toEqual({ icon: DownloadOutlined, color: '#E8590C' })
  })

  it('uses distinct colors so the four entries can be told apart at a glance', () => {
    const colors = Object.values(quickAccessIcons).map(icon => icon.color)
    expect(new Set(colors).size).toBe(colors.length)
  })

  it('resolves built-in kinds to their dedicated icons', () => {
    expect(resolveQuickAccessIcon({ kind: 'home' }).icon).toBe(HomeOutlined)
    expect(resolveQuickAccessIcon({ kind: 'desktop' }).icon).toBe(DesktopWindowsOutlined)
    expect(resolveQuickAccessIcon({ kind: 'documents' }).icon).toBe(DescriptionOutlined)
    expect(resolveQuickAccessIcon({ kind: 'downloads' }).icon).toBe(DownloadOutlined)
  })

  it('keeps the trash icon with its original color', () => {
    const icon = resolveQuickAccessIcon({ isTrash: true })
    expect(icon.icon).toBe(DeleteOutlined)
    expect(icon.color).toBe('#D99A22')
  })

  it('keeps pinned folders on the special folder icon', () => {
    const icon = resolveQuickAccessIcon({ isPinned: true })
    expect(icon.icon).toBe(FolderSpecialOutlined)
    expect(icon.color).toBe('#4B7BEC')
  })

  it('falls back to the plain folder icon for unlabeled items', () => {
    const icon = resolveQuickAccessIcon({})
    expect(icon.icon).toBe(FolderOutlined)
    expect(icon.color).toBe('#D99A22')
  })
})
