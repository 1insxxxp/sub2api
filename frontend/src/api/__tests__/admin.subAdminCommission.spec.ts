import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
  },
}))

import subAdminCommissionAPI from '@/api/admin/subAdminCommission'

describe('admin sub-admin commission api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
  })

  it('updates global commission settings', async () => {
    put.mockResolvedValueOnce({ data: { commission_rate: 0.08 } })

    const result = await subAdminCommissionAPI.updateSettings({ commission_rate: 0.08 })

    expect(result.commission_rate).toBe(0.08)
    expect(put).toHaveBeenCalledWith('/admin/sub-admin-commissions/settings', {
      commission_rate: 0.08,
    })
  })

  it('replaces sub-admin group grants', async () => {
    put.mockResolvedValueOnce({ data: [] })

    await subAdminCommissionAPI.replaceGrants(12, { group_ids: [3, 4] })

    expect(put).toHaveBeenCalledWith('/admin/sub-admin-commissions/grants/12', {
      group_ids: [3, 4],
    })
  })

  it('loads workbench calendar and day logs', async () => {
    get.mockResolvedValueOnce({ data: [] })
    get.mockResolvedValueOnce({
      data: { items: [], total: 0, page: 2, page_size: 10, pages: 0 },
    })

    await subAdminCommissionAPI.getWorkbenchCalendar({ month: '2026-08' })
    await subAdminCommissionAPI.getWorkbenchDayGroupLogs('2026-08-22', 9, {
      page: 2,
      page_size: 10,
    })

    expect(get).toHaveBeenNthCalledWith(1, '/admin/workbench/commission/calendar', {
      params: { month: '2026-08' },
    })
    expect(get).toHaveBeenNthCalledWith(
      2,
      '/admin/workbench/commission/days/2026-08-22/groups/9/logs',
      {
        params: { page: 2, page_size: 10 },
      }
    )
  })
})
