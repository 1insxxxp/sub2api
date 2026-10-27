import { describe, expect, it } from 'vitest'
import { COMMON_ERROR_STATUS_CODES, errorDisplayStatus } from '../errorBadges'

describe('error request downstream status', () => {
  it('keeps canceled connection status separate from a prior provider error', () => {
    expect(errorDisplayStatus({ status_code: 503, client_canceled: true, downstream_status_code: 200 })).toBe(200)
    expect(errorDisplayStatus({ status_code: 503, client_canceled: true })).toBe(0)
    expect(errorDisplayStatus({ status_code: 503, client_canceled: false, downstream_status_code: 200 })).toBe(503)
    expect(COMMON_ERROR_STATUS_CODES).toContain(200)
  })
})
