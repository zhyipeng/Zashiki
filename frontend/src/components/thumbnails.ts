import type { FileEntry } from '../../bindings/zashiki/internal/filemanager'
import { resolveFileIcon } from './fileIcons'

export const THUMBNAIL_SIZE = 256

/**
 * 是否为看图模式下需要加载缩略图的条目。
 * 与列表视图的图标判断保持同一来源：目录/可执行/链接不视为图片。
 */
export function isImageEntry(entry: FileEntry): boolean {
  return resolveFileIcon(entry).source === 'image'
}
