import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post }
}))

import { getAffiliateReferralStatus, getPublicSettings, resolveAffiliateReferral } from '@/api/auth'

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

describe('registration-only human verification', () => {
  const settings = { registration_enabled: true, turnstile_enabled: false, turnstile_site_key: 'widget-id' }

  beforeEach(() => get.mockReset())

  it('activates the configured widget only for a challenged registration', async () => {
    get.mockResolvedValueOnce({ data: settings }).mockResolvedValueOnce({ data: { provider: 'turnstile' } })
    await expect(getPublicSettings('registration')).resolves.toMatchObject({
      turnstile_enabled: true,
      turnstile_site_key: 'widget-id'
    })
    expect(get).toHaveBeenNthCalledWith(2, '/auth/registration-challenge')
    expect(settings.turnstile_enabled).toBe(false)
  })

  it('keeps the other public settings when the preflight cannot be read', async () => {
    get.mockResolvedValueOnce({ data: settings }).mockRejectedValueOnce(new Error('unavailable'))
    await expect(getPublicSettings('registration')).resolves.toBe(settings)
  })

  it('loads ordinary login settings without a challenge lookup', async () => {
    get.mockResolvedValueOnce({ data: settings })
    await expect(getPublicSettings()).resolves.toBe(settings)
    expect(get).toHaveBeenCalledTimes(1)
  })
})
