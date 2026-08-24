import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export type LotteryPrizeType = 'balance' | 'product'
export type LotteryAttemptMode = 'daily' | 'total'

export interface LotteryActivity {
  id: number
  name: string
  description: string
  status: string
  attempt_mode: LotteryAttemptMode
  attempt_limit: number
  starts_at?: string | null
  ends_at?: string | null
}

export interface LotteryPrize {
  id: number
  activity_id: number
  name: string
  description: string
  type: LotteryPrizeType
  weight: number
  balance_amount?: number | null
  enabled: boolean
  sort_order: number
  available_item_count: number
}

export interface LotteryPublicState {
  activity: LotteryActivity
  prizes: LotteryPrize[]
  attempts_used: number
  attempts_remaining: number
}

export interface LotteryDraw {
  id: number
  prize_name: string
  prize_type: LotteryPrizeType
  balance_amount?: number | null
  product_content?: string | null
  created_at: string
}

export interface LotteryDrawResult {
  draw: LotteryDraw
  attempts_used: number
  attempts_remaining: number
}

export interface LotteryHistoryQuery {
  page?: number
  page_size?: number
}

export async function getLotteryState(): Promise<LotteryPublicState> {
  const { data } = await apiClient.get<LotteryPublicState>('/lottery/state')
  return data
}

export async function drawLottery(attemptKey: string): Promise<LotteryDrawResult> {
  const { data } = await apiClient.post<LotteryDrawResult>('/lottery/draw', { attempt_key: attemptKey })
  return data
}

export async function getLotteryHistory(params: LotteryHistoryQuery = {}): Promise<PaginatedResponse<LotteryDraw>> {
  const { data } = await apiClient.get<PaginatedResponse<LotteryDraw>>('/lottery/history', { params })
  return data
}

export const lotteryAPI = { getState: getLotteryState, draw: drawLottery, history: getLotteryHistory }

export default lotteryAPI
