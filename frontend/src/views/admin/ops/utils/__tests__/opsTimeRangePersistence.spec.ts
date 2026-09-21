import { describe, expect, it } from 'vitest'

import {
  OPS_TIME_RANGE_STORAGE_KEY,
  loadOpsTimeRangeSnapshot,
  saveOpsTimeRangeSnapshot,
  type OpsTimeRangeSnapshot
} from '../opsTimeRangePersistence'

const allowed = new Set(['5m', '30m', '1h', '6h', '24h', 'custom'])

function storage(initial?: string): Storage {
  let value = initial ?? null
  return {
    getItem: () => value,
    setItem: (_key, next) => { value = next },
    removeItem: () => { value = null },
    clear: () => { value = null },
    key: () => null,
    get length() { return value === null ? 0 : 1 }
  }
}

describe('ops time range persistence', () => {
  it('loads a valid preset snapshot', () => {
    const result = loadOpsTimeRangeSnapshot(
      storage(JSON.stringify({ timeRange: '6h' })),
      allowed
    )

    expect(result).toEqual({ timeRange: '6h', customStartTime: null, customEndTime: null })
  })

  it('loads a valid custom snapshot with ordered timestamps', () => {
    const result = loadOpsTimeRangeSnapshot(
      storage(JSON.stringify({
        timeRange: 'custom',
        customStartTime: '2026-09-20T00:00:00.000Z',
        customEndTime: '2026-09-20T01:00:00.000Z'
      })),
      allowed
    )

    expect(result).toEqual({
      timeRange: 'custom',
      customStartTime: '2026-09-20T00:00:00.000Z',
      customEndTime: '2026-09-20T01:00:00.000Z'
    })
  })

  it('rejects invalid ranges, malformed JSON, and reversed custom timestamps', () => {
    expect(loadOpsTimeRangeSnapshot(storage('{"timeRange":"90d"}'), allowed)).toBeNull()
    expect(loadOpsTimeRangeSnapshot(storage('{'), allowed)).toBeNull()
    expect(loadOpsTimeRangeSnapshot(storage(JSON.stringify({
      timeRange: 'custom',
      customStartTime: '2026-09-20T02:00:00.000Z',
      customEndTime: '2026-09-20T01:00:00.000Z'
    })), allowed)).toBeNull()
  })

  it('writes a normalized snapshot and tolerates storage failures', () => {
    const snapshot: OpsTimeRangeSnapshot = {
      timeRange: '1h',
      customStartTime: 'old',
      customEndTime: 'old'
    }
    const target = storage()
    saveOpsTimeRangeSnapshot(target, snapshot)
    expect(target.getItem(OPS_TIME_RANGE_STORAGE_KEY)).toBe(
      JSON.stringify({ timeRange: '1h', customStartTime: null, customEndTime: null })
    )

    const failingStorage = storage()
    failingStorage.setItem = () => { throw new Error('quota') }
    expect(() => saveOpsTimeRangeSnapshot(failingStorage, snapshot)).not.toThrow()
  })
})
