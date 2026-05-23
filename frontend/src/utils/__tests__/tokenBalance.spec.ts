import { describe, expect, it } from 'vitest'
import {
  estimateAvailableTokens,
  formatAvailableTokens,
  TOKENS_PER_USD_ESTIMATE
} from '@/utils/tokenBalance'

describe('tokenBalance', () => {
  it('estimates available tokens from USD balance', () => {
    expect(TOKENS_PER_USD_ESTIMATE).toBe(1_000_000)
    expect(estimateAvailableTokens(1)).toBe(1_000_000)
    expect(estimateAvailableTokens(0.001)).toBe(1_000)
  })

  it('does not return negative or invalid token estimates', () => {
    expect(estimateAvailableTokens(0)).toBe(0)
    expect(estimateAvailableTokens(-1)).toBe(0)
    expect(estimateAvailableTokens(Number.NaN)).toBeNull()
    expect(estimateAvailableTokens(undefined)).toBeNull()
  })

  it('formats available token counts compactly', () => {
    expect(formatAvailableTokens(1_034_000_000)).toBe('10.34亿')
    expect(formatAvailableTokens(12_500_000)).toBe('1250万')
    expect(formatAvailableTokens(12_345)).toBe('1.23万')
    expect(formatAvailableTokens(999)).toBe('999')
    expect(formatAvailableTokens(null)).toBe('-')
  })
})
