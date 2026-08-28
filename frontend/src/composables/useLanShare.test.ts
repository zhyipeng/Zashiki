import { describe, expect, it } from 'vitest'
import {
  TextReceived,
  TransferProgress,
  TransferProgressPhase,
} from '../../bindings/zashiki/internal/lanshare/models'
import {
  isTerminalTransfer,
  maxReceivedTexts,
  mergeTransferEvent,
  pickPrimaryUrl,
  prependReceivedText,
  transferPercent,
} from './useLanShare'
import type { TransferState } from './useLanShare'

function makeEvent(overrides: Partial<TransferProgress> = {}): TransferProgress {
  return TransferProgress.createFrom({
    transferId: 't1',
    direction: 'download',
    remoteAddr: '192.168.1.8',
    name: 'a.txt',
    totalBytes: 1000,
    doneBytes: 0,
    phase: TransferProgressPhase.TransferPhaseRun,
    ...overrides,
  })
}

function makeState(overrides: Partial<TransferState> = {}): TransferState {
  return {
    id: 't1',
    direction: 'download',
    remoteAddr: '192.168.1.8',
    name: 'a.txt',
    totalBytes: 1000,
    doneBytes: 0,
    phase: 'run',
    error: '',
    ...overrides,
  }
}

describe('isTerminalTransfer', () => {
  it('treats done and error as terminal', () => {
    expect(isTerminalTransfer('done')).toBe(true)
    expect(isTerminalTransfer('error')).toBe(true)
  })

  it('treats run as active', () => {
    expect(isTerminalTransfer('run')).toBe(false)
  })
})

describe('mergeTransferEvent', () => {
  it('appends a new transfer', () => {
    const states = mergeTransferEvent([], makeEvent())
    expect(states).toHaveLength(1)
    expect(states[0].id).toBe('t1')
    expect(states[0].direction).toBe('download')
    expect(states[0].phase).toBe('run')
  })

  it('upserts by transfer id preserving order', () => {
    const first = mergeTransferEvent([], makeEvent())
    const second = mergeTransferEvent(first, makeEvent({ transferId: 't2', name: 'b.txt' }))
    expect(second.map(state => state.id)).toEqual(['t1', 't2'])

    const updated = mergeTransferEvent(second, makeEvent({ doneBytes: 500 }))
    expect(updated).toHaveLength(2)
    expect(updated[0].doneBytes).toBe(500)
    expect(updated.map(state => state.id)).toEqual(['t1', 't2'])
  })

  it('keeps the error message', () => {
    const states = mergeTransferEvent([], makeEvent({
      phase: TransferProgressPhase.TransferPhaseError,
      error: 'client disconnected',
    }))
    expect(states[0].phase).toBe('error')
    expect(states[0].error).toBe('client disconnected')
  })
})

describe('transferPercent', () => {
  it('returns 100 for finished transfers', () => {
    expect(transferPercent(makeState({ phase: 'done' }))).toBe(100)
  })

  it('derives percent from bytes', () => {
    expect(transferPercent(makeState({ doneBytes: 250, totalBytes: 1000 }))).toBe(25)
  })

  it('clamps to 100 when done exceeds total', () => {
    expect(transferPercent(makeState({ doneBytes: 1200, totalBytes: 1000 }))).toBe(100)
  })

  it('returns null when total is unknown', () => {
    expect(transferPercent(makeState({ totalBytes: -1 }))).toBeNull()
  })
})

describe('pickPrimaryUrl', () => {
  it('returns the first URL', () => {
    expect(pickPrimaryUrl(['http://192.168.1.5:53100/ab/', 'http://10.0.0.2:53100/ab/']))
      .toBe('http://192.168.1.5:53100/ab/')
  })

  it('returns empty string without candidates', () => {
    expect(pickPrimaryUrl([])).toBe('')
  })
})

describe('prependReceivedText', () => {
  function makeText(id: string): TextReceived {
    return TextReceived.createFrom({ id, text: `text-${id}`, remoteAddr: '192.168.1.8' })
  }

  it('prepends newest first', () => {
    const texts = prependReceivedText([makeText('1')], makeText('2'))
    expect(texts.map(text => text.id)).toEqual(['2', '1'])
  })

  it('caps the feed', () => {
    let texts: TextReceived[] = []
    for (let i = 0; i < maxReceivedTexts + 5; i++) {
      texts = prependReceivedText(texts, makeText(String(i)))
    }
    expect(texts).toHaveLength(maxReceivedTexts)
    expect(texts[0].id).toBe(String(maxReceivedTexts + 4))
  })
})
