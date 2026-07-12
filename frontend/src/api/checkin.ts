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
  total_reward_amount?: number | null
  reward_amount?: number | null
  checked_in_at?: string | null
  next_reset_at?: string
  min_total_usage_usd: number
  total_usage_usd: number
  min_total_recharge_usd: number
  total_recharge_usd: number
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
  bonus_amount: number
}

export interface CheckinRecord {
  id: number
  user_id: number
  checkin_date: string
  streak_day: number
  base_reward_amount: number
  bonus_reward_amount: number
  total_reward_amount: number
  reward_amount: number
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
