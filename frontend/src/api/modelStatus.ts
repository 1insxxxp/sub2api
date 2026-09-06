import axios from 'axios'
import type { ApiResponse } from '@/types'
import { getAPIBaseURL } from './url'

export type ModelStatusOutcome = 'success' | 'failure' | 'empty' | 'unknown'
export type ModelStatusHealth = 'healthy' | 'degraded' | 'unavailable' | 'insufficient_data' | 'no_data' | 'unknown'

export interface ModelStatusMetrics {
  total: number
  success: number
  failure: number
  empty: number
  unknown: number
  success_rate: number | null
  avg_ttft_ms: number | null
  avg_duration_ms: number | null
  ttft_samples: number
  duration_samples: number
}

export interface ModelStatusModel {
  name: string
  platform: string
  status: ModelStatusHealth
  metrics: ModelStatusMetrics
  buckets?: ModelStatusBucket[]
  /** @deprecated kept for older cached responses during rollout */
  recent?: Array<{ at: string; outcome: ModelStatusOutcome; status_code?: number }>
}

export interface ModelStatusBucket {
  start_at: string
  end_at: string
  total: number
  success: number
  failure: number
  empty: number
  unknown: number
  requests: Array<{ at: string; outcome: ModelStatusOutcome; status_code?: number }>
}

export interface ModelStatusGroup {
  id: number
  name: string
  platform: string
  metrics: ModelStatusMetrics
  models: ModelStatusModel[]
}

export interface ModelStatusResponse {
  generated_at: string
  snapshot_at?: string
  bucket_count?: number
  bucket_interval_minutes?: number
  refresh_interval_seconds: 30
  coverage: { status: 'partial'; terminal_errors_enabled: boolean; reasons: string[] }
  summary: ModelStatusMetrics
  groups: ModelStatusGroup[]
}

// Public statistics must not refresh, clear, or redirect a user's auth session.
const publicClient = axios.create({ baseURL: getAPIBaseURL(), timeout: 20000, withCredentials: false })

export async function getModelStatus(options?: { signal?: AbortSignal }): Promise<ModelStatusResponse> {
  const { data } = await publicClient.get<ApiResponse<ModelStatusResponse>>('/model-status', {
    signal: options?.signal,
  })
  if (data.code !== 0 || !data.data) throw new Error('Model status is unavailable')
  return data.data
}
