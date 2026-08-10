import { describe, expect, it } from 'vitest'
import { formatAvailableChannelRate } from '../availableChannelRateDisplay'

describe('formatAvailableChannelRate', () => {
  it.each([
    [7.5, '7.50'],
    [1.239, '1.23'],
    [0.035, '0.03'],
    [1.999, '1.99'],
    [1.15, '1.15'],
    [0.29, '0.29'],
  ])('formats %s by truncating downward to two decimals', (value, expected) => {
    expect(formatAvailableChannelRate(value)).toBe(expected)
  })

  it.each([null, undefined, Number.NaN, Number.POSITIVE_INFINITY, -0.01])(
    'returns a placeholder for invalid value %s',
    value => {
      expect(formatAvailableChannelRate(value)).toBe('-')
    },
  )
})
