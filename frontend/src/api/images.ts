import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

const IMAGE_STUDIO_REQUEST_TIMEOUT_MS = 300000

export type ImageStudioMode = 'generation' | 'edit'

export interface ImageStudioAspectRatio {
  ratio: string
  size: string
  billing_tier: string
}

export interface ImageStudioConfig {
  enabled: boolean
  allowed_models: string[]
  default_model: string
  aspect_ratios: ImageStudioAspectRatio[]
  max_reference_image_mb: number
  retention_days: number
  max_images_per_user: number
}

export interface ImageStudioModelOption {
  model: string
  label: string
  mapped_model?: string
  capabilities: string[]
}

export interface ImageStudioQualityOption {
  quality: string
  label: string
  billing_tier: string
  estimated_cost: number
}

export interface ImageStudioPricePreviewItem {
  ratio: string
  quality: string
  size: string
  billing_tier: string
  estimated_cost: number
}

export interface ImageStudioGroupOption {
  id: number
  name: string
  description?: string
  platform: string
  models: ImageStudioModelOption[]
  qualities: ImageStudioQualityOption[]
  prices: ImageStudioPricePreviewItem[]
}

export interface ImageStudioOptions {
  enabled: boolean
  default_group_id?: number | null
  default_model: string
  groups: ImageStudioGroupOption[]
}

export interface ImageStudioImage {
  id: number
  user_id: number
  mode: ImageStudioMode | string
  model: string
  prompt: string
  aspect_ratio: string
  size: string
  image_url: string
  storage_driver: string
  storage_object_key: string
  mime_type: string
  output_format?: string
  background?: string
  bytes: number
  cost: number
  usage_log_id?: number | null
  source_image_count: number
  expires_at?: string | null
  deleted_at?: string | null
  created_at: string
  updated_at: string
}

export type ImageStudioTaskStatus = 'queued' | 'running' | 'succeeded' | 'failed'

export interface ImageStudioTask {
  id: number
  user_id: number
  api_key_id?: number | null
  group_id?: number | null
  image_id?: number | null
  image?: ImageStudioImage | null
  mode: ImageStudioMode | string
  status: ImageStudioTaskStatus | string
  model: string
  prompt: string
  aspect_ratio: string
  quality: string
  output_format?: string
  background?: string
  size: string
  estimated_cost: number
  source_image_count: number
  reference_object_keys?: string[]
  error_reason?: string
  error_message?: string
  started_at?: string | null
  completed_at?: string | null
  created_at: string
  updated_at: string
}

export interface ImageStudioGeneratePayload {
  api_key_id?: number | null
  group_id?: number | null
  model: string
  prompt: string
  aspect_ratio: string
  quality?: string
  output_format?: string
  background?: string
}

export interface ImageStudioTaskPayload extends ImageStudioGeneratePayload {
  mode: ImageStudioMode
}

export interface ImageStudioEditPayload extends ImageStudioGeneratePayload {
  images: File[]
}

export interface ImageStudioListParams {
  page?: number
  page_size?: number
}

export async function getConfig(): Promise<ImageStudioConfig> {
  const { data } = await apiClient.get<ImageStudioConfig>('/user/images/config')
  return data
}

export async function getOptions(): Promise<ImageStudioOptions> {
  const { data } = await apiClient.get<ImageStudioOptions>('/user/images/options')
  return data
}

export async function generate(payload: ImageStudioGeneratePayload): Promise<ImageStudioImage> {
  const { data } = await apiClient.post<ImageStudioImage>('/user/images/generate', payload, {
    timeout: IMAGE_STUDIO_REQUEST_TIMEOUT_MS,
  })
  return data
}

export async function createTask(payload: ImageStudioTaskPayload): Promise<ImageStudioTask> {
  const { data } = await apiClient.post<ImageStudioTask>('/user/images/tasks', payload)
  return data
}

export async function getTask(id: number): Promise<ImageStudioTask> {
  const { data } = await apiClient.get<ImageStudioTask>(`/user/images/tasks/${id}`)
  return data
}

export async function listTasks(
  params: ImageStudioListParams = {},
): Promise<PaginatedResponse<ImageStudioTask>> {
  const { data } = await apiClient.get<PaginatedResponse<ImageStudioTask>>('/user/images/tasks', {
    params,
  })
  return data
}

export async function edit(payload: ImageStudioEditPayload): Promise<ImageStudioImage> {
  const formData = new FormData()
  if (payload.api_key_id != null) {
    formData.append('api_key_id', String(payload.api_key_id))
  }
  if (payload.group_id != null) {
    formData.append('group_id', String(payload.group_id))
  }
  formData.append('model', payload.model)
  formData.append('prompt', payload.prompt)
  formData.append('aspect_ratio', payload.aspect_ratio)
  if (payload.quality) {
    formData.append('quality', payload.quality)
  }
  if (payload.output_format) {
    formData.append('output_format', payload.output_format)
  }
  if (payload.background) {
    formData.append('background', payload.background)
  }
  for (const image of payload.images) {
    formData.append('image', image)
  }

  const { data } = await apiClient.post<ImageStudioImage>('/user/images/edit', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: IMAGE_STUDIO_REQUEST_TIMEOUT_MS,
  })
  return data
}

export async function list(
  params: ImageStudioListParams = {},
): Promise<PaginatedResponse<ImageStudioImage>> {
  const { data } = await apiClient.get<PaginatedResponse<ImageStudioImage>>('/user/images', {
    params,
  })
  return data
}

export async function deleteImage(id: number): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/user/images/${id}`)
  return data
}

export const imageStudioAPI = {
  getConfig,
  getOptions,
  generate,
  createTask,
  getTask,
  listTasks,
  edit,
  list,
  delete: deleteImage,
}

export default imageStudioAPI
