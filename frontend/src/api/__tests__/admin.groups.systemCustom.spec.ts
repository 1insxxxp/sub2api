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
  previewPricingCoverage,
  updateSystemCustomGroup
} from '@/api/admin/groups'
import type {
  CreateSystemCustomGroupRequest,
  SystemCustomGroup,
  SystemCustomGroupApiError,
  SystemCustomGroupCandidate,
  SystemCustomGroupDeleteResponse,
  SystemCustomGroupSyncPreview,
  UpdateSystemCustomGroupRequest
} from '@/types'

const systemCustomGroup = {
  group: {
    id: 90,
    name: 'Tavern monthly card',
    description: '',
    platform: 'composite',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'subscription',
    system_custom_routing_enabled: true,
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: 300,
    default_validity_days: 30,
    created_at: '2026-08-13T00:00:00Z',
    updated_at: '2026-08-13T00:00:00Z'
  },
  sources: [
    {
      id: 901,
      group_id: 90,
      source_group_id: 11,
      priority: 0,
      group: {
        id: 11,
        name: 'Tavern A',
        description: '',
        platform: 'anthropic',
        status: 'active',
        subscription_type: 'standard'
      },
      created_at: '2026-08-13T00:00:00Z',
      updated_at: '2026-08-13T00:00:00Z'
    },
    {
      id: 902,
      group_id: 90,
      source_group_id: 12,
      priority: 1,
      created_at: '2026-08-13T00:00:00Z',
      updated_at: '2026-08-13T00:00:00Z'
    }
  ],
  summary: {
    unique_models: 2,
    fallback_routes: 0,
    unavailable_sources: 0,
    unpriced_routes: 0
  },
  models: []
} satisfies SystemCustomGroup

describe('admin system custom group API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    deleteRequest.mockReset()
  })

  it('models structured API errors and network errors without assuming status or code', () => {
    const validationError = {
      status: 400,
      code: 'SYSTEM_CUSTOM_GROUP_DUPLICATE_PUBLIC_MODEL',
      message: 'system custom group public model already exists',
      reason: 'SYSTEM_CUSTOM_GROUP_DUPLICATE_PUBLIC_MODEL',
      metadata: {
        public_model: 'claude-sonnet',
        source_group_id: '12',
        source_model: 'claude-sonnet'
      }
    } satisfies SystemCustomGroupApiError
    const networkError = {
      message: 'Network Error'
    } satisfies SystemCustomGroupApiError

    expect(validationError.code).toBe('SYSTEM_CUSTOM_GROUP_DUPLICATE_PUBLIC_MODEL')
    expect('status' in networkError).toBe(false)
    expect('code' in networkError).toBe(false)
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

  it('previews prospective group pricing without saving the form', async () => {
    const request = {
      group_id: 90,
      platform: 'anthropic',
      models: ['claude-sonnet-4'],
      model_pricing: []
    }
    const coverage = {
      models: [{ model: 'claude-sonnet-4', status: 'missing' as const }]
    }
    post.mockResolvedValueOnce({ data: coverage })

    await expect(previewPricingCoverage(request)).resolves.toEqual(coverage)
    expect(post).toHaveBeenCalledWith('/admin/groups/pricing-coverage', request)
  })

  it('creates a group with ordered source groups', async () => {
    const request: CreateSystemCustomGroupRequest = {
      name: 'Tavern monthly card',
      description: 'All tavern sources',
      daily_limit_usd: 20,
      weekly_limit_usd: null,
      monthly_limit_usd: 300,
      default_validity_days: 30,
      source_group_ids: [11, 12]
    }
    const created: SystemCustomGroup = systemCustomGroup
    post.mockResolvedValueOnce({ data: created })

    await expect(createSystemCustomGroup(request)).resolves.toBe(created)
    expect(post).toHaveBeenCalledWith('/admin/system-custom-groups', request)
    expect(post.mock.calls[0][1].source_group_ids).toEqual([11, 12])
  })

  it('loads one system custom group and unwraps data', async () => {
    const group: SystemCustomGroup = systemCustomGroup
    get.mockResolvedValueOnce({ data: group })

    await expect(getSystemCustomGroup(90)).resolves.toBe(group)
    expect(get).toHaveBeenCalledWith('/admin/system-custom-groups/90')
  })

  it('updates ordered source groups', async () => {
    const request: UpdateSystemCustomGroupRequest = {
      name: 'Tavern monthly card',
      description: null,
      daily_limit_usd: null,
      weekly_limit_usd: null,
      monthly_limit_usd: 300,
      default_validity_days: 31,
      source_group_ids: [12, 11]
    }
    const updated: SystemCustomGroup = {
      ...systemCustomGroup,
      group: {
        ...systemCustomGroup.group,
        description: 'Updated routes',
        daily_limit_usd: 20,
        weekly_limit_usd: 100,
        default_validity_days: 31,
        updated_at: '2026-08-13T01:00:00Z'
      }
    }
    put.mockResolvedValueOnce({ data: updated })

    await expect(updateSystemCustomGroup(90, request)).resolves.toBe(updated)
    expect(put).toHaveBeenCalledWith('/admin/system-custom-groups/90', request)
    expect(put.mock.calls[0][1].source_group_ids).toEqual([12, 11])
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
