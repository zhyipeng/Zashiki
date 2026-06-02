export function isWindowsSeparator(separator: string): boolean {
  return separator === '\\'
}

export function pathRoot(path: string, separator: string): string {
  if (!path) return ''
  if (!isWindowsSeparator(separator)) return '/'

  const normalized = normalizeWindowsPath(path)
  const driveMatch = normalized.match(/^[A-Za-z]:/)
  if (driveMatch) return `${driveMatch[0]}\\`

  const uncMatch = normalized.match(/^\\\\[^\\]+\\[^\\]+/)
  return uncMatch ? `${uncMatch[0]}\\` : ''
}

export function baseName(path: string, separator: string): string {
  const trimmed = trimTrailingSeparators(path, separator)
  const parts = isWindowsSeparator(separator)
    ? trimmed.split(/[\\/]/)
    : trimmed.split('/')
  return parts.filter(Boolean).pop() || trimmed || path
}

export function joinPath(base: string, child: string, separator: string): string {
  if (!base) return child
  const trimmed = trimTrailingSeparators(base, separator)
  const root = pathRoot(base, separator)
  const prefix = trimmed === root ? root : trimmed + separator
  return prefix + child
}

export function parentPath(path: string, separator: string): string | null {
  if (!path) return null

  const normalized = isWindowsSeparator(separator)
    ? normalizeWindowsPath(path)
    : path
  const root = pathRoot(normalized, separator)
  const trimmed = trimTrailingSeparators(normalized, separator)

  if (trimmed === trimTrailingSeparators(root, separator)) return null

  const index = trimmed.lastIndexOf(separator)
  if (index < 0) return root || null
  if (root && index < root.length) return root

  const parent = trimmed.slice(0, index)
  if (!parent || parent === trimTrailingSeparators(root, separator)) return root
  return parent
}

export function ancestorPaths(path: string, separator: string): string[] {
  const ancestors: string[] = []
  let current = parentPath(path, separator)
  while (current) {
    ancestors.unshift(current)
    current = parentPath(current, separator)
  }
  return ancestors
}

function normalizeWindowsPath(path: string): string {
  return path.replace(/\//g, '\\')
}

function trimTrailingSeparators(path: string, separator: string): string {
  const root = pathRootWithoutTrim(path, separator)
  let end = path.length
  while (end > root.length && (path[end - 1] === '/' || path[end - 1] === '\\')) {
    end--
  }
  return path.slice(0, end)
}

function pathRootWithoutTrim(path: string, separator: string): string {
  if (!isWindowsSeparator(separator)) return '/'

  const normalized = normalizeWindowsPath(path)
  const driveMatch = normalized.match(/^[A-Za-z]:\\?/)
  if (driveMatch) {
    return driveMatch[0].endsWith('\\') ? driveMatch[0] : `${driveMatch[0]}\\`
  }

  const uncMatch = normalized.match(/^\\\\[^\\]+\\[^\\]+\\?/)
  if (!uncMatch) return ''
  return uncMatch[0].endsWith('\\') ? uncMatch[0] : `${uncMatch[0]}\\`
}
