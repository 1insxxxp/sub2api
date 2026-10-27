import { describe, expect, it } from 'vitest'
import { exportUsageTokenCount, formatUsageTokenCount, usageTokenSourceLabel } from '../usageTokenSource'

const t = (key: string) => key

describe('usage token source presentation', () => {
  it('keeps unreported input out of numeric exports without erasing a measured zero', () => {
    expect(exportUsageTokenCount(0, 'unreported')).toBe('')
    expect(exportUsageTokenCount(0, 'upstream')).toBe(0)
    expect(formatUsageTokenCount(0, 'unreported', t)).toBe('usage.tokensUnreported')
    expect(formatUsageTokenCount(0, 'upstream', t)).toBe('0')
  })

  it('retains legacy and estimated values with distinct source labels', () => {
    expect(exportUsageTokenCount(21916, undefined)).toBe(21916)
    expect(exportUsageTokenCount(5071, 'estimated')).toBe(5071)
    expect(usageTokenSourceLabel(undefined, t)).toBe('usage.tokensSourceUnknown')
    expect(usageTokenSourceLabel('unknown', t)).toBe('usage.tokensSourceUnknown')
    expect(usageTokenSourceLabel('estimated', t)).toBe('usage.tokensEstimated')
    expect(usageTokenSourceLabel('upstream', t)).toBe('usage.tokensSourceUpstream')
    expect(usageTokenSourceLabel('unreported', t)).toBe('usage.tokensUnreported')
  })
})
