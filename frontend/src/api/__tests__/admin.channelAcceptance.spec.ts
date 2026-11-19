import { beforeEach, describe, expect, it, vi } from 'vitest'
const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))
import { channelAcceptanceApi } from '@/api/admin/channelAcceptance'
beforeEach(() => { vi.resetAllMocks() })
describe('channel acceptance API', () => {
  it('loads selected targets and history with explicit account IDs', async () => {
    get.mockResolvedValue({ data: { items: [{ account_id: 7 }] } })
    await expect(channelAcceptanceApi.targets([7, 9])).resolves.toEqual([{ account_id: 7 }])
    expect(get).toHaveBeenLastCalledWith('/admin/channel-acceptance/targets', { params: { account_ids: '7,9' } })
    await channelAcceptanceApi.list([7])
    expect(get).toHaveBeenLastCalledWith('/admin/channel-acceptance/runs', { params: { account_ids: '7' } })
  })
  it('starts only the approved preview and returns downloadable JSON', async () => {
    post.mockResolvedValue({ data: { id: 'run-1' } })
    await channelAcceptanceApi.start('preview-1')
    expect(post).toHaveBeenCalledWith('/admin/channel-acceptance/runs', { preview_id: 'preview-1' })
    const report = new Blob(['{}'], { type: 'application/json' })
    get.mockResolvedValue({ data: report })
    await expect(channelAcceptanceApi.exportReport('run-1')).resolves.toBe(report)
    expect(get).toHaveBeenLastCalledWith('/admin/channel-acceptance/runs/run-1/export', { responseType: 'blob' })
  })
})
