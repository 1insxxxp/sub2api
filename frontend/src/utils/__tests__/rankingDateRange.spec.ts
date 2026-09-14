import { describe, expect, it } from 'vitest'
import { getRankingDateRange } from '../rankingDateRange'

describe('getRankingDateRange', () => {
  const now = new Date(2026, 8, 15, 16, 30)
  it.each([
    ['today', '2026-09-15', '2026-09-15'],
    ['yesterday', '2026-09-14', '2026-09-14'],
    ['7days', '2026-09-09', '2026-09-15'],
    ['30days', '2026-08-17', '2026-09-15'],
  ] as const)('returns inclusive %s calendar dates', (preset, start, end) => {
    expect(getRankingDateRange(preset, now)).toEqual({ start, end })
  })
})
