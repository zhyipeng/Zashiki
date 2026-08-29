import type { Component } from 'vue'
import {
  DeleteOutlined,
  DescriptionOutlined,
  DesktopWindowsOutlined,
  DownloadOutlined,
  FolderOutlined,
  FolderSpecialOutlined,
  HomeOutlined,
} from '@vicons/material'

export type QuickAccessKind = 'home' | 'desktop' | 'documents' | 'downloads'

export interface QuickAccessIcon {
  icon: Component
  color: string
}

/** 内置快速访问目录的专属图标，色值与应用整体调色板一致。 */
export const quickAccessIcons: Record<QuickAccessKind, QuickAccessIcon> = {
  home: { icon: HomeOutlined, color: '#4B7BEC' },
  desktop: { icon: DesktopWindowsOutlined, color: '#7048E8' },
  documents: { icon: DescriptionOutlined, color: '#2F9E44' },
  downloads: { icon: DownloadOutlined, color: '#E8590C' },
}

const trashIcon: QuickAccessIcon = { icon: DeleteOutlined, color: '#D99A22' }
const pinnedIcon: QuickAccessIcon = { icon: FolderSpecialOutlined, color: '#4B7BEC' }
const folderIcon: QuickAccessIcon = { icon: FolderOutlined, color: '#D99A22' }

export function resolveQuickAccessIcon(item: {
  kind?: QuickAccessKind
  isTrash?: boolean
  isPinned?: boolean
}): QuickAccessIcon {
  if (item.isTrash) return trashIcon
  if (item.kind) return quickAccessIcons[item.kind]
  if (item.isPinned) return pinnedIcon
  return folderIcon
}
