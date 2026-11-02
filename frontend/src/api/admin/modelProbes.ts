import { apiClient } from '../client'

export interface ProbeGroup { id: number; name: string; platform: string }
export interface ProbeModel { id: string; display_name?: string }
export interface ProbeConfigGroup extends ProbeGroup { enabled: boolean }
export interface ProbeGroupChange { group_id: number; enabled: boolean }
export type ProbeStatus = 'success' | 'failed' | 'timeout' | 'cancelled' | 'unsupported'
export interface ModelProbeResult {
  group_id: number
  model: string
  status: ProbeStatus
  duration_ms: number
  started_at: string
  message?: string
  error_code?: string
}
export async function listGroups(signal?: AbortSignal): Promise<{ items: ProbeGroup[] }> {
  const { data } = await apiClient.get<{ items: ProbeGroup[] }>('/admin/model-probes/groups', { signal })
  return data
}
export async function listModels(groupId: number, signal?: AbortSignal): Promise<{ group: ProbeGroup; items: ProbeModel[] }> {
  const { data } = await apiClient.get<{ group: ProbeGroup; items: ProbeModel[] }>(`/admin/model-probes/groups/${groupId}/models`, { signal })
  return data
}
export async function runProbe(groupId: number, model: string, signal?: AbortSignal): Promise<ModelProbeResult> {
  const { data } = await apiClient.post<ModelProbeResult>('/admin/model-probes/run', { group_id: groupId, model }, { signal, timeout: 180000 })
  return data
}
export async function listConfigGroups(signal?: AbortSignal): Promise<{ items: ProbeConfigGroup[] }> {
  const { data } = await apiClient.get<{ items: ProbeConfigGroup[] }>('/admin/model-probes/config/groups', { signal })
  return data
}
export async function saveConfigGroups(groups: ProbeGroupChange[], signal?: AbortSignal): Promise<{ items: ProbeGroupChange[] }> {
  const { data } = await apiClient.put<{ items: ProbeGroupChange[] }>('/admin/model-probes/config/groups', { groups }, { signal })
  return data
}
