import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import OpsSlaErrorSummaryModal from '../OpsSlaErrorSummaryModal.vue'

const { getSLAErrorSummary } = vi.hoisted(() => ({ getSLAErrorSummary: vi.fn() }))
vi.mock('@/api/admin/ops', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/ops')>('@/api/admin/ops')
  return { ...actual, opsAPI: { ...actual.opsAPI, getSLAErrorSummary } }
})
vi.mock('vue-i18n', async (importOriginal) => { const actual = await importOriginal<typeof import('vue-i18n')>(); return { ...actual, useI18n: () => ({ t: (key: string, args?: any) => args ? `${key}:${JSON.stringify(args)}` : key }) } })
const BaseDialogStub = { props: ['show', 'title'], template: '<div v-if="show"><h1>{{ title }}</h1><slot /></div>' }
const summary = {
  total_errors: 2, group_count: 1, latest_at: '2026-09-25T01:00:00Z', groups_truncated: false,
  groups: [{ group_id: 1, group_name: 'Group A', error_count: 2, model_count: 1, account_count: 1, latest_at: '2026-09-25T01:00:00Z', total_models: 1, models_truncated: false,
    models: [{ model: 'model-long', error_count: 2, latest_at: '2026-09-25T01:00:00Z', status_codes: { '503': 2 }, total_accounts: 1, accounts_truncated: false,
      accounts: [{ account_id: 2, account_name: 'Account A', error_count: 2, latest_at: '2026-09-25T01:00:00Z', latest_status_code: 503, total_reasons: 1, reasons_truncated: false,
        reasons: [{ message: 'request failed', error_type: 'request', status_code: 503, count: 2, latest_at: '2026-09-25T01:00:00Z', representative_error_id: 88 }] }] }] }]
}
function mountModal(show = true) { return mount(OpsSlaErrorSummaryModal, { props: { show, timeRange: '1h' }, global: { stubs: { BaseDialog: BaseDialogStub } } }) }
function summaryWithRows(rowCount: number) {
  const group = summary.groups[0]
  const model = group.models[0]
  const account = model.accounts[0]
  const reasons = Array.from({ length: rowCount }, (_, index) => ({
    ...account.reasons[0],
    message: `request failed ${index}`,
    representative_error_id: 200 + index,
    latest_at: new Date(Date.UTC(2026, 8, 25, 1, index, 0)).toISOString()
  }))
  return {
    ...summary,
    total_errors: rowCount,
    groups: [{ ...group, error_count: rowCount, models: [{ ...model, error_count: rowCount, accounts: [{ ...account, error_count: rowCount, total_reasons: rowCount, reasons }] }] }]
  }
}
beforeEach(() => { getSLAErrorSummary.mockReset(); getSLAErrorSummary.mockResolvedValue(summary) })
describe('OpsSlaErrorSummaryModal', () => {
  it('renders SLA grouped rows and emits detail id', async () => {
    const wrapper = mountModal(); await nextTick(); await nextTick()
    expect(getSLAErrorSummary).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Group A'); expect(wrapper.text()).toContain('model-long'); expect(wrapper.text()).toContain('request failed')
    expect(wrapper.find('[data-testid=ops-sla-summary-table-0]').exists()).toBe(true)
    const accountToggle = wrapper.find('button[aria-expanded="true"]')
    expect(accountToggle.text()).toContain('Account A')
    await accountToggle.trigger('click')
    expect(wrapper.find('[data-testid=ops-sla-summary-row-88]').exists()).toBe(false)
    await wrapper.find('button[aria-expanded="false"]').trigger('click')
    expect(wrapper.find('[data-testid=ops-sla-summary-row-88]').exists()).toBe(true)
    await wrapper.find('[data-testid=ops-sla-summary-row-88] button').trigger('click')
    expect(wrapper.emitted('openErrorDetail')).toEqual([[88]])
  })
  it('shows error and empty states', async () => {
    getSLAErrorSummary.mockRejectedValueOnce(new Error('nope'))
    const wrapper = mountModal(); await nextTick(); await nextTick(); expect(wrapper.text()).toContain('slaSummaryError')
    getSLAErrorSummary.mockResolvedValueOnce({ ...summary, total_errors: 0, groups: [] })
    await wrapper.get('button').trigger('click'); await nextTick(); await nextTick(); expect(wrapper.text()).toContain('slaSummaryEmpty')
  })
  it('paginates each group with ten latest rows by default', async () => {
    getSLAErrorSummary.mockResolvedValueOnce(summaryWithRows(11))
    const wrapper = mountModal(); await nextTick(); await nextTick()
    expect(wrapper.findAll('[data-testid^="ops-sla-summary-row-"]').length).toBe(10)
    expect(wrapper.find('[data-testid="ops-sla-summary-row-210"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="ops-sla-summary-row-200"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="ops-sla-summary-pagination-0"]').exists()).toBe(true)
    await wrapper.findAll('button[aria-label="pagination.next"]')[0].trigger('click'); await nextTick()
    expect(wrapper.find('[data-testid="ops-sla-summary-row-200"]').exists()).toBe(true)
  })
})
