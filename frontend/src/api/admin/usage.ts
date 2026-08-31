/**
 * Admin Usage API endpoints
 * Handles admin-level usage logs and statistics retrieval
 */

import { apiClient } from '../client'
import type { AdminUsageLog, UsageQueryParams, PaginatedResponse, UsageRequestType } from '@/types'
import type { EndpointStat } from '@/types'

// ==================== Types ====================

export interface AdminUsageStatsResponse {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_tokens: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_cost: number
  total_actual_cost: number
  total_account_cost: number
  average_duration_ms: number
  endpoints?: EndpointStat[]
  upstream_endpoints?: EndpointStat[]
  endpoint_paths?: EndpointStat[]
}

export interface SimpleUser {
  id: number
  email: string
  deleted: boolean
}

export interface SimpleApiKey {
  id: number
  name: string
  user_id: number
}

export interface UsageCleanupFilters {
  start_time: string
  end_time: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
}

export interface UsageCleanupTask {
  id: number
  status: string
  filters: UsageCleanupFilters
  created_by: number
  deleted_rows: number
  error_message?: string | null
  canceled_by?: number | null
  canceled_at?: string | null
  started_at?: string | null
  finished_at?: string | null
  created_at: string
  updated_at: string
}

export interface CreateUsageCleanupTaskRequest {
  start_date: string
  end_date: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string | null
  request_type?: UsageRequestType | null
  stream?: boolean | null
  billing_type?: number | null
  timezone?: string
}

export interface AdminUsageQueryParams extends UsageQueryParams {
  user_id?: number
  exact_total?: boolean
  billing_mode?: string
  upstream_model_mismatch?: boolean
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  // 错误请求 tab 专属筛选(仅传给错误列表接口;共用同一 filters 对象)
  error_phase?: string | null
  error_category?: string | null
  status_code?: number | null
}

export interface EmptyResponseOutcomeEvidence {
  http_status: number
  upstream_status: number
  has_text: boolean
  has_tool_call: boolean
  has_reasoning: boolean
  has_media: boolean
  output_bytes: number
  event_count: number
  stream_completed: boolean
  finish_reason?: string
  disconnect_source: string
  upstream_error_kind: string
  collector_version: number
}

export interface AdminEmptyResponseClaim {
  id: number
  usage_log_id: number
  status: string
  reason_code: string
  estimated_refund: number
  refunded_amount: number
  user_id: number
  user_email: string
  api_key_id: number
  account_id: number
  account_name: string
  group_id: number | null
  group_name: string
  model: string
  user_reason: string
  admin_note: string
  rule_version: number
  compensation_source: 'automatic' | 'manual' | 'none'
  balance_refund: number
  subscription_refund: number
  api_key_quota_refund: number
  request_id: string
  usage_created_at: string
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_cost: number
  actual_cost: number
  compensated_cost: number
  billing_type: number
  request_type: string
  stream: boolean
  duration_ms?: number | null
  first_token_ms?: number | null
  inbound_endpoint: string
  upstream_endpoint: string
  reviewed_by?: number | null
  reviewed_at?: string | null
  compensated_at?: string | null
  evidence: EmptyResponseOutcomeEvidence
  created_at: string
  updated_at: string
}

export interface EmptyResponseClaimMetricDimension {
  id: number
  name: string
  charged_requests: number
  claims: number
  refund_amount: number
  empty_response_rate: number
}

export interface EmptyResponseClaimMetrics {
  total_charged_requests: number
  total_claims: number
  compensated_claims: number
  manual_review_claims: number
  rejected_claims: number
  total_refund_amount: number
  empty_response_rate: number
  by_group: EmptyResponseClaimMetricDimension[]
  by_account: EmptyResponseClaimMetricDimension[]
  by_model: EmptyResponseClaimMetricDimension[]
}

// ==================== API Functions ====================

/**
 * List all usage logs with optional filters (admin only)
 * @param params - Query parameters for filtering and pagination
 * @returns Paginated list of usage logs
 */
export async function list(
  params: AdminUsageQueryParams,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminUsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminUsageLog>>('/admin/usage', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Get usage statistics with optional filters (admin only)
 * @param params - Query parameters for filtering
 * @returns Usage statistics
 */
export async function getStats(params: {
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string
  request_type?: UsageRequestType
  stream?: boolean
  native_compaction_v2?: boolean | null
  upstream_model_mismatch?: boolean
  period?: string
  start_date?: string
  end_date?: string
  timezone?: string
  nocache?: number
}): Promise<AdminUsageStatsResponse> {
  const { data } = await apiClient.get<AdminUsageStatsResponse>('/admin/usage/stats', {
    params
  })
  return data
}

/**
 * Search users by email keyword (admin only)
 * @param keyword - Email keyword to search
 * @returns List of matching users (max 30)
 */
export async function searchUsers(keyword: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>('/admin/usage/search-users', {
    params: { q: keyword }
  })
  return data
}

/**
 * Search API keys by user ID and/or keyword (admin only)
 * @param userId - Optional user ID to filter by
 * @param keyword - Optional keyword to search in key name
 * @returns List of matching API keys (max 30)
 */
export async function searchApiKeys(userId?: number, keyword?: string): Promise<SimpleApiKey[]> {
  const params: Record<string, unknown> = {}
  if (userId !== undefined) {
    params.user_id = userId
  }
  if (keyword) {
    params.q = keyword
  }
  const { data } = await apiClient.get<SimpleApiKey[]>('/admin/usage/search-api-keys', {
    params
  })
  return data
}

/**
 * List usage cleanup tasks (admin only)
 * @param params - Query parameters for pagination
 * @returns Paginated list of cleanup tasks
 */
export async function listCleanupTasks(
  params: { page?: number; page_size?: number },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<UsageCleanupTask>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageCleanupTask>>('/admin/usage/cleanup-tasks', {
    params,
    signal: options?.signal
  })
  return data
}

/**
 * Create a usage cleanup task (admin only)
 * @param payload - Cleanup task parameters
 * @returns Created cleanup task
 */
export async function createCleanupTask(payload: CreateUsageCleanupTaskRequest): Promise<UsageCleanupTask> {
  const { data } = await apiClient.post<UsageCleanupTask>('/admin/usage/cleanup-tasks', payload)
  return data
}

/**
 * Cancel a usage cleanup task (admin only)
 * @param taskId - Task ID to cancel
 */
export async function cancelCleanupTask(taskId: number): Promise<{ id: number; status: string }> {
  const { data } = await apiClient.post<{ id: number; status: string }>(
    `/admin/usage/cleanup-tasks/${taskId}/cancel`
  )
  return data
}

export async function listClaims(params: {
  page?: number
  page_size?: number
  status?: string
  model?: string
  user_id?: number
  group_id?: number
  account_id?: number
  start_date?: string
  end_date?: string
}): Promise<PaginatedResponse<AdminEmptyResponseClaim>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminEmptyResponseClaim>>(
    '/admin/usage/empty-response-claims',
    { params }
  )
  return data
}

export async function getClaimMetrics(params: { start_date?: string; end_date?: string }): Promise<EmptyResponseClaimMetrics> {
  const { data } = await apiClient.get<EmptyResponseClaimMetrics>('/admin/usage/empty-response-claims/metrics', { params })
  return data
}

export async function approveClaim(id: number, payload: { note: string }): Promise<AdminEmptyResponseClaim> {
  const { data } = await apiClient.post<AdminEmptyResponseClaim>(`/admin/usage/empty-response-claims/${id}/approve`, payload)
  return data
}

export async function rejectClaim(id: number, payload: { note: string }): Promise<AdminEmptyResponseClaim> {
  const { data } = await apiClient.post<AdminEmptyResponseClaim>(`/admin/usage/empty-response-claims/${id}/reject`, payload)
  return data
}

export async function batchClaims(payload: { ids: number[]; action: 'approved' | 'rejected'; note: string }): Promise<{ succeeded: number[]; failed: Record<number, string>; claims: AdminEmptyResponseClaim[] }> {
  const { data } = await apiClient.post<{
    succeeded: number[]
    failed: Record<number, string>
    claims: AdminEmptyResponseClaim[]
  }>('/admin/usage/empty-response-claims/batch', payload)
  return data
}

export const adminUsageAPI = {
  list,
  getStats,
  searchUsers,
  searchApiKeys,
  listCleanupTasks,
  createCleanupTask,
  cancelCleanupTask,
  listClaims,
  getClaimMetrics,
  approveClaim,
  rejectClaim,
  batchClaims
}

export default adminUsageAPI
