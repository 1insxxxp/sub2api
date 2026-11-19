import { apiClient } from '../client'

export type AcceptanceExtra = 'long_context' | 'image' | 'audio' | 'tools'
export interface AcceptancePrice {
  input_per_million: number
  output_per_million: number
  cache_per_million: number
  source: string
}
export interface AcceptanceModel {
  unavailable_reason?: string
  model: string
  upstream_model: string
  protocols: string[]
  price?: AcceptancePrice | null
}
export interface AcceptanceTarget {
  unavailable_reason?: string
  account_id: number
  name: string
  platform: string
  models: AcceptanceModel[]
  groups?: { id: number; name: string }[]
}
export interface AcceptanceTargetRequest {
  account_id: number
  model: string
  group_id?: number
  price?: AcceptancePrice
}
export interface AcceptancePreviewRequest {
  targets: AcceptanceTargetRequest[]
  extras: AcceptanceExtra[]
  budget_usd?: number
}
export interface AcceptanceSnapshot {
  account_id: number
  account_name: string
  platform: string
  model: string
  upstream_model: string
  group_id?: number
  fingerprint: string
  price: AcceptancePrice
}
export interface AcceptanceUsage {
  cache_create?: number | null
  output_includes_thoughts?: boolean
  input: number | null
  output: number | null
  thoughts: number | null
  cache_read: number | null
  total: number | null
  source: string
}
export interface AcceptanceSource {
  kind: 'direct' | 'declared' | 'unverified'
  channel: string
  host: string
  path: string
  auth_kind: string
  adapter: string
  requested_model: string
  upstream_model: string
  response_model?: string
}
export interface AcceptanceResult {
  status: string
  source: AcceptanceSource
  raw_usage: AcceptanceUsage
  client_usage: AcceptanceUsage
  billing_usage: AcceptanceUsage
  findings: { code: string; level: string; message: string }[]
  duration_ms: number
  first_token_ms?: number | null
  http_status: number
  finish_reason: string
  completed: boolean
  upstream_usage_final?: boolean
  has_text: boolean
  has_reasoning: boolean
  has_tool: boolean
  output_excerpt: string
  usage_frames?: unknown[]
  effective_parameters?: Record<string, unknown>
  cost_usd: number | null
}
export interface AcceptanceSample {
  id: string
  run_id?: string
  ordinal: number
  target: AcceptanceSnapshot
  category: string
  protocol: string
  stream: boolean
  prompt: string
  expected?: string
  input_size: number
  max_output_tokens: number
  timeout_seconds: number
  estimated_cost_usd: number
  status: string
  result?: AcceptanceResult | null
}
export interface AcceptancePreview {
  id: string
  rule_version: string
  created_at: string
  expires_at: string
  request: AcceptancePreviewRequest
  samples: AcceptanceSample[]
  generation_calls: number
  count_calls: number
  estimated_cost_usd: number
  can_start: boolean
  issues: string[]
}
export interface AcceptanceRun {
  id: string
  preview_id: string
  actor_id: number
  rule_version: string
  status: string
  created_at: string
  updated_at: string
  budget_usd: number | null
  estimated_cost_usd: number
  reserved_cost_usd: number
  spent_cost_usd: number
  total: number
  completed: number
  stale_config: boolean
  samples?: AcceptanceSample[]
}
const base = '/admin/channel-acceptance'
export const channelAcceptanceApi = {
  async targets(accountIds: number[]): Promise<AcceptanceTarget[]> {
    const { data } = await apiClient.get<{ items: AcceptanceTarget[] }>(`${base}/targets`, { params: { account_ids: accountIds.join(',') } })
    return data.items ?? []
  },
  async preview(request: AcceptancePreviewRequest): Promise<AcceptancePreview> {
    const { data } = await apiClient.post<AcceptancePreview>(`${base}/previews`, request)
    return data
  },
  async start(previewId: string): Promise<AcceptanceRun> {
    const { data } = await apiClient.post<AcceptanceRun>(`${base}/runs`, { preview_id: previewId })
    return data
  },
  async list(accountIds: number[]): Promise<AcceptanceRun[]> {
    const { data } = await apiClient.get<{ items: AcceptanceRun[] }>(`${base}/runs`, { params: { account_ids: accountIds.join(',') } })
    return data.items ?? []
  },
  async get(id: string): Promise<AcceptanceRun> {
    const { data } = await apiClient.get<AcceptanceRun>(`${base}/runs/${encodeURIComponent(id)}`)
    return data
  },
  async cancel(id: string): Promise<AcceptanceRun> {
    const { data } = await apiClient.post<AcceptanceRun>(`${base}/runs/${encodeURIComponent(id)}/cancel`)
    return data
  },
  async exportReport(id: string): Promise<Blob> {
    const { data } = await apiClient.get<Blob>(`${base}/runs/${encodeURIComponent(id)}/export`, { responseType: 'blob' })
    return data
  }
}
