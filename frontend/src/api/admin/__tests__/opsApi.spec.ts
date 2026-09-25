import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { opsAPI, type OpsUpstreamErrorSummary } from '@/api/admin/ops'

describe('admin ops API', () => {
  beforeEach(() => {
    get.mockReset()
    const emptySummary: OpsUpstreamErrorSummary = {
      total_errors: 0,
      group_count: 0,
      latest_at: null,
      groups: [],
      groups_truncated: false,
    }
    get.mockResolvedValue({ data: emptySummary })
  })

  it('loads the grouped upstream error summary with all filters except pagination', async () => {
    const summary: OpsUpstreamErrorSummary = {
      total_errors: 3,
      group_count: 1,
      latest_at: '2026-09-25T06:00:00Z',
      groups_truncated: false,
      groups: [{
        group_id: null,
        group_name: '未分组',
        error_count: 3,
        model_count: 1,
        account_count: 1,
        latest_at: '2026-09-25T06:00:00Z',
        total_models: 1,
        models_truncated: false,
        models: [{
          model: 'gpt-5',
          error_count: 3,
          latest_at: '2026-09-25T06:00:00Z',
          status_codes: { '429': 3 },
          total_accounts: 1,
          accounts_truncated: false,
          accounts: [{
            account_id: null,
            account_name: '未知账号',
            error_count: 3,
            latest_at: '2026-09-25T06:00:00Z',
            latest_status_code: 429,
            total_reasons: 1,
            reasons_truncated: false,
            reasons: [{
              message: 'rate limited',
              error_type: 'upstream_error',
              status_code: 429,
              count: 3,
              latest_at: '2026-09-25T06:00:00Z',
              representative_error_id: 99,
            }],
          }],
        }],
      }],
    }
    get.mockReset()
    get.mockResolvedValueOnce({ data: summary }).mockResolvedValueOnce({
      data: {
        total_errors: 0,
        group_count: 0,
        latest_at: null,
        groups: [],
        groups_truncated: false,
      } satisfies OpsUpstreamErrorSummary,
    })

    const params = {
      time_range: '6h',
      start_time: '2026-09-25T00:00:00Z',
      end_time: '2026-09-25T06:00:00Z',
      platform: 'openai',
      group_id: 17,
      account_id: 23,
      q: 'timeout',
      phase: 'upstream',
      error_owner: 'provider',
      error_source: 'upstream_http',
      view: 'errors' as const,
      resolved: 'false',
      status_codes: '429,500',
      status_codes_other: 'true',
      model: 'gpt-5',
      category: 'upstream',
      page: 4,
      page_size: 100,
    }

    await expect(opsAPI.getUpstreamErrorSummary(params)).resolves.toEqual(summary)

    const response = await opsAPI.getUpstreamErrorSummary({})
    expect(response.groups).toEqual([])
    expect(response.groups_truncated).toBe(false)
    expect(summary.groups[0].models[0].accounts[0].reasons[0]).toMatchObject({
      count: 3,
      representative_error_id: 99,
    })

    expect(get).toHaveBeenCalledWith('/admin/ops/upstream-errors/summary', {
      params: {
        time_range: '6h',
        start_time: '2026-09-25T00:00:00Z',
        end_time: '2026-09-25T06:00:00Z',
        platform: 'openai',
        group_id: 17,
        account_id: 23,
        q: 'timeout',
        phase: 'upstream',
        error_owner: 'provider',
        error_source: 'upstream_http',
        view: 'errors',
        resolved: 'false',
        status_codes: '429,500',
        status_codes_other: 'true',
        model: 'gpt-5',
        category: 'upstream',
      },
    })
  })
})
