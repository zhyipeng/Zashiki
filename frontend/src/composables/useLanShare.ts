import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { LanShareService } from '../../bindings/zashiki/internal/lanshare'
import {
  ItemsChanged,
  ServerStatus,
  ShareItem,
  TextReceived,
  TransferProgress,
} from '../../bindings/zashiki/internal/lanshare/models'

export type TransferDirection = 'download' | 'upload'
export type TransferPhase = 'run' | 'done' | 'error'

export interface TransferState {
  id: string
  direction: TransferDirection
  remoteAddr: string
  name: string
  totalBytes: number
  doneBytes: number
  phase: TransferPhase
  error: string
}

export interface ShareRequest {
  paths: string[]
  seq: number
}

/** Terminal phases after which a transfer row lingers briefly, then disappears. */
export function isTerminalTransfer(phase: TransferPhase): boolean {
  return phase === 'done' || phase === 'error'
}

/** Percent in [0, 100]; null when total size is unknown. */
export function transferPercent(state: TransferState): number | null {
  if (state.phase === 'done') return 100
  if (state.totalBytes > 0) {
    return Math.min(100, Math.floor((state.doneBytes / state.totalBytes) * 100))
  }
  return null
}

/** Merge one backend event into the state list (upsert by transfer id). */
export function mergeTransferEvent(
  states: TransferState[],
  event: TransferProgress,
): TransferState[] {
  const state: TransferState = {
    id: event.transferId,
    direction: event.direction as TransferDirection,
    remoteAddr: event.remoteAddr,
    name: event.name,
    totalBytes: event.totalBytes,
    doneBytes: event.doneBytes,
    phase: event.phase as TransferPhase,
    error: event.error ?? '',
  }
  const index = states.findIndex(item => item.id === state.id)
  if (index === -1) return [...states, state]
  const next = states.slice()
  next[index] = state
  return next
}

/** Cap for the received-text feed: oldest entries fall off. */
export const maxReceivedTexts = 20

/** Prepend a received text, trimming the feed to the cap. */
export function prependReceivedText(
  texts: TextReceived[],
  text: TextReceived,
): TextReceived[] {
  return [text, ...texts].slice(0, maxReceivedTexts)
}

/** The first (highest-ranked) candidate URL, for the QR code. */
export function pickPrimaryUrl(urls: string[]): string {
  return urls[0] ?? ''
}

const transferRetentionMs = 5000

const shareStatus = ref<ServerStatus | null>(null)
const shareItems = ref<ShareItem[]>([])
const transfers = ref<TransferState[]>([])
const receivedTexts = ref<TextReceived[]>([])
/** File paths queued by the FileTable quick entry, consumed by TransferPage. */
const pendingShareRequest = ref<ShareRequest | null>(null)

const removalTimers = new Map<string, ReturnType<typeof setTimeout>>()

function scheduleTransferRemoval(id: string) {
  const existing = removalTimers.get(id)
  if (existing) clearTimeout(existing)
  removalTimers.set(id, setTimeout(() => {
    transfers.value = transfers.value.filter(state => state.id !== id)
    removalTimers.delete(id)
  }, transferRetentionMs))
}

let eventsBound = false

/** Bind the Wails lanshare events once for the whole app lifetime. */
export function bindLanShareEvents() {
  if (eventsBound) return
  eventsBound = true
  Events.On('lanshare:server-status', data => {
    shareStatus.value = ServerStatus.createFrom(data.data)
  })
  Events.On('lanshare:items-changed', data => {
    shareItems.value = ItemsChanged.createFrom(data.data).items
  })
  Events.On('lanshare:transfer-progress', data => {
    const event = TransferProgress.createFrom(data.data)
    transfers.value = mergeTransferEvent(transfers.value, event)
    if (isTerminalTransfer(event.phase as TransferPhase)) {
      scheduleTransferRemoval(event.transferId)
    }
  })
  Events.On('lanshare:text-received', data => {
    receivedTexts.value = prependReceivedText(
      receivedTexts.value,
      TextReceived.createFrom(data.data),
    )
  })
}

/** Queue a quick-entry share request; App switches to the transfer view. */
export function requestLanShare(paths: string[]) {
  if (paths.length === 0) return
  pendingShareRequest.value = { paths: [...paths], seq: Date.now() }
}

/** Consume the queued quick-entry request (TransferPage side). */
export function consumeShareRequest(): ShareRequest | null {
  const request = pendingShareRequest.value
  pendingShareRequest.value = null
  return request
}

async function refreshStatus() {
  try {
    shareStatus.value = await LanShareService.GetStatus()
  } catch (err) {
    console.error('GetStatus failed:', err)
  }
}

export function useLanShare() {
  return {
    status: shareStatus,
    items: shareItems,
    transfers,
    receivedTexts,
    pendingShareRequest,
    primaryUrl: () => pickPrimaryUrl(shareStatus.value?.urls ?? []),
    refreshStatus,
    ensureServer: () => LanShareService.EnsureServer(),
    addFiles: (paths: string[]) => LanShareService.AddFiles(paths),
    addText: (text: string) => LanShareService.AddText(text),
    removeItem: (id: string) => LanShareService.RemoveItem(id),
    clearItems: () => LanShareService.ClearItems(),
    stopServer: () => LanShareService.StopServer(),
    requestLanShare,
    consumeShareRequest,
  }
}
