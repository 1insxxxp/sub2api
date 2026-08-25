import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, deleteRequest } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  deleteRequest: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: deleteRequest },
}))

import { drawLottery, getLotteryHistory, getLotteryState } from '@/api/lottery'
import { appendPrizeItems, saveActivity } from '@/api/admin/lottery'

describe('lottery API contracts', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    deleteRequest.mockReset()
  })

  it('loads the user state and submits an idempotent draw key', async () => {
    const state = { activity: { id: 1 }, prizes: [], attempts_used: 0, attempts_remaining: 1 }
    const result = { draw: { id: 4, prize_id: 7, prize_name: 'Balance', prize_type: 'balance' }, attempts_used: 1, attempts_remaining: 0 }
    get.mockResolvedValueOnce({ data: state })
    post.mockResolvedValueOnce({ data: result })

    await expect(getLotteryState()).resolves.toBe(state)
    await expect(drawLottery('attempt-1')).resolves.toBe(result)
    expect(result.draw.prize_id).toBe(7)
    expect(get).toHaveBeenCalledWith('/lottery/state')
    expect(post).toHaveBeenCalledWith('/lottery/draw', { attempt_key: 'attempt-1' })
  })

  it('passes history pagination and admin inventory payloads through unchanged', async () => {
    const history = { items: [], total: 0, page: 2, page_size: 10, pages: 0 }
    const activity = { id: 1, name: 'Lucky Draw' }
    const inventory = { added: 2 }
    get.mockResolvedValueOnce({ data: history })
    put.mockResolvedValueOnce({ data: activity })
    post.mockResolvedValueOnce({ data: inventory })

    await getLotteryHistory({ page: 2, page_size: 10 })
    await saveActivity({ id: 1, name: 'Lucky Draw', description: '', status: 'active', attempt_mode: 'daily', attempt_limit: 1 })
    await appendPrizeItems(8, ['code-a', 'code-b'])

    expect(get).toHaveBeenCalledWith('/lottery/history', { params: { page: 2, page_size: 10 } })
    expect(put).toHaveBeenCalledWith('/admin/lottery/activity', expect.objectContaining({ id: 1, status: 'active' }))
    expect(post).toHaveBeenCalledWith('/admin/lottery/prizes/8/items', { contents: ['code-a', 'code-b'] })
  })
})
