import { beforeEach, describe, expect, it, vi } from 'vitest'
const mocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), delete: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: mocks }))
import { groupsAPI } from '../groups'
beforeEach(() => {
  vi.resetAllMocks()
  mocks.get.mockResolvedValue({ data: { items: [], total: 0, page: 2, page_size: 10 } })
  mocks.post.mockResolvedValue({ data: { id: 9 } })
  mocks.delete.mockResolvedValue({ data: { message: 'success' } })
})
describe('group account user exclusion API scoping', () => {
  it('keeps list pagination and cancellation attached to the selected group', async () => {
    const controller = new AbortController()
    const result = await groupsAPI.listAccountUserExclusions(42, 2, 10, { signal: controller.signal })
    expect(mocks.get).toHaveBeenCalledWith('/admin/groups/42/account-user-exclusions', { params: { page: 2, page_size: 10 }, signal: controller.signal })
    expect(result.total).toBe(0)
  })
  it('adds a user/account pair and removes by exclusion ID within the group', async () => {
    await groupsAPI.addAccountUserExclusion(42, { account_id: 10, user_id: 86 })
    expect(mocks.post).toHaveBeenCalledWith('/admin/groups/42/account-user-exclusions', { account_id: 10, user_id: 86 })
    await groupsAPI.removeAccountUserExclusion(42, 9)
    expect(mocks.delete).toHaveBeenCalledWith('/admin/groups/42/account-user-exclusions/9')
  })
})
