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

export type ShortcutBinding = ShortcutKey | ShortcutKey[]

export interface ShortcutAction {
  id: string
  label: string
  keys: ShortcutBinding[]
  run: (event: KeyboardEvent) => void | Promise<void>
  allowInEditable?: boolean
  disabled?: () => boolean
  preventDefault?: boolean
}

export function formatShortcutBinding(binding: ShortcutBinding): string {
  return normalizeBinding(binding).map(formatShortcutKey).join(' ')
}

function formatShortcutKey(shortcut: ShortcutKey): string {
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
  let pendingSequence: ShortcutKey[] = []
  let pendingTimer: number | null = null

  function onKeydown(event: KeyboardEvent) {
    if (options.active && !options.active()) return
    handleKeydown(event, false)
  }

  function activeBindings(editable: boolean) {
    return actions().flatMap((action) => {
      if (action.disabled?.()) return []
      return action.keys.flatMap((binding) => {
        const sequence = normalizeBinding(binding)
        const allow = action.allowInEditable || sequence.some(shortcut => shortcut.allowInEditable)
        if (editable && !allow) return []
        return [{ action, sequence }]
      })
    })
  }

  function handleKeydown(event: KeyboardEvent, retried: boolean) {
    const editable = isEditableTarget(event.target)
    const snapshot = eventToShortcutKey(event)
    const sequence = [...pendingSequence, snapshot]
    const bindings = activeBindings(editable)
    const fullMatch = bindings.find(item => item.sequence.length === sequence.length && matchesSequence(sequence, item.sequence))
    const hasPrefix = bindings.some(item => item.sequence.length > sequence.length && matchesSequence(sequence, item.sequence.slice(0, sequence.length)))

    if (hasPrefix && !fullMatch) {
      event.preventDefault()
      setPendingSequence(sequence)
      return
    }

    clearPendingSequence()

    if (!fullMatch) {
      if (!retried && sequence.length > 1) {
        handleKeydown(event, true)
      }
      return
    }

    if (fullMatch.action.preventDefault !== false) {
      event.preventDefault()
    }
    void fullMatch.action.run(event)
  }

  onMounted(() => {
    window.addEventListener('keydown', onKeydown)
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', onKeydown)
    clearPendingSequence()
  })

  function setPendingSequence(sequence: ShortcutKey[]) {
    clearPendingSequence()
    pendingSequence = sequence
    pendingTimer = window.setTimeout(() => {
      pendingSequence = []
      pendingTimer = null
    }, 800)
  }

  function clearPendingSequence() {
    pendingSequence = []
    if (pendingTimer !== null) {
      window.clearTimeout(pendingTimer)
      pendingTimer = null
    }
  }
}

function matchesSequence(actual: ShortcutKey[], expected: ShortcutKey[]): boolean {
  if (actual.length > expected.length) return false
  return actual.every((item, index) => matchesShortcutKey(item, expected[index]))
}

function matchesShortcutKey(actual: ShortcutKey, expected: ShortcutKey): boolean {
  if (normalizeKey(actual.key) !== normalizeKey(expected.key)) return false

  if (expected.ctrlOrMeta) {
    if (!actual.ctrl && !actual.meta) return false
  } else {
    if (!!actual.ctrl !== !!expected.ctrl) return false
    if (!!actual.meta !== !!expected.meta) return false
  }

  return !!actual.shift === !!expected.shift && !!actual.alt === !!expected.alt
}

function eventToShortcutKey(event: KeyboardEvent): ShortcutKey {
  return {
    key: normalizeKey(event.key),
    shift: event.shiftKey,
    ctrl: event.ctrlKey,
    meta: event.metaKey,
    alt: event.altKey,
  }
}

function normalizeBinding(binding: ShortcutBinding): ShortcutKey[] {
  return Array.isArray(binding) ? binding : [binding]
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
      return key
  }
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName.toLowerCase()
  return tag === 'input' || tag === 'textarea' || target.isContentEditable
}
