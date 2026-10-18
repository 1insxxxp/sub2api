import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post, remove } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
  remove: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
    delete: remove,
  },
}))

import subAdminCommissionAPI from '@/api/admin/subAdminCommission'

describe('admin sub-admin commission api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    post.mockReset()
    remove.mockReset()
  })

  it('updates global commission settings', async () => {
    put.mockResolvedValueOnce({ data: { commission_rate: 0.08 } })

    const result = await subAdminCommissionAPI.updateSettings({ commission_rate: 0.08 })

    expect(result.commission_rate).toBe(0.08)
    expect(put).toHaveBeenCalledWith('/admin/sub-admin-commissions/settings', {
      commission_rate: 0.08,
    })
  })

  it('replaces shared commission grants', async () => {
    put.mockResolvedValueOnce({ data: [] })

    await subAdminCommissionAPI.replaceGrants({ group_ids: [3, 4] })

    expect(put).toHaveBeenCalledWith('/admin/sub-admin-commissions/grants', {
      group_ids: [3, 4],
    })
  })

  it('loads only the workbench calendar and day group summaries', async () => {
    get.mockResolvedValue({ data: [] })
    await subAdminCommissionAPI.getWorkbenchCalendar({ month: '2026-08' })
    await subAdminCommissionAPI.getWorkbenchDayGroups('2026-08-22')
    expect(get).toHaveBeenNthCalledWith(1, '/admin/workbench/commission/calendar', {
      params: { month: '2026-08' },
    })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/workbench/commission/days/2026-08-22/groups')
    expect(subAdminCommissionAPI).not.toHaveProperty('getWorkbenchDayGroupLogs')
  })

  it('lists exclusions with pagination and preserves the server effective time', async () => {
    const result = { items: [{ id: 1, user_id: 42, user_email: 'client@example.com', username: 'Client', started_at: '2026-10-18T10:00:00Z' }], total: 21, page: 2, page_size: 10, pages: 3 }
    get.mockResolvedValueOnce({ data: result })
    expect(await subAdminCommissionAPI.listExclusions({ page: 2, page_size: 10 })).toEqual(result)
    expect(get).toHaveBeenCalledWith('/admin/sub-admin-commissions/exclusions', { params: { page: 2, page_size: 10 } })
  })

  it('adds and removes an exclusion by user ID without changing user status', async () => {
    post.mockResolvedValueOnce({ data: { id: 1, user_id: 42, started_at: '2026-10-18T10:00:00Z' } })
    remove.mockResolvedValueOnce({ data: { message: 'success' } })
    await subAdminCommissionAPI.addExclusion({ user_id: 42 })
    await subAdminCommissionAPI.removeExclusion(42)
    expect(post).toHaveBeenCalledWith('/admin/sub-admin-commissions/exclusions', { user_id: 42 })
    expect(remove).toHaveBeenCalledWith('/admin/sub-admin-commissions/exclusions/42')
  })
})
