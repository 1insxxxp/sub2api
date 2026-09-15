import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import RechargeRanking from '../RechargeRanking.vue'

const getRechargeRanking = vi.fn()
vi.mock('@/api/admin/dashboard', () => ({ getRechargeRanking: (...args: unknown[]) => getRechargeRanking(...args) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const saveAs = vi.fn()
vi.mock('file-saver', () => ({ saveAs: (...args: unknown[]) => saveAs(...args) }))
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

  it('shows current inviter identity and unbound state on desktop and mobile', async () => {
    getRechargeRanking.mockResolvedValue({
      items: [
        { user_id: 7, email: 'user@example.com', username: 'user', inviter_id: 42, inviter_email: 'inviter@example.com', inviter_username: 'inviter', total_amount: 1, online_amount: 1, redeem_amount: 0, affiliate_amount: 0, admin_amount: 0, reward_amount: 0, refund_amount: 0, other_amount: 0, recharge_count: 0 },
        { user_id: 8, email: 'other@example.com', username: 'other', inviter_id: null, total_amount: 0, online_amount: 0, redeem_amount: 0, affiliate_amount: 0, admin_amount: 0, reward_amount: 0, refund_amount: 0, other_amount: 0, recharge_count: 0 },
      ], total: 2, summary: {}
    })
    const wrapper = mount(RechargeRanking, { props: { startDate: '2026-09-01', endDate: '2026-09-01', filters: {} }, global: { stubs: { Pagination: PaginationStub, LoadingSpinner: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('inviter@example.com')
    expect(wrapper.text()).toContain('#42')
    expect(wrapper.text()).toContain('admin.usage.rechargeRanking.unbound')
    const firstArticle = wrapper.findAll('article')[0]
    const labels = firstArticle.findAll('dt').map((node) => node.text())
    const countIndex = labels.findIndex((label) => label.includes('columns.count'))
    expect(firstArticle.findAll('dd')[countIndex].text()).toBe('0')
  })

  it('includes inviter fields in current-page CSV export', async () => {
    getRechargeRanking.mockResolvedValue({
      items: [{ user_id: 7, email: 'user@example.com', username: 'user', inviter_id: 42, inviter_email: 'inviter@example.com', inviter_username: 'inviter', total_amount: 1, online_amount: 1, redeem_amount: 0, affiliate_amount: 0, admin_amount: 0, reward_amount: 0, refund_amount: 0, other_amount: 0, recharge_count: 0 }], total: 1, summary: {} })
    const wrapper = mount(RechargeRanking, { props: { startDate: '2026-09-01', endDate: '2026-09-01', filters: {} }, global: { stubs: { Pagination: PaginationStub, LoadingSpinner: true } } })
    await flushPromises()
    await wrapper.findAll('button').at(-1)!.trigger('click')
    expect(saveAs).toHaveBeenCalled()
    const blob = saveAs.mock.calls.at(-1)?.[0] as Blob
    const csv = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result))
      reader.onerror = reject
      reader.readAsText(blob)
    })
    expect(csv).toContain('admin.usage.rechargeRanking.columns.inviterId')
    expect(csv).toContain('admin.usage.rechargeRanking.columns.inviterEmail')
    expect(csv).toContain('42')
    expect(csv).toContain('inviter@example.com')
  })

  it('keeps the filter toolbar usable on narrow screens', async () => {
    const wrapper = mount(RechargeRanking, { props: { startDate: '2026-09-01', endDate: '2026-09-01', filters: {} }, global: { stubs: { Pagination: PaginationStub, LoadingSpinner: true } } })
    await flushPromises()
    const toolbar = wrapper.find('div.flex.flex-wrap.items-center.justify-between')
    expect(toolbar.classes()).toContain('min-w-0')
    expect(toolbar.find('p').classes()).toContain('flex-1')
    expect(toolbar.find('select').classes()).toContain('min-w-0')
    expect(toolbar.findAll('button').at(-1)!.classes()).toContain('w-full')
    expect(wrapper.find('div.md\\:hidden').exists()).toBe(true)
  })
})
