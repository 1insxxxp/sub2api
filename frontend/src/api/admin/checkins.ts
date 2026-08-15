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

export interface AdminCheckinConfig {
  enabled: boolean
  min_total_usage_usd: number
  min_total_recharge_usd: number
  tiers: CheckinRewardTier[]
  streak_enabled: boolean
  streak_rules: CheckinStreakRule[]
  usage_rebate_enabled: boolean
  usage_rebate_rate_percent: number
  usage_rebate_cap: number
  total_reward_cap: number
  probability_total: number
  preview: CheckinRewardPreview
}

export interface CheckinRewardTier {
  amount: number
  probability: number
  sort_order: number
}

export interface CheckinStreakRule {
  day: number
  bonus_amount: number
  bonus_rate_percent?: number
}

export interface CheckinRewardPreview {
  min_reward: number
  max_reward: number
  average_reward: number
}

export type CheckinRewardCampaignStatus = 'draft' | 'enabled' | 'disabled'

export type CheckinRewardCampaignLifecycle =
  | 'draft'
  | 'upcoming'
  | 'active'
  | 'ended'
  | 'disabled'

export interface AdminCheckinRewardCampaign {
  id: number
  name: string
  status: CheckinRewardCampaignStatus
  lifecycle_status: CheckinRewardCampaignLifecycle
  start_date: string
  end_date: string
  reward_tiers: CheckinRewardTier[]
  probability_total: number
  preview: CheckinRewardPreview
  created_by?: number | null
  updated_by?: number | null
  created_at: string
  updated_at: string
}

export interface CreateCheckinRewardCampaignRequest {
  name: string
  start_date: string
  end_date: string
  reward_tiers: CheckinRewardTier[]
}

export interface UpdateCheckinRewardCampaignRequest {
  name: string
  start_date: string
  end_date: string
  reward_tiers: CheckinRewardTier[]
}

export interface CopyCheckinRewardCampaignRequest {
  name: string
}

export interface DeleteCheckinRewardCampaignResponse {
  id: number
  deleted: boolean
}

export interface AdminCheckinRecord {
  id: number
  user_id: number
  user_email?: string
  username?: string
  checkin_date: string
  streak_day: number
  base_reward_amount: number
  bonus_reward_amount: number
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

export async function getConfig(): Promise<AdminCheckinConfig> {
  const { data } = await apiClient.get<AdminCheckinConfig>('/admin/checkins/config')
  return data
}

export async function updateConfig(
  request: AdminCheckinConfig
): Promise<AdminCheckinConfig> {
  const { data } = await apiClient.put<AdminCheckinConfig>('/admin/checkins/config', request)
  return data
}

export async function listCampaigns(
  lifecycle?: CheckinRewardCampaignLifecycle | 'all',
  options?: { signal?: AbortSignal }
): Promise<AdminCheckinRewardCampaign[]> {
  const { data } = await apiClient.get<AdminCheckinRewardCampaign[]>(
    '/admin/checkins/campaigns',
    {
      params: { lifecycle },
      signal: options?.signal,
    }
  )
  return data
}

export async function getCampaign(
  id: number,
  options?: { signal?: AbortSignal }
): Promise<AdminCheckinRewardCampaign> {
  const { data } = await apiClient.get<AdminCheckinRewardCampaign>(
    `/admin/checkins/campaigns/${id}`,
    { signal: options?.signal }
  )
  return data
}

export async function createCampaign(
  request: CreateCheckinRewardCampaignRequest
): Promise<AdminCheckinRewardCampaign> {
  const { data } = await apiClient.post<AdminCheckinRewardCampaign>(
    '/admin/checkins/campaigns',
    request
  )
  return data
}

export async function updateCampaign(
  id: number,
  request: UpdateCheckinRewardCampaignRequest
): Promise<AdminCheckinRewardCampaign> {
  const { data } = await apiClient.put<AdminCheckinRewardCampaign>(
    `/admin/checkins/campaigns/${id}`,
    request
  )
  return data
}

export async function enableCampaign(id: number): Promise<AdminCheckinRewardCampaign> {
  const { data } = await apiClient.post<AdminCheckinRewardCampaign>(
    `/admin/checkins/campaigns/${id}/enable`
  )
  return data
}

export async function disableCampaign(id: number): Promise<AdminCheckinRewardCampaign> {
  const { data } = await apiClient.post<AdminCheckinRewardCampaign>(
    `/admin/checkins/campaigns/${id}/disable`
  )
  return data
}

export async function copyCampaign(
  id: number,
  request: CopyCheckinRewardCampaignRequest
): Promise<AdminCheckinRewardCampaign> {
  const { data } = await apiClient.post<AdminCheckinRewardCampaign>(
    `/admin/checkins/campaigns/${id}/copy`,
    request
  )
  return data
}

export async function deleteCampaign(
  id: number
): Promise<DeleteCheckinRewardCampaignResponse> {
  const { data } = await apiClient.delete<DeleteCheckinRewardCampaignResponse>(
    `/admin/checkins/campaigns/${id}`
  )
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
  getConfig,
  updateConfig,
  getStats,
  listCampaigns,
  getCampaign,
  createCampaign,
  updateCampaign,
  enableCampaign,
  disableCampaign,
  copyCampaign,
  deleteCampaign,
  listRecords,
  listBlacklist,
  addBlacklist,
  removeBlacklist,
}

export default checkinsAPI
