/**
 * User daily check-in API endpoints.
 */

import { apiClient } from './client'

export interface CheckinStatus {
  enabled: boolean
  blacklisted: boolean
  checked_in: boolean
  checkin_date: string
  reward_amount?: number | null
  checked_in_at?: string | null
  next_reset_at?: string
}

export interface CheckinResult extends CheckinStatus {
  already_checked_in: boolean
  balance_before?: number | null
  balance_after: number
}

export async function getCheckinStatus(): Promise<CheckinStatus> {
  const { data } = await apiClient.get<CheckinStatus>('/user/checkin/status')
  return data
}

export async function submitCheckin(): Promise<CheckinResult> {
  const { data } = await apiClient.post<CheckinResult>('/user/checkin')
  return data
}
