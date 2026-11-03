import { apiClient } from '../client'
import type { AdminGroup } from '@/types'
import type { CategoryDefaultCategory } from '@/utils/categoryDefaultGroups'

export interface CategoryDefaultModel {
  public_model: string
  source_group_id: number
  source_model: string
  fingerprint?: string
}

export interface CategoryDefaultConfig {
  group_id: number
  category: CategoryDefaultCategory
  enabled: boolean
  revision: number
  source_group_ids: number[]
  models: CategoryDefaultModel[]
  models_count: number
}

export interface CategoryDefaultCandidate {
  group: AdminGroup
  models: string[]
  models_count: number
  invalid_reason?: string
}

export interface CategoryDefaultConflict {
  public_model: string
  source_group_ids: number[]
  source_group_names?: string[]
}

export interface CategoryDefaultPreview {
  revision: number
  preview_hash: string
  added: CategoryDefaultModel[]
  modified: CategoryDefaultModel[]
  removed: CategoryDefaultModel[]
  unchanged: CategoryDefaultModel[]
  conflicts: CategoryDefaultConflict[]
  models_count: number
}

const base = '/admin/category-default-groups'
export const categoryDefaultGroupsAPI = {
  async list(): Promise<CategoryDefaultConfig[]> {
    return (await apiClient.get<CategoryDefaultConfig[]>(base)).data
  },
  async candidates(category: CategoryDefaultCategory): Promise<CategoryDefaultCandidate[]> {
    return (await apiClient.get<CategoryDefaultCandidate[]>(`${base}/${category}/candidates`)).data
  },
  async preview(category: CategoryDefaultCategory, sourceGroupIds: number[]): Promise<CategoryDefaultPreview> {
    return (await apiClient.post<CategoryDefaultPreview>(`${base}/${category}/sync-preview`, { source_group_ids: sourceGroupIds })).data
  },
  async apply(category: CategoryDefaultCategory, sourceGroupIds: number[], preview: Pick<CategoryDefaultPreview, 'revision' | 'preview_hash'>): Promise<CategoryDefaultConfig> {
    return (await apiClient.post<CategoryDefaultConfig>(`${base}/${category}/sync-apply`, {
      source_group_ids: sourceGroupIds,
      expected_revision: preview.revision,
      preview_hash: preview.preview_hash,
    })).data
  },
  async setStatus(category: CategoryDefaultCategory, enabled: boolean): Promise<CategoryDefaultConfig> {
    return (await apiClient.put<CategoryDefaultConfig>(`${base}/${category}/status`, { enabled })).data
  },
}
