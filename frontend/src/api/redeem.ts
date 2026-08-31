/**
 * Redeem code API endpoints
 * Handles redeem code redemption for users
 */

import { apiClient } from './client'
import type { PaginatedResponse, RedeemCode, RedeemCodeRequest } from '@/types'

export interface RedeemHistoryItem {
  id: number
  code: string
  type: string
  value: number
  status: string
  used_at: string
  created_at: string
  // Notes from admin for admin_balance/admin_concurrency types
  notes?: string
  // Subscription-specific fields
  group_id?: number
  validity_days?: number
  group?: {
    id: number
    name: string
  }
}

export interface GenerateBalanceTransferCodeRequest {
  amount: number
  count?: number
  expires_in_days?: number
  notes?: string
  single_use_per_user?: boolean
  threshold_exempt?: boolean
}

export type GeneratedRedeemCode = RedeemCode

export interface RedeemListParams {
  page: number
  page_size: number
}

export interface RedeemResult {
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_id?: number
  group_name?: string
  validity_days?: number
}

/**
 * Redeem a code
 * @param code - Redeem code string
 * @returns Redemption result with updated balance or concurrency
 */
export async function redeem(code: string): Promise<RedeemResult> {
  const payload: RedeemCodeRequest = { code }

  const { data } = await apiClient.post<RedeemResult>('/redeem', payload)

  return data
}

/**
 * Get user's redemption history
 * @returns List of redeemed codes
 */
export async function getHistory(
  params: RedeemListParams
): Promise<PaginatedResponse<RedeemHistoryItem>> {
  const { data } = await apiClient.get<PaginatedResponse<RedeemHistoryItem>>('/redeem/history', {
    params
  })
  return data
}

export async function generateBalanceTransferCode(
  payload: GenerateBalanceTransferCodeRequest
): Promise<GeneratedRedeemCode> {
  const codes = await generateBalanceTransferCodes(payload)
  return codes[0]
}

export async function generateBalanceTransferCodes(
  payload: GenerateBalanceTransferCodeRequest
): Promise<GeneratedRedeemCode[]> {
  const { data } = await apiClient.post<GeneratedRedeemCode[] | GeneratedRedeemCode>(
    '/admin/workbench/redeem/generated',
    payload
  )
  return Array.isArray(data) ? data : [data]
}

export async function getGenerated(
  params: RedeemListParams
): Promise<PaginatedResponse<GeneratedRedeemCode>> {
  const { data } = await apiClient.get<PaginatedResponse<GeneratedRedeemCode>>(
    '/admin/workbench/redeem/generated',
    { params }
  )
  return data
}

export async function deleteGenerated(id: number): Promise<GeneratedRedeemCode> {
  const { data } = await apiClient.delete<GeneratedRedeemCode>(
    `/admin/workbench/redeem/generated/${id}`
  )
  return data
}

export async function deleteGeneratedBatch(ids: number[]): Promise<GeneratedRedeemCode[]> {
  const { data } = await apiClient.post<GeneratedRedeemCode[]>(
    '/admin/workbench/redeem/generated/batch-delete',
    { ids }
  )
  return data
}

export async function generateUserBalanceTransferCodes(
  payload: GenerateBalanceTransferCodeRequest
): Promise<GeneratedRedeemCode[]> {
  const { data } = await apiClient.post<GeneratedRedeemCode[] | GeneratedRedeemCode>(
    '/redeem/generate',
    payload
  )
  return Array.isArray(data) ? data : [data]
}

export async function getUserGenerated(
  params: RedeemListParams
): Promise<PaginatedResponse<GeneratedRedeemCode>> {
  const { data } = await apiClient.get<PaginatedResponse<GeneratedRedeemCode>>('/redeem/generated', {
    params
  })
  return data
}

export async function deleteUserGenerated(id: number): Promise<GeneratedRedeemCode> {
  const { data } = await apiClient.delete<GeneratedRedeemCode>(`/redeem/generated/${id}`)
  return data
}

export async function deleteUserGeneratedBatch(ids: number[]): Promise<GeneratedRedeemCode[]> {
  const { data } = await apiClient.post<GeneratedRedeemCode[]>('/redeem/generated/batch-delete', {
    ids
  })
  return data
}

export async function convertBalanceToRedeemCodes(value: number, count: number): Promise<{
  codes: RedeemCode[]
  total_value: number
  new_balance: number
}> {
  const { data } = await apiClient.post<{
    codes: RedeemCode[]
    total_value: number
    new_balance: number
  }>('/manager/redeem-codes/convert-balance', { value, count })
  return data
}

export const redeemAPI = {
  redeem,
  getHistory,
  generateBalanceTransferCode,
  generateBalanceTransferCodes,
  deleteGenerated,
  deleteGeneratedBatch,
  getGenerated,
  generateUserBalanceTransferCodes,
  getUserGenerated,
  deleteUserGenerated,
  deleteUserGeneratedBatch,
  convertBalanceToRedeemCodes
}

export default redeemAPI
