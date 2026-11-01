import { apiClient } from '../client'

export interface UserSupportAccess { enabled: boolean }
export interface WorkbenchPermissions { commission: boolean; affiliate_leaderboard: boolean; user_support: boolean }
export interface WorkbenchAccess extends WorkbenchPermissions { balance_transfer: boolean }
export type SupportRequestStatus = 'completed' | 'interrupted' | 'failed' | 'empty' | 'unknown'
export type SupportErrorCategory = 'auth' | 'quota' | 'rate_limit' | 'invalid_request' | 'timeout' | 'service_unavailable' | 'other'

export interface SupportUsageRecord {
  id: number
  created_at: string
  model: string
  group_name: string
  input_tokens: number | null
  output_tokens: number | null
  cache_read_tokens: number | null
  charged_amount: number
  duration_ms: number | null
  first_token_ms: number | null
  status: SupportRequestStatus
  stream: boolean
}

export interface SupportErrorRecord {
  id: number
  created_at: string
  model: string
  category: SupportErrorCategory
}

export interface UserSupportResult {
  user: { id: number; email: string; balance: number }
  trial: { status: 'granted' | 'already_received'; amount: number; granted_at: string; legacy: boolean }
  usage: { status: 'ok' | 'unavailable'; items: SupportUsageRecord[] }
  errors: { status: 'ok' | 'unavailable'; items: SupportErrorRecord[] }
}

export async function getAccess(): Promise<UserSupportAccess> {
  const { data } = await apiClient.get<UserSupportAccess>('/admin/user-support/access')
  return data
}

export async function getWorkbenchAccess(): Promise<WorkbenchAccess> {
  const { data } = await apiClient.get<WorkbenchAccess>('/admin/workbench/access')
  return data
}

export async function getUserWorkbenchAccess(userID: number): Promise<WorkbenchPermissions> {
  const { data } = await apiClient.get<WorkbenchPermissions>(`/admin/users/${userID}/workbench-access`)
  return data
}

export async function setUserWorkbenchAccess(userID: number, permissions: WorkbenchPermissions): Promise<WorkbenchPermissions> {
  const { data } = await apiClient.put<WorkbenchPermissions>(`/admin/users/${userID}/workbench-access`, permissions)
  return data
}

export async function lookup(account: string): Promise<UserSupportResult> {
  const { data } = await apiClient.post<UserSupportResult>('/admin/user-support/lookup', { account })
  return data
}

export async function getUserAccess(userID: number): Promise<UserSupportAccess> {
  const { data } = await apiClient.get<UserSupportAccess>(`/admin/users/${userID}/support-access`)
  return data
}

export async function setUserAccess(userID: number, enabled: boolean): Promise<UserSupportAccess> {
  const { data } = await apiClient.put<UserSupportAccess>(`/admin/users/${userID}/support-access`, { enabled })
  return data
}
