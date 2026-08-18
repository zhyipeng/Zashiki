export const fallbackPageEntryOffset = 10

export type NavigationMouseAction = 'back' | 'forward'

export function navigationMouseAction(button: number): NavigationMouseAction | null {
  if (button === 3) return 'back'
  if (button === 4) return 'forward'
  return null
}

export function pageEntryOffset(
  viewportHeight: number,
  rowHeight: number,
  fallback = fallbackPageEntryOffset,
): number {
  if (!Number.isFinite(viewportHeight) || !Number.isFinite(rowHeight)) return fallback
  if (viewportHeight <= 0 || rowHeight <= 0) return fallback

  return Math.max(1, Math.floor(viewportHeight / rowHeight) - 1)
}

