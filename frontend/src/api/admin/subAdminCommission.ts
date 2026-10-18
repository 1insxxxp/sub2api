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

export interface SubAdminCommissionExclusion {
  id: number
  user_id: number
  user_email: string
  username: string
  started_at: string
}

export interface CommissionExclusionsQuery {
  page?: number
  page_size?: number
}

export interface WorkbenchCommissionCalendarQuery {
  month?: string
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

export async function listExclusions(
  query: CommissionExclusionsQuery = {}
): Promise<PaginatedResponse<SubAdminCommissionExclusion>> {
  const { data } = await apiClient.get<PaginatedResponse<SubAdminCommissionExclusion>>(
    '/admin/sub-admin-commissions/exclusions',
    { params: query }
  )
  return data
}

export async function addExclusion(
  payload: { user_id: number }
): Promise<SubAdminCommissionExclusion> {
  const { data } = await apiClient.post<SubAdminCommissionExclusion>(
    '/admin/sub-admin-commissions/exclusions',
    payload
  )
  return data
}

export async function removeExclusion(userID: number): Promise<void> {
  await apiClient.delete(`/admin/sub-admin-commissions/exclusions/${userID}`)
}

export const subAdminCommissionAPI = {
  getSettings,
  updateSettings,
  listGrants,
  replaceGrants,
  getWorkbenchGrants,
  getWorkbenchCalendar,
  getWorkbenchDayGroups,
  listExclusions,
  addExclusion,
  removeExclusion,
}

export default subAdminCommissionAPI
