import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import UserTokenRanking from '../UserTokenRanking.vue'

const getUserBreakdown = vi.fn()

vi.mock('@/api/admin/dashboard', () => ({
  getUserBreakdown: (...args: unknown[]) => getUserBreakdown(...args),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const item = (id: number, tokens: number) => ({
  user_id: id,
  email: `u${id}@test.com`,
  requests: 1,
  input_tokens: tokens,
  output_tokens: 0,
  cache_tokens: 0,
  total_tokens: tokens,
  actual_cost: 0.5,
})

const mountRanking = (props: Record<string, unknown> = {}) =>
  mount(UserTokenRanking, {
    props: {
      startDate: '2026-07-01',
      endDate: '2026-07-08',
      filters: {},
      ...props,
    },
    global: { stubs: { Select: true, LoadingSpinner: true, Pagination: true } },
  })

describe('UserTokenRanking', () => {
  beforeEach(() => {
    getUserBreakdown.mockReset()
    getUserBreakdown.mockResolvedValue({ users: [item(1, 100), item(2, 50)] })
  })

  it('loads on mount with shared filters and emits select-user with id + email on row click', async () => {
    const wrapper = mountRanking({ filters: { group_id: 3 }, model: 'claude-fable-5' })
    await flushPromises()

    expect(getUserBreakdown).toHaveBeenCalledWith(expect.objectContaining({
      group_id: 3,
      model: 'claude-fable-5',
      start_date: '2026-07-01',
      end_date: '2026-07-08',
      sort_by: 'balance_deducted',
      page: 1,
      page_size: 50,
      sort_order: 'desc',
    }))

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)

    await rows[0].trigger('click')
    expect(wrapper.emitted('select-user')![0]).toEqual([1, 'u1@test.com'])
  })

  it('toggles cost sorting direction on repeated header clicks', async () => {
    const wrapper = mountRanking()
    await flushPromises()
    await wrapper.get('[data-sort=balance_deducted]').trigger('click')
    await flushPromises()
    expect(getUserBreakdown).toHaveBeenLastCalledWith(expect.objectContaining({ sort_by: 'balance_deducted', sort_order: 'asc', page: 1 }))
  })

  it('shows an error and supports retry instead of presenting failures as empty results', async () => {
    getUserBreakdown.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountRanking()
    await flushPromises()
    expect(wrapper.find('[role=alert]').exists()).toBe(true)
    await wrapper.get('[data-test=ranking-retry]').trigger('click')
    await flushPromises()
    expect(getUserBreakdown).toHaveBeenCalledTimes(2)
  })

  it('reloads when shared filters change', async () => {
    const wrapper = mountRanking()
    await flushPromises()
    expect(getUserBreakdown).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ filters: { user_id: 9 } })
    await flushPromises()

    expect(getUserBreakdown).toHaveBeenCalledTimes(2)
    expect(getUserBreakdown).toHaveBeenLastCalledWith(expect.objectContaining({ user_id: 9 }))
  })
})
