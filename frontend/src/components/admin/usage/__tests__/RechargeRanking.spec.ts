import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import RechargeRanking from '../RechargeRanking.vue'

const getRechargeRanking = vi.fn()
vi.mock('@/api/admin/dashboard', () => ({ getRechargeRanking: (...args: unknown[]) => getRechargeRanking(...args) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const PaginationStub = { emits: ['update:page'], template: '<button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button>' }

describe('RechargeRanking', () => {
  beforeEach(() => {
    getRechargeRanking.mockReset().mockResolvedValue({ items: [], total: 100, summary: {} })
  })
  it('resets to page one when dates change while retaining source and sort', async () => {
    const wrapper = mount(RechargeRanking, { props: { startDate: '2026-09-01', endDate: '2026-09-01', filters: {} }, global: { stubs: { Pagination: PaginationStub, LoadingSpinner: true } } })
    await flushPromises()
    await wrapper.get('select').setValue('redeem')
    await flushPromises()
    await wrapper.findAll('select')[1].setValue('recharge_count')
    await flushPromises()
    await wrapper.get('[data-test=next-page]').trigger('click')
    await flushPromises()
    expect(getRechargeRanking).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, source: 'redeem', sort_by: 'recharge_count' }))
    await wrapper.setProps({ startDate: '2026-09-02', endDate: '2026-09-07' })
    await flushPromises()
    expect(getRechargeRanking).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, start_date: '2026-09-02', end_date: '2026-09-07', source: 'redeem', sort_by: 'recharge_count' }))
  })
})
