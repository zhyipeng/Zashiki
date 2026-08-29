import type { Component } from 'vue'
import { FolderOutlined, ImageOutlined, InsertDriveFileOutlined, LinkOutlined, MovieOutlined, MusicNoteOutlined } from '@vicons/material'
import { Document24Regular, DocumentText24Regular } from '@vicons/fluent'
import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'

export type FileIconSource =
  | 'directory'
  | 'executable'
  | 'symlink'
  | 'image'
  | 'audio'
  | 'video'
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
  audio: {
    icon: MusicNoteOutlined,
    color: '#7048E8',
    label: '音频',
  },
  video: {
    icon: MovieOutlined,
    color: '#E03131',
    label: '视频',
  },
} satisfies Record<'directory' | 'executable' | 'symlink' | 'image' | 'audio' | 'video', FileIconDefinition>

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

export const audioExtensions = new Set([
  '.aac',
  '.ac3',
  '.aif',
  '.aiff',
  '.amr',
  '.ape',
  '.caf',
  '.flac',
  '.m4a',
  '.mka',
  '.mid',
  '.midi',
  '.mp3',
  '.oga',
  '.ogg',
  '.opus',
  '.ra',
  '.wav',
  '.wma',
])

export const videoExtensions = new Set([
  '.3g2',
  '.3gp',
  '.asf',
  '.avi',
  '.divx',
  '.f4v',
  '.flv',
  '.m2ts',
  '.m4v',
  '.mkv',
  '.mov',
  '.mp4',
  '.mpe',
  '.mpeg',
  '.mpg',
  '.mts',
  '.ogv',
  '.rm',
  '.rmvb',
  '.vob',
  '.webm',
  '.wmv',
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
  if (audioExtensions.has(extension)) {
    return { ...specialFileIcons.audio, source: 'audio' }
  }
  if (videoExtensions.has(extension)) {
    return { ...specialFileIcons.video, source: 'video' }
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

export function fileTypeLabel(entry: FileEntry): string {
  if (entry.isDir) return '目录'
  if (entry.isExecutable) return '可执行'
  if (entry.isSymlink) return '链接'

  const extension = fileExtension(entry.name)
  if (imageExtensions.has(extension)) return '图片'
  if (audioExtensions.has(extension)) return '音频'
  if (videoExtensions.has(extension)) return '视频'
  if (textExtensions.has(extension)) return '文本'
  if (extension) return extension.slice(1).toUpperCase()
  return '文件'
}

export function isTextFile(entry: FileEntry): boolean {
  if (entry.isDir || entry.isExecutable || entry.isSymlink) return false
  return textExtensions.has(fileExtension(entry.name))
}

/** .exe 是唯一保证内嵌图标资源的可执行类型，可尝试展示提取出的真实图标。 */
export function isExeFile(entry: FileEntry): boolean {
  if (entry.isDir) return false
  return fileExtension(entry.name) === '.exe'
}

export function fileExtension(name: string): string {
  const index = name.lastIndexOf('.')
  if (index <= 0 || index === name.length - 1) return ''
  return name.slice(index).toLowerCase()
}
