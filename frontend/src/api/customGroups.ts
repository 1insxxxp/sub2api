import { apiClient } from './client'
import type { CustomGroupCandidate, UserCustomGroup, UserCustomGroupModel } from '@/types'

export type CustomGroupModelInput = Pick<UserCustomGroupModel, 'public_model' | 'source_group_id' | 'source_model'>

export interface DeleteCustomGroupResult {
  deleted: boolean
  unbound_api_key_count: number
}

export const customGroupsAPI = {
  async list(): Promise<UserCustomGroup[]> {
    const { data } = await apiClient.get<UserCustomGroup[]>('/custom-groups')
    return data
  },
  async candidates(): Promise<CustomGroupCandidate[]> {
    const { data } = await apiClient.get<CustomGroupCandidate[]>('/custom-groups/candidates')
    return data
  },
  async create(name: string, models: CustomGroupModelInput[]): Promise<UserCustomGroup> {
    const { data } = await apiClient.post<UserCustomGroup>('/custom-groups', { name, models })
    return data
  },
  async update(id: number, payload: { name?: string; status?: 'active' | 'disabled'; models?: CustomGroupModelInput[] }): Promise<UserCustomGroup> {
    const { data } = await apiClient.put<UserCustomGroup>(`/custom-groups/${id}`, payload)
    return data
  },
  async delete(id: number, force = false): Promise<DeleteCustomGroupResult> {
    const { data } = await apiClient.delete<DeleteCustomGroupResult>(`/custom-groups/${id}`, {
      params: force ? { force: true } : undefined,
    })
    return data
  }
}
