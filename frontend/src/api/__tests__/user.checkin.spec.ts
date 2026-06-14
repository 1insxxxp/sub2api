import { describe, expect, it, vi, beforeEach } from 'vitest'

const apiGet = vi.fn()
const apiPost = vi.fn()

vi.mock('@/api/client', () => ({
  apiClient: {
    get: (...args: unknown[]) => apiGet(...args),
    post: (...args: unknown[]) => apiPost(...args),
  },
}))

describe('user check-in api', () => {
  beforeEach(() => {
    apiGet.mockReset()
    apiPost.mockReset()
  })

  it('loads current check-in status from the production user path', async () => {
    const status = {
      enabled: true,
      checked_in_today: false,
      current_streak: 6,
      lifetime_checkin_days: 12,
    }
    apiGet.mockResolvedValue({ data: status })

    const { getCheckinStatus } = await import('@/api/user')
    await expect(getCheckinStatus()).resolves.toBe(status)

    expect(apiGet).toHaveBeenCalledWith('/user/checkin/status')
  })

  it('submits daily check-in to the production user path', async () => {
    const result = {
      enabled: true,
      checked_in_today: true,
      total_reward_amount: 11,
      balance_after: 23.5,
    }
    apiPost.mockResolvedValue({ data: result })

    const { checkin } = await import('@/api/user')
    await expect(checkin()).resolves.toBe(result)

    expect(apiPost).toHaveBeenCalledWith('/user/checkin')
  })
})
