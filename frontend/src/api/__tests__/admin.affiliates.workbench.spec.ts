import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { get }
}))

import affiliatesAPI from '@/api/admin/affiliates'

describe('admin affiliate workbench api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads the read-only leaderboard from the workbench route', async () => {
    get.mockResolvedValueOnce({ data: { items: [] } })

    await affiliatesAPI.getWorkbenchLeaderboard()

    expect(get).toHaveBeenCalledWith('/admin/workbench/affiliates/leaderboard')
  })
})
