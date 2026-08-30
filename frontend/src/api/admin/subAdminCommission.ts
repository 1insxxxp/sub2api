import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface SubAdminCommissionSettings {
  commission_rate: number
}

export interface UpdateSubAdminCommissionSettingsRequest {
  commission_rate: number
}

export interface SubAdminCommissionGrant {
  id: number
  sub_admin_id: number
  sub_admin_email?: string
  group_id: number
  group_name: string
  granted_date: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface ReplaceSubAdminCommissionGrantsRequest {
  group_ids: number[]
}

export interface SubAdminCommissionCalendarDay {
  date: string
  enabled: boolean
  actual_cost: number
  commission_amount: number
}

export interface SubAdminCommissionDayGroup {
  group_id: number
  group_name: string
  requests: number
  total_tokens: number
  actual_cost: number
  commission_amount: number
}

export interface SubAdminCommissionUsageLog {
  id: number
  request_id: string
  created_at: string
  user_id: number
  user_email: string
  api_key_id: number
  api_key_name: string
  group_id: number
  group_name: string
  model: string
  requested_model?: string
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  actual_cost: number
  total_tokens: number
}

export interface WorkbenchCommissionCalendarQuery {
  month?: string
}

export interface WorkbenchCommissionLogsQuery {
  page?: number
  page_size?: number
}

export async function getSettings(): Promise<SubAdminCommissionSettings> {
  const { data } = await apiClient.get<SubAdminCommissionSettings>(
    '/admin/sub-admin-commissions/settings'
  )
  return data
}

export async function updateSettings(
  payload: UpdateSubAdminCommissionSettingsRequest
): Promise<SubAdminCommissionSettings> {
  const { data } = await apiClient.put<SubAdminCommissionSettings>(
    '/admin/sub-admin-commissions/settings',
    payload
  )
  return data
}

export async function listGrants(): Promise<SubAdminCommissionGrant[]> {
  const { data } = await apiClient.get<SubAdminCommissionGrant[]>(
    '/admin/sub-admin-commissions/grants'
  )
  return data
}

export async function replaceGrants(
  payload: ReplaceSubAdminCommissionGrantsRequest
): Promise<SubAdminCommissionGrant[]> {
  const { data } = await apiClient.put<SubAdminCommissionGrant[]>(
    '/admin/sub-admin-commissions/grants',
    payload
  )
  return data
}

export async function getWorkbenchGrants(): Promise<SubAdminCommissionGrant[]> {
  const { data } = await apiClient.get<SubAdminCommissionGrant[]>(
    '/admin/workbench/commission/grants'
  )
  return data
}

export async function getWorkbenchCalendar(
  query: WorkbenchCommissionCalendarQuery = {}
): Promise<SubAdminCommissionCalendarDay[]> {
  const { data } = await apiClient.get<SubAdminCommissionCalendarDay[]>(
    '/admin/workbench/commission/calendar',
    { params: query }
  )
  return data
}

export async function getWorkbenchDayGroups(
  date: string
): Promise<SubAdminCommissionDayGroup[]> {
  const { data } = await apiClient.get<SubAdminCommissionDayGroup[]>(
    `/admin/workbench/commission/days/${date}/groups`
  )
  return data
}

export async function getWorkbenchDayGroupLogs(
  date: string,
  groupID: number,
  query: WorkbenchCommissionLogsQuery = {}
): Promise<PaginatedResponse<SubAdminCommissionUsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<SubAdminCommissionUsageLog>>(
    `/admin/workbench/commission/days/${date}/groups/${groupID}/logs`,
    { params: query }
  )
  return data
}

export const subAdminCommissionAPI = {
  getSettings,
  updateSettings,
  listGrants,
  replaceGrants,
  getWorkbenchGrants,
  getWorkbenchCalendar,
  getWorkbenchDayGroups,
  getWorkbenchDayGroupLogs,
}

export default subAdminCommissionAPI
