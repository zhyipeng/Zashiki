import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'

export interface PreviewRenderer {
  kind: string
  label: string
}

const unsupportedPreviewRenderer: PreviewRenderer = {
  kind: 'unsupported',
  label: '不可预览',
}

export const previewRenderers: PreviewRenderer[] = [
  {
    kind: 'text',
    label: '文本',
  },
  {
    kind: 'image',
    label: '图片',
  },
  {
    kind: 'office',
    label: 'Office',
  },
  unsupportedPreviewRenderer,
]

export function resolvePreviewRenderer(preview: FilePreview | null | undefined): PreviewRenderer {
  if (!preview) return unsupportedPreviewRenderer
  return previewRenderers.find(renderer => renderer.kind === preview.kind) || unsupportedPreviewRenderer
}

export function formatPreviewSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  if (index === 0) return `${bytes} B`
  return `${(bytes / 1024 ** index).toFixed(1)} ${units[index]}`
}

export function previewTextContent(preview: FilePreview | null | undefined): string {
  if (!preview) return ''
  if (!isJsonPreview(preview)) return preview.content
  try {
    const parsed = JSON.parse(preview.content)
    const formatted = JSON.stringify(parsed, null, 2)
    return formatted === undefined ? preview.content : formatted
  } catch {
    return preview.content
  }
}

export function isFormattedJsonPreview(preview: FilePreview | null | undefined): boolean {
  if (!preview || !isJsonPreview(preview)) return false
  return previewTextContent(preview) !== preview.content
}

function isJsonPreview(preview: FilePreview): boolean {
  if (preview.kind !== 'text') return false
  const mimeType = preview.mimeType.toLowerCase()
  if (mimeType === 'application/json' || mimeType.endsWith('+json')) return true
  return preview.name.toLowerCase().endsWith('.json') || preview.path.toLowerCase().endsWith('.json')
}
