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
    kind: 'html',
    label: 'HTML',
  },
  {
    kind: 'office',
    label: 'Office',
  },
  {
    kind: 'pdf',
    label: 'PDF',
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

export function isMarkdownPreview(preview: FilePreview | null | undefined): boolean {
  if (!preview || preview.kind !== 'text') return false
  const mimeType = preview.mimeType.toLowerCase()
  if (mimeType === 'text/markdown') return true
  return preview.name.toLowerCase().endsWith('.md') || preview.path.toLowerCase().endsWith('.md')
}

export function isHtmlPreview(preview: FilePreview | null | undefined): boolean {
  if (!preview || preview.kind !== 'html') return false
  return true
}

/**
 * Map file extension or mimeType to a Shiki language id.
 * Returns undefined if the language is not supported for code highlighting.
 */
export function resolveCodeLanguage(preview: FilePreview): string | undefined {
  const ext = extractExt(preview.name || preview.path)
  // Direct extension → language mappings
  const extMap: Record<string, string> = {
    js: 'javascript',
    mjs: 'javascript',
    cjs: 'javascript',
    ts: 'typescript',
    tsx: 'tsx',
    jsx: 'jsx',
    vue: 'vue',
    css: 'css',
    scss: 'scss',
    less: 'less',
    html: 'html',
    htm: 'html',
    xml: 'xml',
    json: 'json',
    yaml: 'yaml',
    yml: 'yaml',
    toml: 'toml',
    md: 'markdown',
    mdx: 'mdx',
    py: 'python',
    rb: 'ruby',
    go: 'go',
    rs: 'rust',
    java: 'java',
    kt: 'kotlin',
    kts: 'kotlin',
    scala: 'scala',
    c: 'c',
    cpp: 'cpp',
    cc: 'cpp',
    cxx: 'cpp',
    h: 'c',
    hpp: 'cpp',
    hh: 'cpp',
    hxx: 'cpp',
    cs: 'csharp',
    swift: 'swift',
    sh: 'shellscript',
    bash: 'shellscript',
    zsh: 'shellscript',
    fish: 'shellscript',
    sql: 'sql',
    php: 'php',
    r: 'r',
    lua: 'lua',
    perl: 'perl',
    pl: 'perl',
    dart: 'dart',
    el: 'elisp',
    vim: 'vim',
    dockerfile: 'dockerfile',
    makefile: 'makefile',
    gradle: 'gradle',
    gitignore: 'gitignore',
    gitattributes: 'gitignore',
    editorconfig: 'editorconfig',
    proto: 'proto',
    diff: 'diff',
    patch: 'diff',
    ini: 'ini',
    cfg: 'ini',
    conf: 'ini',
    log: 'log',
    txt: 'plaintext',
  }
  if (ext && extMap[ext]) return extMap[ext]

  // mimeType fallback
  const mime = preview.mimeType?.toLowerCase() || ''
  const mimeMap: Record<string, string> = {
    'text/javascript': 'javascript',
    'text/typescript': 'typescript',
    'text/css': 'css',
    'text/html': 'html',
    'text/markdown': 'markdown',
    'text/x-python': 'python',
    'text/x-shellscript': 'shellscript',
    'text/x-ruby': 'ruby',
    'text/x-java': 'java',
    'text/x-c': 'c',
    'text/x-c++': 'cpp',
    'text/x-csharp': 'csharp',
    'text/x-swift': 'swift',
    'text/x-sql': 'sql',
    'text/x-php': 'php',
    'text/x-r': 'r',
    'text/x-lua': 'lua',
    'text/x-perl': 'perl',
    'text/x-dart': 'dart',
    'text/x-vue': 'vue',
    'text/xml': 'xml',
    'application/json': 'json',
    'application/xml': 'xml',
    'application/yaml': 'yaml',
    'application/toml': 'toml',
  }
  if (mimeMap[mime]) return mimeMap[mime]

  // Special filenames
  const name = preview.name?.toLowerCase() || ''
  if (name === 'dockerfile') return 'dockerfile'
  if (name === 'makefile' || name === 'gnumakefile') return 'makefile'
  if (name === 'cmakelists.txt' || name.endsWith('.cmake')) return 'cmake'
  if (name === '.gitignore' || name === '.gitattributes') return 'gitignore'
  if (name === '.editorconfig') return 'editorconfig'
  if (name === '.env' || name.startsWith('.env.')) return 'dotenv'

  return undefined
}

/**
 * Determine whether a text preview should use Shiki code highlighting.
 * Markdown and plain text without a recognized language are excluded.
 */
export function isCodePreview(preview: FilePreview | null | undefined): boolean {
  if (!preview || preview.kind !== 'text') return false
  if (isMarkdownPreview(preview)) return false
  const lang = resolveCodeLanguage(preview)
  return lang !== undefined && lang !== 'plaintext' && lang !== 'log'
}

function extractExt(filename: string): string {
  const dot = filename.lastIndexOf('.')
  if (dot <= 0) return ''
  return filename.slice(dot + 1).toLowerCase()
}
