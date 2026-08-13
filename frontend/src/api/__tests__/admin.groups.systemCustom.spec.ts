import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, deleteRequest } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  deleteRequest: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    put,
    delete: deleteRequest
  }
}))

import {
  createSystemCustomGroup,
  deleteSystemCustomGroup,
  getSystemCustomGroup,
  getSystemCustomGroupCandidates,
  getSystemCustomGroupSyncPreview,
  updateSystemCustomGroup
} from '@/api/admin/groups'
import type {
  CreateSystemCustomGroupRequest,
  SystemCustomGroup,
  SystemCustomGroupCandidate,
  SystemCustomGroupDeleteResponse,
  SystemCustomGroupSyncPreview,
  UpdateSystemCustomGroupRequest
} from '@/types'

const explicitModels = [
  {
    public_model: 'claude-sonnet@tavern-a',
    source_group_id: 11,
    source_model: 'claude-sonnet',
    enabled: true
  },
  {
    public_model: 'claude-sonnet@tavern-b',
    source_group_id: 12,
    source_model: 'claude-sonnet',
    enabled: false
  }
]

describe('admin system custom group API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    deleteRequest.mockReset()
  })

  it('loads candidates from the dedicated endpoint and unwraps data', async () => {
    const candidates: SystemCustomGroupCandidate[] = [
      {
        group: {
          id: 11,
          name: 'Tavern A',
          platform: 'anthropic',
          status: 'active',
          subscription_type: 'standard'
        },
        models: ['claude-sonnet']
      }
    ]
    get.mockResolvedValueOnce({ data: candidates })

    await expect(getSystemCustomGroupCandidates()).resolves.toEqual(candidates)
    expect(get).toHaveBeenCalledWith('/admin/system-custom-groups/candidates')
  })

  it('creates a group without changing explicit aliases or enabled flags', async () => {
    const request: CreateSystemCustomGroupRequest = {
      name: 'Tavern monthly card',
      description: 'All tavern sources',
      daily_limit_usd: 20,
      weekly_limit_usd: null,
      monthly_limit_usd: 300,
      default_validity_days: 30,
      models: explicitModels
    }
    const created = { group: { id: 90 }, models: explicitModels } as SystemCustomGroup
    post.mockResolvedValueOnce({ data: created })

    await expect(createSystemCustomGroup(request)).resolves.toBe(created)
    expect(post).toHaveBeenCalledWith('/admin/system-custom-groups', request)
    expect(post.mock.calls[0][1].models).toEqual(explicitModels)
  })

  it('loads one system custom group and unwraps data', async () => {
    const group = { group: { id: 90 }, models: explicitModels } as SystemCustomGroup
    get.mockResolvedValueOnce({ data: group })

    await expect(getSystemCustomGroup(90)).resolves.toBe(group)
    expect(get).toHaveBeenCalledWith('/admin/system-custom-groups/90')
  })

  it('updates a complete route snapshot without changing explicit aliases or enabled flags', async () => {
    const request: UpdateSystemCustomGroupRequest = {
      name: 'Tavern monthly card',
      description: null,
      daily_limit_usd: null,
      weekly_limit_usd: null,
      monthly_limit_usd: 300,
      default_validity_days: 31,
      models: explicitModels
    }
    const updated = { group: { id: 90 }, models: explicitModels } as SystemCustomGroup
    put.mockResolvedValueOnce({ data: updated })

    await expect(updateSystemCustomGroup(90, request)).resolves.toBe(updated)
    expect(put).toHaveBeenCalledWith('/admin/system-custom-groups/90', request)
    expect(put.mock.calls[0][1].models).toEqual(explicitModels)
  })

  it('loads the sync preview and unwraps all route states', async () => {
    const preview: SystemCustomGroupSyncPreview = {
      added: [
        {
          public_model: 'claude-opus',
          source_group_id: 11,
          source_model: 'claude-opus',
          selected: false
        }
      ],
      missing: [],
      conflicting: [
        {
          public_model: 'claude-sonnet',
          source_group_id: 12,
          source_model: 'claude-sonnet',
          reason: 'duplicate public model'
        }
      ]
    }
    get.mockResolvedValueOnce({ data: preview })

    await expect(getSystemCustomGroupSyncPreview(90)).resolves.toBe(preview)
    expect(get).toHaveBeenCalledWith('/admin/system-custom-groups/90/sync-preview')
  })

  it('deletes through the dedicated endpoint and unwraps the deletion result', async () => {
    const deleted: SystemCustomGroupDeleteResponse = { id: 90, deleted: true }
    deleteRequest.mockResolvedValueOnce({ data: deleted })

    await expect(deleteSystemCustomGroup(90)).resolves.toEqual(deleted)
    expect(deleteRequest).toHaveBeenCalledWith('/admin/system-custom-groups/90')
  })
})
