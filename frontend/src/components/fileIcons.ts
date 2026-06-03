import type { Component } from 'vue'
import { FolderOutlined, ImageOutlined, InsertDriveFileOutlined, LinkOutlined } from '@vicons/material'
import { Document24Regular, DocumentText24Regular } from '@vicons/fluent'
import type { FileEntry } from '../../bindings/zashiki'

export type FileIconSource =
  | 'directory'
  | 'executable'
  | 'symlink'
  | 'image'
  | 'extension'
  | 'text'
  | 'binary'

export interface FileIconDefinition {
  icon: Component
  color?: string
  label?: string
}

export interface ResolvedFileIcon extends FileIconDefinition {
  source: FileIconSource
}

export const specialFileIcons = {
  directory: {
    icon: FolderOutlined,
    color: '#D99A22',
    label: '目录',
  },
  executable: {
    icon: InsertDriveFileOutlined,
    color: '#4B7BEC',
    label: '可执行文件',
  },
  symlink: {
    icon: LinkOutlined,
    color: '#6B7280',
    label: '链接',
  },
  image: {
    icon: ImageOutlined,
    color: '#2F9E44',
    label: '图片',
  },
} satisfies Record<'directory' | 'executable' | 'symlink' | 'image', FileIconDefinition>

export const fallbackFileIcons = {
  text: {
    icon: DocumentText24Regular,
    color: '#4B5563',
    label: '文本文件',
  },
  binary: {
    icon: Document24Regular,
    color: '#6B7280',
    label: '二进制文件',
  },
} satisfies Record<'text' | 'binary', FileIconDefinition>

export const extensionFileIcons: Record<string, FileIconDefinition> = {
  // Fill concrete extension mappings here, for example:
  // '.md': { icon: DocumentText24Regular, color: '#4B5563', label: 'Markdown' },
}

export const imageExtensions = new Set([
  '.avif',
  '.bmp',
  '.gif',
  '.heic',
  '.jpeg',
  '.jpg',
  '.png',
  '.svg',
  '.tif',
  '.tiff',
  '.webp',
])

export const textExtensions = new Set([
  '.cfg',
  '.conf',
  '.css',
  '.csv',
  '.go',
  '.html',
  '.ini',
  '.js',
  '.json',
  '.log',
  '.md',
  '.py',
  '.rs',
  '.sh',
  '.toml',
  '.ts',
  '.txt',
  '.vue',
  '.xml',
  '.yaml',
  '.yml',
])

export function resolveFileIcon(entry: FileEntry): ResolvedFileIcon {
  if (entry.isDir) {
    return { ...specialFileIcons.directory, source: 'directory' }
  }
  if (entry.isExecutable) {
    return { ...specialFileIcons.executable, source: 'executable' }
  }
  if (entry.isSymlink) {
    return { ...specialFileIcons.symlink, source: 'symlink' }
  }

  const extension = fileExtension(entry.name)
  if (imageExtensions.has(extension)) {
    return { ...specialFileIcons.image, source: 'image' }
  }

  const extensionIcon = extensionFileIcons[extension]
  if (extensionIcon) {
    return { ...extensionIcon, source: 'extension' }
  }

  if (textExtensions.has(extension)) {
    return { ...fallbackFileIcons.text, source: 'text' }
  }
  return { ...fallbackFileIcons.binary, source: 'binary' }
}

export function fileExtension(name: string): string {
  const index = name.lastIndexOf('.')
  if (index <= 0 || index === name.length - 1) return ''
  return name.slice(index).toLowerCase()
}
