/**
 * User daily check-in API endpoints.
 */

import { apiClient } from './client'

export interface CheckinStatus {
  enabled: boolean
  eligible: boolean
  blacklisted: boolean
  checked_in: boolean
  checkin_date: string
  streak_day?: number | null
  current_streak: number
  lifetime_checkin_days: number
  base_reward_amount?: number | null
  bonus_reward_amount?: number | null
  previous_day_usage_amount?: number | null
  usage_rebate_amount?: number | null
  reward_cap_adjustment?: number | null
  estimated_usage_rebate?: number | null
  total_reward_amount?: number | null
  reward_amount?: number | null
  reward_campaign_id?: number | null
  reward_campaign_name?: string | null
  checked_in_at?: string | null
  next_reset_at?: string
  min_total_usage_usd: number
  total_usage_usd: number
  min_total_recharge_usd: number
  total_recharge_usd: number
  min_daily_usage_count?: number
  today_usage_count?: number
  lottery_attempts_reward?: number
  ineligible_reason?: string
  next_streak_rule?: CheckinStreakRule | null
  recent_records?: CheckinRecord[]
}

export interface CheckinResult extends CheckinStatus {
  already_checked_in: boolean
  balance_before?: number | null
  balance_after: number
}

export interface CheckinStreakRule {
  day: number
  lottery_attempts?: number
  bonus_amount?: number
  bonus_rate_percent?: number
}

export interface CheckinRecord {
  id: number
  user_id: number
  checkin_date: string
  streak_day: number
  base_reward_amount: number
  bonus_reward_amount: number
  lottery_attempts_reward?: number
  previous_day_usage_amount: number
  usage_rebate_amount: number
  reward_cap_adjustment: number
  total_reward_amount: number
  reward_amount: number
  reward_campaign_id?: number | null
  reward_campaign_name?: string | null
  balance_before: number
  balance_after: number
  created_at: string
}

export async function getCheckinStatus(): Promise<CheckinStatus> {
  const { data } = await apiClient.get<CheckinStatus>('/user/checkin/status')
  return data
}

export async function submitCheckin(): Promise<CheckinResult> {
  const { data } = await apiClient.post<CheckinResult>('/user/checkin')
  return data
}
