import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import type { CancellablePromise } from '@wailsio/runtime'
import { OperationProgress } from '../../bindings/zashiki/internal/filemanager/models'

export type OperationKind = 'copy' | 'move' | 'delete' | 'trash' | 'sync'
export type OperationPhase = 'scan' | 'run' | 'done' | 'error' | 'cancelled'

export interface OperationProgressState {
  id: string
  kind: OperationKind
  phase: OperationPhase
  totalItems: number
  doneItems: number
  totalBytes: number
  doneBytes: number
  currentName: string
  error: string
}

/** Terminal phases after which the row lingers briefly, then disappears. */
export function isTerminalPhase(phase: OperationPhase): boolean {
  return phase === 'done' || phase === 'error' || phase === 'cancelled'
}

/** Whether a caught backend error represents a user-requested cancellation. */
export function isOperationCancelledError(err: unknown): boolean {
  if (!err) return false
  const text = err instanceof Error ? `${err.name}: ${err.message}` : String(err)
  return text.includes('context canceled') || text.includes('context cancelled') ||
    text.includes('call cancelled') || text.includes('CallCancelled')
}

/** Merge one backend event into the state list (upsert by operation id). */
export function mergeProgressEvent(
  states: OperationProgressState[],
  event: OperationProgress,
): OperationProgressState[] {
  const state: OperationProgressState = {
    id: event.operationId,
    kind: event.kind as OperationKind,
    phase: event.phase as OperationPhase,
    totalItems: event.totalItems,
    doneItems: event.doneItems,
    totalBytes: event.totalBytes,
    doneBytes: event.doneBytes,
    currentName: event.currentName,
    error: event.error ?? '',
  }
  const index = states.findIndex(item => item.id === state.id)
  if (index === -1) return [...states, state]
  const next = states.slice()
  next[index] = state
  return next
}

/** Percent in [0, 100]; byte-level when available, otherwise item-level; null when unknowable. */
export function progressPercent(state: OperationProgressState): number | null {
  if (state.phase === 'done') return 100
  if (state.totalBytes > 0) {
    return Math.min(100, Math.floor((state.doneBytes / state.totalBytes) * 100))
  }
  if (state.totalItems > 0) {
    return Math.min(100, Math.floor((state.doneItems / state.totalItems) * 100))
  }
  return null
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = -1
  do {
    value /= 1024
    unit++
  } while (value >= 1024 && unit < units.length - 1)
  return `${value >= 100 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`
}

/** Human progress text like "2.1/4.7 GB" or "3/128 项". */
export function progressByteText(state: OperationProgressState): string {
  if (state.totalBytes <= 0) return ''
  return `${formatBytes(state.doneBytes)}/${formatBytes(state.totalBytes)}`
}

const operationKindLabels: Record<OperationKind, string> = {
  copy: '复制',
  move: '移动',
  delete: '删除',
  trash: '回收',
  sync: '同步',
}

export function operationLabel(kind: OperationKind): string {
  return operationKindLabels[kind] ?? kind
}

const terminalPhaseRetentionMs = 5000

const progressStates = ref<OperationProgressState[]>([])
const cancellers = new Map<string, () => void>()
const removalTimers = new Map<string, ReturnType<typeof setTimeout>>()
/** Sequential ids linking a caller's promise to the backend operation event. */
const pendingCancellables = new Map<string, CancellablePromise<unknown>>()
let nextPendingToken = 1

/**
 * Track a just-started operation so its promise can be cancelled from the
 * progress UI, and await its result. The backend event does not carry a caller
 * token, so pending promises bind to operation ids in FIFO order.
 */
export function trackOperationPromise<T>(promise: CancellablePromise<T>): CancellablePromise<T> {
  const token = `pending-${nextPendingToken++}`
  pendingCancellables.set(token, promise as CancellablePromise<unknown>)
  return promise
}

/** Bind the oldest unbound pending promise to a newly seen operation id. */
function bindPendingPromise(id: string) {
  if (cancellers.has(id)) return
  for (const [token, promise] of pendingCancellables) {
    pendingCancellables.delete(token)
    cancellers.set(id, () => {
      try {
        promise.cancel()
      } catch {
        // The promise may already be settled; cancellation is best-effort.
      }
    })
    return
  }
}

function scheduleRemoval(id: string) {
  const existing = removalTimers.get(id)
  if (existing) clearTimeout(existing)
  removalTimers.set(id, setTimeout(() => {
    progressStates.value = progressStates.value.filter(state => state.id !== id)
    cancellers.delete(id)
    removalTimers.delete(id)
  }, terminalPhaseRetentionMs))
}

function onProgressEvent(event: OperationProgress) {
  progressStates.value = mergeProgressEvent(progressStates.value, event)
  if (isTerminalPhase(event.phase as OperationPhase)) {
    scheduleRemoval(event.operationId)
  } else {
    bindPendingPromise(event.operationId)
  }
}

let eventsBound = false

/** Bind the Wails progress event once for the whole app lifetime. */
export function bindOperationProgressEvents() {
  if (eventsBound) return
  eventsBound = true
  Events.On('filemanager:operation-progress', data => {
    const payload = data.data
    if (payload instanceof OperationProgress) {
      onProgressEvent(payload)
    } else if (payload && typeof payload === 'object') {
      onProgressEvent(OperationProgress.createFrom(payload as Record<string, unknown>))
    }
  })
}

function cancelOperation(id: string) {
  cancellers.get(id)?.()
}

/** Dismiss a finished row immediately. */
function dismissOperation(id: string) {
  progressStates.value = progressStates.value.filter(state => state.id !== id)
  cancellers.delete(id)
  const timer = removalTimers.get(id)
  if (timer) {
    clearTimeout(timer)
    removalTimers.delete(id)
  }
}

export function useOperationProgress() {
  return {
    states: progressStates,
    cancelOperation,
    dismissOperation,
  }
}
