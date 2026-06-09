import { onUnmounted } from 'vue'

type DirectoryChangedCallback = (dir: string) => void

type DirectoryEventToken = symbol

const callbacks = new Map<DirectoryEventToken, { currentPath: () => string, onChanged: DirectoryChangedCallback }>()
const listeners = new Map<DirectoryEventToken, (dirs: string[]) => void>()

export function useDirectoryEvents(currentPath: () => string, onChanged: DirectoryChangedCallback) {
  const token = Symbol('directory-events')
  callbacks.set(token, { currentPath, onChanged })
  onUnmounted(() => {
    callbacks.delete(token)
  })
  return token
}

export function useDirectoryChangeListener(onChanged: (dirs: string[]) => void) {
  const token = Symbol('directory-change-listener')
  listeners.set(token, onChanged)
  onUnmounted(() => {
    listeners.delete(token)
  })
  return token
}

export function notifyDirectoriesChanged(dirs: string[], options: { exclude?: DirectoryEventToken } = {}) {
  const changed = new Set(dirs.filter(Boolean))
  if (changed.size === 0) return
  const changedDirs = Array.from(changed)
  for (const [token, listener] of listeners) {
    if (options.exclude && token === options.exclude) continue
    listener(changedDirs)
  }
  for (const [token, item] of callbacks) {
    if (options.exclude && token === options.exclude) continue
    const path = item.currentPath()
    if (changed.has(path)) {
      item.onChanged(path)
    }
  }
}
