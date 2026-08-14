import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post }
}))

import { getAffiliateReferralStatus, resolveAffiliateReferral } from '@/api/auth'

describe('affiliate referral lock API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('normalizes and resolves an affiliate referral without exposing extra fields', async () => {
    post.mockResolvedValue({ data: { valid: true, locked: true, ignored: 'secret' } })

    await expect(resolveAffiliateReferral('  AFF123  ')).resolves.toEqual({
      valid: true,
      locked: true
    })
    expect(post).toHaveBeenCalledWith('/auth/affiliate-referral/resolve', {
      aff_code: 'AFF123'
    })
  })

  it('returns only the referral lock status', async () => {
    get.mockResolvedValue({ data: { locked: true, ignored: 'secret' } })

    await expect(getAffiliateReferralStatus()).resolves.toEqual({ locked: true })
    expect(get).toHaveBeenCalledWith('/auth/affiliate-referral/status')
  })

  it.each([404, 405])('preserves status %s for blue-green fallback', async (status) => {
    const error = { status, message: 'not available' }
    post.mockRejectedValue(error)

    await expect(resolveAffiliateReferral('AFF123')).rejects.toBe(error)
  })
})
