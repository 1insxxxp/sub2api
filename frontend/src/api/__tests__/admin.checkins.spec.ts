import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post, deleteRequest } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
  deleteRequest: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
    delete: deleteRequest,
  },
}))

import checkinsAPI from '@/api/admin/checkins'

describe('admin check-ins api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    post.mockReset()
    deleteRequest.mockReset()
  })

  it('loads stats, records, blacklist and updates blacklist through backend endpoints', async () => {
    get.mockResolvedValueOnce({ data: { today_count: 1 } })
    get.mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 0 } })
    get.mockResolvedValueOnce({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 0 } })
    post.mockResolvedValueOnce({ data: { id: 7, user_id: 99 } })
    deleteRequest.mockResolvedValueOnce({ data: { message: 'ok' } })

    await checkinsAPI.getStats()
    await checkinsAPI.listRecords(2, 50, { search: 'alice', date: '2026-06-05' })
    await checkinsAPI.listBlacklist(1, 20, { active_only: true, search: 'alice' })
    await checkinsAPI.addBlacklist({ user_id: 99, reason: 'abuse' })
    await checkinsAPI.removeBlacklist(99)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/checkins/stats')
    expect(get).toHaveBeenNthCalledWith(2, '/admin/checkins/records', {
      params: { page: 2, page_size: 50, search: 'alice', date: '2026-06-05' },
      signal: undefined,
    })
    expect(get).toHaveBeenNthCalledWith(3, '/admin/checkins/blacklist', {
      params: { page: 1, page_size: 20, active_only: true, search: 'alice' },
      signal: undefined,
    })
    expect(post).toHaveBeenCalledWith('/admin/checkins/blacklist', {
      user_id: 99,
      reason: 'abuse',
    })
    expect(deleteRequest).toHaveBeenCalledWith('/admin/checkins/blacklist/99')
  })

  it('loads and saves usage-linked reward configuration', async () => {
    const config = {
      enabled: true,
      min_total_usage_usd: 25,
      min_total_recharge_usd: 50,
      tiers: [{ amount: 0.3, probability: 100, sort_order: 1 }],
      streak_enabled: true,
      streak_rules: [{ day: 7, bonus_amount: 0, bonus_rate_percent: 10 }],
      usage_rebate_enabled: true,
      usage_rebate_rate_percent: 8,
      usage_rebate_cap: 8,
      total_reward_cap: 10,
      probability_total: 100,
      preview: { min_reward: 0.3, max_reward: 0.3, average_reward: 0.3 },
    }
    get.mockResolvedValueOnce({ data: config })
    put.mockResolvedValueOnce({ data: config })

    await checkinsAPI.getConfig()
    await checkinsAPI.updateConfig(config)

    expect(get).toHaveBeenCalledWith('/admin/checkins/config')
    expect(put).toHaveBeenCalledWith('/admin/checkins/config', config)
  })
})
