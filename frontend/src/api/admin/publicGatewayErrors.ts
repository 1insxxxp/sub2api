import { apiClient } from '../client'

export interface PublicGatewayErrorOutput {
  status: number
  code: string
  message: string
}

export interface PublicGatewayErrorRule extends PublicGatewayErrorOutput {
  id: string
  name: string
  enabled: boolean
  priority: number
  upstream_statuses: number[]
  pattern: string
}

export interface PublicGatewayErrorSettings {
  rules: PublicGatewayErrorRule[]
  fallback: PublicGatewayErrorOutput
  bad_request_fallback: PublicGatewayErrorOutput
}

export type PublicGatewayErrorFormat = 'http' | 'sse' | 'responses' | 'gemini'

export interface PublicGatewayErrorPreviewRequest {
  settings: PublicGatewayErrorSettings
  status: number
  diagnostic: string
  format: PublicGatewayErrorFormat
}

export interface PublicGatewayErrorPreviewResult extends PublicGatewayErrorOutput {
  rule_id?: string
  rule_name?: string
  payload: Record<string, unknown>
}

export async function getSettings(): Promise<PublicGatewayErrorSettings> {
  const { data } = await apiClient.get<PublicGatewayErrorSettings>('/admin/settings/public-gateway-errors')
  return data
}

export async function updateSettings(settings: PublicGatewayErrorSettings): Promise<PublicGatewayErrorSettings> {
  const { data } = await apiClient.put<PublicGatewayErrorSettings>('/admin/settings/public-gateway-errors', settings)
  return data
}

export async function preview(request: PublicGatewayErrorPreviewRequest): Promise<PublicGatewayErrorPreviewResult> {
  const { data } = await apiClient.post<PublicGatewayErrorPreviewResult>('/admin/settings/public-gateway-errors/preview', request)
  return data
}

export const publicGatewayErrorsAPI = { getSettings, updateSettings, preview }
export default publicGatewayErrorsAPI
