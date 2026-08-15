import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post },
}))

import {
  getCheckinStatus,
  submitCheckin,
  type CheckinRecord,
  type CheckinResult,
  type CheckinStatus,
} from '@/api/checkin'

const campaignRecord = {
  id: 1,
  user_id: 12,
  checkin_date: '2026-08-15',
  streak_day: 2,
  base_reward_amount: 1,
  bonus_reward_amount: 0,
  previous_day_usage_amount: 0,
  usage_rebate_amount: 0,
  reward_cap_adjustment: 0,
  total_reward_amount: 1,
  reward_amount: 1,
  reward_campaign_id: 42,
  reward_campaign_name: 'Summer surprise',
  balance_before: 10,
  balance_after: 11,
  created_at: '2026-08-15T01:00:00Z',
} satisfies CheckinRecord

const baselineRecord = {
  ...campaignRecord,
  id: 2,
  reward_campaign_id: null,
  reward_campaign_name: null,
} satisfies CheckinRecord

const campaignStatus = {
  enabled: true,
  eligible: true,
  blacklisted: false,
  checked_in: false,
  checkin_date: '2026-08-16',
  current_streak: 2,
  lifetime_checkin_days: 8,
  min_total_usage_usd: 0,
  total_usage_usd: 0,
  min_total_recharge_usd: 0,
  total_recharge_usd: 0,
  reward_campaign_id: 42,
  reward_campaign_name: 'Summer surprise',
  recent_records: [campaignRecord, baselineRecord],
} satisfies CheckinStatus

describe('user check-in campaign response contract', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('preserves optional campaign origin fields in status and history responses', async () => {
    get.mockResolvedValueOnce({ data: campaignStatus })

    await expect(getCheckinStatus()).resolves.toBe(campaignStatus)
    expect(get).toHaveBeenCalledWith('/user/checkin/status')
  })

  it('preserves the stored campaign snapshot after check-in', async () => {
    const result = {
      ...campaignStatus,
      checked_in: true,
      already_checked_in: false,
      reward_campaign_name: 'Stored snapshot',
      balance_after: 11,
    } satisfies CheckinResult
    post.mockResolvedValueOnce({ data: result })

    await expect(submitCheckin()).resolves.toBe(result)
    expect(post).toHaveBeenCalledWith('/user/checkin')
  })
})
