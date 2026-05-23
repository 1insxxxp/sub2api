import { describe, expect, it } from 'vitest'
import { formatCompactNumber, formatTokenCount, formatTokensK } from '../format'

describe('formatCompactNumber', () => {
  it('formats boundary values with K/M/B', () => {
    expect(formatCompactNumber(0)).toBe('0')
    expect(formatCompactNumber(999)).toBe('999')
    expect(formatCompactNumber(1000)).toBe('1.0K')
    expect(formatCompactNumber(999999)).toBe('1000.0K')
    expect(formatCompactNumber(1000000)).toBe('1.0M')
    expect(formatCompactNumber(1000000000)).toBe('1.0B')
  })

  it('supports disabling billion unit (requests style)', () => {
    expect(formatCompactNumber(1000000000, { allowBillions: false })).toBe('1000.0M')
  })

  it('returns 0 for nullish input', () => {
    expect(formatCompactNumber(null)).toBe('0')
    expect(formatCompactNumber(undefined)).toBe('0')
  })
})

describe('formatTokenCount', () => {
  it('formats token counts with Chinese units', () => {
    expect(formatTokenCount(0)).toBe('0')
    expect(formatTokenCount(9999)).toBe('9,999')
    expect(formatTokenCount(10000)).toBe('1万')
    expect(formatTokenCount(12345)).toBe('1.23万')
    expect(formatTokenCount(100000000)).toBe('1亿')
    expect(formatTokenCount(123456789)).toBe('1.23亿')
  })

  it('keeps the legacy token formatter on Chinese units', () => {
    expect(formatTokensK(12345)).toBe('1.23万')
  })
})
