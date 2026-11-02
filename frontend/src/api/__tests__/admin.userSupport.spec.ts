import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
import * as support from '@/api/admin/userSupport'

beforeEach(() => { post.mockReset() })
describe('support read-only queries and explicit grants', () => {
  it('searches fragments without a grant request', async () => {
    const data = { items: [{ id: 7147, email: 'customer@example.com' }] }
    post.mockResolvedValue({ data })
    expect(typeof support.search).toBe('function')
    expect(await support.search('customer')).toEqual(data)
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/admin/user-support/search', { query: 'customer' })
  })
  it('keeps lookup separate from issuing credit', async () => {
    const data = { user: { id: 7147 }, trial: { status: 'eligible' } }
    post.mockResolvedValue({ data })
    expect(await support.lookup('7147')).toEqual(data)
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/admin/user-support/lookup', { account: '7147' })
  })
  it('grants only to the explicit numeric user ID', async () => {
    const data = { user: { id: 7147 }, trial: { status: 'granted' } }
    post.mockResolvedValue({ data })
    expect(typeof support.grant).toBe('function')
    expect(await support.grant(7147)).toEqual(data)
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/admin/user-support/grant', { user_id: 7147 })
  })
})
