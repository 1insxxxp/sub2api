import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

const root = '/admin/checkins/risk'
export const riskReasons = [
  'browser', 'registration_ip', 'login_ip', 'usage_ip', 'payment_ip', 'checkin_ip', 'invitation',
] as const
export type RiskReason = typeof riskReasons[number]
export type ReviewStatus = 'observed' | 'pending' | 'approved' | 'rejected'
export type RiskMode = 'observe' | 'enforce'

export interface RiskSettings {
  checkin_mode: RiskMode
  registration_mode: RiskMode
  window_minutes: number
  challenge_count: number
  cooldown_count: number
  cooldown_minutes: number
}
export interface RiskConfig extends RiskSettings {
  registration_hits_today?: { challenge: number; cooldown: number }
}
export interface RiskReview {
  user_id: number
  user_email: string
  owner_email: string
  browser_hash: string
  checkin_date: string
  status: ReviewStatus
  created_at: string
}
export interface RiskAssociation {
  user_email: string
  related_email: string
  inviter_email: string
  user_agent: string
  related_user_agent: string
  reasons: RiskReason[]
  evidence: { reason: RiskReason; first_seen_at: string; last_seen_at: string }[]
  first_seen_at: string
  last_seen_at: string
  high_risk: boolean
  checkin_count: number
  reward_amount: number
  related_checkin_count: number
  related_reward_amount: number
  status: ReviewStatus | 'none'
}
export interface RiskWhitelistEntry {
  email: string
  reason: string
  created_at: string
}

interface PageQuery {
  page: number
  page_size: number
  search?: string
}
export interface AssociationQuery extends PageQuery {
  reason?: RiskReason
  status?: ReviewStatus | 'none'
  start_date?: string
  end_date?: string
}
export interface ReviewQuery extends PageQuery {
  date?: string
  status?: ReviewStatus
}
type Decision = Pick<RiskReview, 'user_id' | 'browser_hash' | 'checkin_date'> & {
  decision: 'approved' | 'rejected'
  reason: string
}

export const checkinRisk = {
  async reviews(params: ReviewQuery) {
    return (await apiClient.get<PaginatedResponse<RiskReview>>(root + '/reviews', { params })).data
  },
  async associations(params: AssociationQuery) {
    return (await apiClient.get<PaginatedResponse<RiskAssociation>>(root + '/associations', { params })).data
  },
  async whitelist(params: PageQuery) {
    return (await apiClient.get<PaginatedResponse<RiskWhitelistEntry>>(root + '/whitelist', { params })).data
  },
  async decide(input: Decision) {
    await apiClient.post(root + '/reviews/decision', input)
  },
  async changeWhitelist(input: { email: string; enabled: boolean; reason: string }) {
    await apiClient.put(root + '/whitelist', input)
  },
  async config() {
    return (await apiClient.get<RiskConfig>(root + '/config')).data
  },
  async saveConfig(input: RiskSettings) {
    return (await apiClient.put<RiskConfig>(root + '/config', input)).data
  },
}
