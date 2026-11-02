import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))
import { listGroups, listModels, runProbe } from '@/api/admin/modelProbes'

beforeEach(() => { vi.resetAllMocks() })
describe('model probe requests', () => {
  it('loads only the dedicated group and model catalogs with cancellation', async () => {
    const controller = new AbortController()
    const data = { items: [] }
    get.mockResolvedValue({ data })
    expect(await listGroups(controller.signal)).toBe(data)
    expect(await listModels(12, controller.signal)).toBe(data)
    expect(get.mock.calls).toEqual([
      ['/admin/model-probes/groups', { signal: controller.signal }],
      ['/admin/model-probes/groups/12/models', { signal: controller.signal }],
    ])
    expect(post).not.toHaveBeenCalled()
  })
  it('sends a public group/model pair and waits for the bounded backend probe', async () => {
    const controller = new AbortController()
    const data = { group_id: 12, model: 'public-model', status: 'success', duration_ms: 30 }
    post.mockResolvedValue({ data })
    expect(await runProbe(12, 'public-model', controller.signal)).toBe(data)
    expect(post).toHaveBeenCalledWith('/admin/model-probes/run', { group_id: 12, model: 'public-model' }, { signal: controller.signal, timeout: 180000 })
  })
})
