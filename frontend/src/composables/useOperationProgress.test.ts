import { describe, expect, it } from 'vitest'
import { OperationKind, OperationProgress, OperationProgressPhase } from '../../bindings/zashiki/internal/filemanager/models'
import {
  formatBytes,
  isTerminalPhase,
  mergeProgressEvent,
  operationLabel,
  progressByteText,
  progressPercent,
} from './useOperationProgress'

function progressEvent(overrides: Partial<OperationProgress> = {}): OperationProgress {
  return OperationProgress.createFrom({
    operationId: 'op-1',
    kind: OperationKind.OperationKindCopy,
    phase: OperationProgressPhase.OperationProgressPhaseRun,
    totalItems: 3,
    doneItems: 1,
    totalBytes: 100,
    doneBytes: 50,
    currentName: 'movie.mkv',
    ...overrides,
  })
}

describe('mergeProgressEvent', () => {
  it('appends a new operation state', () => {
    const states = mergeProgressEvent([], progressEvent())
    expect(states).toHaveLength(1)
    expect(states[0]).toMatchObject({
      id: 'op-1',
      kind: 'copy',
      phase: 'run',
      totalItems: 3,
      doneItems: 1,
      totalBytes: 100,
      doneBytes: 50,
      currentName: 'movie.mkv',
    })
  })

  it('upserts an existing operation by id', () => {
    let states = mergeProgressEvent([], progressEvent())
    states = mergeProgressEvent(states, progressEvent({ doneItems: 2, doneBytes: 80 }))
    expect(states).toHaveLength(1)
    expect(states[0].doneItems).toBe(2)
    expect(states[0].doneBytes).toBe(80)
  })

  it('keeps unrelated operations untouched', () => {
    let states = mergeProgressEvent([], progressEvent())
    states = mergeProgressEvent(states, progressEvent({ operationId: 'op-2' }))
    expect(states.map(state => state.id)).toEqual(['op-1', 'op-2'])
  })
})

describe('progressPercent', () => {
  it('uses byte ratio when total bytes are known', () => {
    expect(progressPercent(mergeProgressEvent([], progressEvent())[0])).toBe(50)
  })

  it('falls back to item ratio when total bytes are unknown', () => {
    const state = mergeProgressEvent([], progressEvent({ totalBytes: -1, doneBytes: 0 }))[0]
    expect(progressPercent(state)).toBe(33)
  })

  it('clamps above 100 and returns 100 when done', () => {
    const overshoot = mergeProgressEvent([], progressEvent({ doneBytes: 150 }))[0]
    expect(progressPercent(overshoot)).toBe(100)
    const done = mergeProgressEvent([], progressEvent({ phase: OperationProgressPhase.OperationProgressPhaseDone }))[0]
    expect(progressPercent(done)).toBe(100)
  })

  it('returns null when nothing is measurable', () => {
    const state = mergeProgressEvent([], progressEvent({ totalBytes: -1, totalItems: 0 }))[0]
    expect(progressPercent(state)).toBeNull()
  })
})

describe('formatBytes', () => {
  it('formats common sizes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2.0 KB')
    expect(formatBytes(1024 * 1024 * 1.5)).toBe('1.5 MB')
    expect(formatBytes(1024 ** 3)).toBe('1.0 GB')
  })
})

describe('progressByteText', () => {
  it('renders done/total byte pairs', () => {
    const state = mergeProgressEvent([], progressEvent())[0]
    expect(progressByteText(state)).toBe('50 B/100 B')
  })

  it('renders nothing when byte totals are unknown', () => {
    const state = mergeProgressEvent([], progressEvent({ totalBytes: -1 }))[0]
    expect(progressByteText(state)).toBe('')
  })
})

describe('isTerminalPhase', () => {
  it('treats done, error and cancelled as terminal', () => {
    expect(isTerminalPhase('done')).toBe(true)
    expect(isTerminalPhase('error')).toBe(true)
    expect(isTerminalPhase('cancelled')).toBe(true)
    expect(isTerminalPhase('scan')).toBe(false)
    expect(isTerminalPhase('run')).toBe(false)
  })
})

describe('operationLabel', () => {
  it('maps known kinds to Chinese labels', () => {
    expect(operationLabel('copy')).toBe('复制')
    expect(operationLabel('move')).toBe('移动')
    expect(operationLabel('delete')).toBe('删除')
    expect(operationLabel('trash')).toBe('回收')
  })
})
