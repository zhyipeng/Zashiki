import { onMounted, onUnmounted } from 'vue'

export interface ShortcutKey {
  key: string
  shift?: boolean
  ctrl?: boolean
  meta?: boolean
  alt?: boolean
  ctrlOrMeta?: boolean
  allowInEditable?: boolean
}

export interface ShortcutAction {
  id: string
  label: string
  keys: ShortcutKey[]
  run: (event: KeyboardEvent) => void | Promise<void>
  allowInEditable?: boolean
  disabled?: () => boolean
  preventDefault?: boolean
}

export function formatShortcutKey(shortcut: ShortcutKey): string {
  const parts: string[] = []
  if (shortcut.ctrlOrMeta) parts.push('Ctrl/Cmd')
  if (shortcut.ctrl) parts.push('Ctrl')
  if (shortcut.meta) parts.push('Cmd')
  if (shortcut.alt) parts.push('Alt')
  if (shortcut.shift) parts.push('Shift')
  parts.push(displayKey(shortcut.key))
  return parts.join(' + ')
}

export function useKeyboardShortcuts(
  actions: () => ShortcutAction[],
  options: { active?: () => boolean } = {},
) {
  function onKeydown(event: KeyboardEvent) {
    if (options.active && !options.active()) return
    const editable = isEditableTarget(event.target)

    const action = actions().find((item) => {
      if (item.disabled?.()) return false
      return item.keys.some((shortcut) => {
        if (editable && !item.allowInEditable && !shortcut.allowInEditable) return false
        return matchesShortcut(event, shortcut)
      })
    })
    if (!action) return

    if (action.preventDefault !== false) {
      event.preventDefault()
    }
    void action.run(event)
  }

  onMounted(() => {
    window.addEventListener('keydown', onKeydown)
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', onKeydown)
  })
}

function matchesShortcut(event: KeyboardEvent, shortcut: ShortcutKey): boolean {
  if (normalizeKey(event.key) !== normalizeKey(shortcut.key)) return false

  if (shortcut.ctrlOrMeta) {
    if (!event.ctrlKey && !event.metaKey) return false
  } else {
    if (event.ctrlKey !== !!shortcut.ctrl) return false
    if (event.metaKey !== !!shortcut.meta) return false
  }

  return event.shiftKey === !!shortcut.shift && event.altKey === !!shortcut.alt
}

function normalizeKey(key: string): string {
  if (key === ' ') return 'space'
  if (key === 'Esc') return 'escape'
  return key.toLowerCase()
}

function displayKey(key: string): string {
  switch (normalizeKey(key)) {
    case 'space':
      return 'Space'
    case 'escape':
      return 'Esc'
    case 'enter':
      return 'Enter'
    default:
      return key.length === 1 ? key.toUpperCase() : key
  }
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return tag === 'input' || tag === 'textarea' || target.isContentEditable
}
