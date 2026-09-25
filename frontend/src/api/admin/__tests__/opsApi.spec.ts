import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { opsAPI } from '@/api/admin/ops'

describe('admin ops API', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({
      data: {
        total_errors: 0,
        group_count: 0,
        groups: [],
      },
    })
  })

  it('loads the grouped upstream error summary with all filters except pagination', async () => {
    const summary = {
      total_errors: 3,
      group_count: 1,
      groups: [],
    }
    get.mockResolvedValue({ data: summary })

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
