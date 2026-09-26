import { describe, expect, it } from 'vitest'
import {
  localDateTimeToRFC3339,
  rfc3339ToLocalDateTime,
  validateRechargePromotion,
} from '../rechargePromotion'

describe('RechargePromotionEditor helpers', () => {
  it('converts datetime-local values using the browser local timezone', () => {
    const value = localDateTimeToRFC3339('2026-09-26T09:30')
    expect(value).toMatch(/^2026-09-26T09:30:00[+-]\d{2}:\d{2}$/)
    expect(rfc3339ToLocalDateTime(value)).toBe('2026-09-26T09:30')
  })

  it('validates only configured promotion values and tolerates cleared dates', () => {
    expect(validateRechargePromotion({ enabled: false, multiplier: 0 })).toBeUndefined()
    expect(validateRechargePromotion({ enabled: true, multiplier: 0 })).toBe('multiplier')
    expect(validateRechargePromotion({ enabled: true, multiplier: 2, start_at: '', end_at: '' })).toBe('range')
    expect(validateRechargePromotion({ enabled: true, multiplier: 2, start_at: '2026-09-27T00:00:00+08:00', end_at: '2026-09-26T00:00:00+08:00' })).toBe('range')
  })
})
