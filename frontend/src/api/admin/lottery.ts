import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { LotteryActivity, LotteryAttemptMode, LotteryPrize, LotteryPrizeType } from '@/api/lottery'

export interface LotteryActivityConfig {
  activity?: LotteryActivity | null
  prizes: LotteryPrize[]
}

export interface SaveLotteryActivityRequest {
  id?: number
  name: string
  description: string
  status: string
  attempt_mode: LotteryAttemptMode
  attempt_limit: number
  starts_at?: string | null
  ends_at?: string | null
}

export interface SaveLotteryPrizeRequest {
  activity_id?: number
  name: string
  description: string
  type: LotteryPrizeType
  weight: number
  balance_amount?: number | null
  enabled: boolean
  sort_order: number
}

export interface LotteryPrizeItem {
  id: number
  prize_id: number
  content: string
  status: 'available' | 'claimed'
  claimed_by?: number | null
  claimed_at?: string | null
  created_at: string
}

export interface LotteryAdminDraw {
  id: number
  activity_id?: number | null
  prize_id?: number | null
  user_id: number
  user_email?: string
  user_name?: string
  user_deleted: boolean
  prize_name: string
  prize_type: LotteryPrizeType
  balance_amount?: number | null
  product_content?: string | null
  attempt_source?: 'activity' | 'wallet'
  created_at: string
}

export interface LotteryAdminDrawQuery {
  page?: number
  page_size?: number
}

export interface LotteryAttemptGrantRequest {
  all?: boolean
  user_ids?: number[]
  amount: number
  description?: string
  request_key: string
}

export interface LotteryAttemptGrantResult {
  affected: number
  total_granted: number
}

export async function getConfig(): Promise<LotteryActivityConfig> {
  const { data } = await apiClient.get<LotteryActivityConfig>('/admin/lottery/config')
  return data
}

export async function saveActivity(request: SaveLotteryActivityRequest): Promise<LotteryActivity> {
  const { data } = await apiClient.put<LotteryActivity>('/admin/lottery/activity', request)
  return data
}

export async function createPrize(request: SaveLotteryPrizeRequest): Promise<LotteryPrize> {
  const { data } = await apiClient.post<LotteryPrize>('/admin/lottery/prizes', request)
  return data
}

export async function updatePrize(id: number, request: Omit<SaveLotteryPrizeRequest, 'activity_id'>): Promise<LotteryPrize> {
  const { data } = await apiClient.put<LotteryPrize>(`/admin/lottery/prizes/${id}`, request)
  return data
}

export async function deletePrize(id: number): Promise<void> {
  await apiClient.delete(`/admin/lottery/prizes/${id}`)
}

export async function listPrizeItems(id: number): Promise<LotteryPrizeItem[]> {
  const { data } = await apiClient.get<LotteryPrizeItem[]>(`/admin/lottery/prizes/${id}/items`)
  return data
}

export async function appendPrizeItems(id: number, contents: string[]): Promise<{ added: number }> {
  const { data } = await apiClient.post<{ added: number }>(`/admin/lottery/prizes/${id}/items`, { contents })
  return data
}

export async function deletePrizeItems(id: number, itemIds: number[]): Promise<{ deleted: number }> {
  const { data } = await apiClient.delete<{ deleted: number }>(`/admin/lottery/prizes/${id}/items`, { data: { item_ids: itemIds } })
  return data
}

export async function listDraws(params: LotteryAdminDrawQuery = {}): Promise<PaginatedResponse<LotteryAdminDraw>> {
  const { data } = await apiClient.get<PaginatedResponse<LotteryAdminDraw>>('/admin/lottery/draws', { params })
  return data
}

export async function grantAttempts(request: LotteryAttemptGrantRequest): Promise<LotteryAttemptGrantResult> {
  const { data } = await apiClient.post<LotteryAttemptGrantResult>('/admin/lottery/attempts/grant', request, {
    headers: { 'Idempotency-Key': request.request_key }
  })
  return data
}

export const lotteryAdminAPI = { getConfig, saveActivity, createPrize, updatePrize, deletePrize, listPrizeItems, appendPrizeItems, deletePrizeItems, listDraws, grantAttempts }

export default lotteryAdminAPI
