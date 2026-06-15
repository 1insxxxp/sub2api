/**
 * Admin daily check-in management API endpoints.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface AdminCheckinStats {
  today_count: number
  today_reward_total: number
  seven_day_count: number
  seven_day_reward_total: number
  thirty_day_count: number
  thirty_day_reward_total: number
  active_blacklist_count: number
  current_checkin_day: string
}

export interface AdminCheckinRecord {
  id: number
  user_id: number
  user_email?: string
  username?: string
  checkin_date: string
  reward_amount: number
  balance_before: number
  balance_after: number
  created_at: string
}

export interface AdminCheckinBlacklistEntry {
  id: number
  user_id: number
  user_email?: string
  username?: string
  reason?: string
  created_by?: number | null
  removed_by?: number | null
  removed_at?: string | null
  created_at: string
  updated_at: string
}

export interface AdminCheckinRecordFilters {
  user_id?: number
  date?: string
  search?: string
}

export interface AdminCheckinBlacklistFilters {
  active_only?: boolean
  search?: string
}

export interface AddCheckinBlacklistRequest {
  user_id: number
  reason?: string
}

export async function getStats(): Promise<AdminCheckinStats> {
  const { data } = await apiClient.get<AdminCheckinStats>('/admin/checkins/stats')
  return data
}

export async function listRecords(
  page: number = 1,
  pageSize: number = 20,
  filters?: AdminCheckinRecordFilters,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminCheckinRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminCheckinRecord>>(
    '/admin/checkins/records',
    {
      params: { page, page_size: pageSize, ...filters },
      signal: options?.signal,
    }
  )
  return data
}

export async function listBlacklist(
  page: number = 1,
  pageSize: number = 20,
  filters?: AdminCheckinBlacklistFilters,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminCheckinBlacklistEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminCheckinBlacklistEntry>>(
    '/admin/checkins/blacklist',
    {
      params: { page, page_size: pageSize, ...filters },
      signal: options?.signal,
    }
  )
  return data
}

export async function addBlacklist(
  request: AddCheckinBlacklistRequest
): Promise<AdminCheckinBlacklistEntry> {
  const { data } = await apiClient.post<AdminCheckinBlacklistEntry>(
    '/admin/checkins/blacklist',
    request
  )
  return data
}

export async function removeBlacklist(userId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/checkins/blacklist/${userId}`
  )
  return data
}

export const checkinsAPI = {
  getStats,
  listRecords,
  listBlacklist,
  addBlacklist,
  removeBlacklist,
}

export default checkinsAPI
