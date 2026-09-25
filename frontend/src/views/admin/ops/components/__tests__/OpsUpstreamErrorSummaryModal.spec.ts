import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import OpsUpstreamErrorSummaryModal from '../OpsUpstreamErrorSummaryModal.vue'

const { getUpstreamErrorSummary } = vi.hoisted(() => ({ getUpstreamErrorSummary: vi.fn() }))
vi.mock('@/api/admin/ops', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/ops')>('@/api/admin/ops')
  return { ...actual, opsAPI: { ...actual.opsAPI, getUpstreamErrorSummary } }
})
vi.mock('vue-i18n', async (importOriginal) => { const actual = await importOriginal<typeof import('vue-i18n')>(); return { ...actual, useI18n: () => ({ t: (key: string, args?: any) => args ? `${key}:${JSON.stringify(args)}` : key }) } })

const BaseDialogStub = { props: ['show', 'title'], template: '<div v-if="show"><h1>{{ title }}</h1><slot /></div>' }
const summary = {
  total_errors: 3, group_count: 1, latest_at: '2026-09-25T01:00:00Z', groups_truncated: false,
  groups: [{ group_id: 1, group_name: 'Group A', error_count: 3, model_count: 1, account_count: 1, latest_at: '2026-09-25T01:00:00Z', total_models: 1, models_truncated: false,
    models: [{ model: 'model-long', error_count: 3, latest_at: '2026-09-25T01:00:00Z', status_codes: { '502': 2, '529': 1 }, total_accounts: 1, accounts_truncated: false,
      accounts: [{ account_id: 2, account_name: 'Account A', error_count: 3, latest_at: '2026-09-25T01:00:00Z', latest_status_code: 529, total_reasons: 2, reasons_truncated: true,
        reasons: [{ message: 'upstream down', error_type: 'upstream', status_code: 502, count: 2, latest_at: '2026-09-25T01:00:00Z', representative_error_id: 77 }] }] }] }]
}
function mountModal(show = true) { return mount(OpsUpstreamErrorSummaryModal, { props: { show, timeRange: '1h' }, global: { stubs: { BaseDialog: BaseDialogStub } } }) }
beforeEach(() => { getUpstreamErrorSummary.mockReset(); getUpstreamErrorSummary.mockResolvedValue(summary) })

describe('OpsUpstreamErrorSummaryModal', () => {
  it('loads four levels with group expanded and status/truncation data', async () => {
    const wrapper = mountModal(); await nextTick(); await nextTick()
    expect(getUpstreamErrorSummary).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Group A'); expect(wrapper.text()).toContain('model-long'); expect(wrapper.text()).toContain('502 ×2')
    const groupButton = wrapper.findAll('button').find(b => b.attributes('aria-expanded') === 'true')
    expect(groupButton).toBeTruthy(); expect(groupButton!.attributes('aria-expanded')).toBe('true'); expect(groupButton!.attributes('aria-controls')).toBeTruthy()
    await groupButton!.trigger('click'); expect(wrapper.text()).not.toContain('model-long')
    await groupButton!.trigger('click');
    const modelButton = wrapper.findAll('button').find(b => b.text().includes('model-long'))!; await modelButton.trigger('click')
    const accountButton = wrapper.findAll('button').find(b => b.text().includes('Account A'))!; expect(accountButton).toBeTruthy(); expect(accountButton.attributes('aria-expanded')).toBe('false'); expect(accountButton.attributes('aria-controls')).toBeTruthy(); await accountButton.trigger('click')
    expect(wrapper.text()).toContain('Account A'); expect(wrapper.text()).toContain('upstream down'); expect(wrapper.text()).toContain('summaryNestedTruncated')
    await wrapper.get('button.font-mono').trigger('click'); expect(wrapper.emitted('openErrorDetail')).toEqual([[77]])
  })
  it('shows error and retries, and supports empty state', async () => {
    getUpstreamErrorSummary.mockRejectedValueOnce(new Error('nope'))
    const wrapper = mountModal(); await nextTick(); await nextTick(); expect(wrapper.text()).toContain('summaryError')
    getUpstreamErrorSummary.mockResolvedValueOnce({ ...summary, total_errors: 0, groups: [] })
    await wrapper.get('button').trigger('click'); await nextTick(); await nextTick(); expect(wrapper.text()).toContain('summaryEmpty')
  })
})
