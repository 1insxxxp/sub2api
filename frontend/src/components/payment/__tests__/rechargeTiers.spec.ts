import { describe, expect, it } from 'vitest'
import { resolveRechargeMultiplier, validRechargeTiers } from '../rechargeTiers'

const tiers = [
  { amount: 10, multiplier: 2 }, { amount: 18, multiplier: 3 }, { amount: 80, multiplier: 4 },
]

describe('recharge tiers', () => {
  it.each([[10, 2], [17.99, 5], [18, 3], [79.99, 5], [80, 4], [800, 5]])('matches amount %s', (amount, expected) => {
    expect(resolveRechargeMultiplier(amount, tiers, 5)).toBe(expected)
  })
  it('uses the fixed multiplier when tiers are absent or the amount is not preset', () => {
    expect(resolveRechargeMultiplier(20, [], 5)).toBe(5)
    expect(resolveRechargeMultiplier(5, tiers, 5)).toBe(5)
  })
  it('rejects empty multipliers, overlapping or unsorted tiers', () => {
    expect(validRechargeTiers([{ ...tiers[0], multiplier: 0 }])).toBe(false)
    expect(validRechargeTiers([tiers[1], tiers[0]])).toBe(false)
    expect(validRechargeTiers([{ ...tiers[0], amount: 10 }, tiers[0]])).toBe(false)
    expect(validRechargeTiers([{ ...tiers[0], multiplier: Infinity }])).toBe(false)
    expect(validRechargeTiers(tiers)).toBe(true)
  })
})
