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
