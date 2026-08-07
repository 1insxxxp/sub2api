import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import EmptyResponseClaimsPanel from '../EmptyResponseClaimsPanel.vue'

const { listClaims, getClaimMetrics, approveClaim, rejectClaim, batchClaims } = vi.hoisted(() => ({
  listClaims: vi.fn(),
  getClaimMetrics: vi.fn(),
  approveClaim: vi.fn(),
  rejectClaim: vi.fn(),
  batchClaims: vi.fn(),
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: { listClaims, getClaimMetrics, approveClaim, rejectClaim, batchClaims },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const claim = {
  id: 1,
  usage_log_id: 9,
  status: 'manual_review',
  reason_code: 'missing_evidence',
  estimated_refund: 1.25,
  refunded_amount: 0,
  user_id: 7,
  user_email: 'user@example.com',
  api_key_id: 8,
  account_id: 3,
  account_name: 'pool-1',
  group_id: 2,
  group_name: 'cc',
  model: 'claude-opus-4-6',
  user_reason: 'empty',
  admin_note: '',
  rule_version: 1,
  balance_refund: 0,
  subscription_refund: 0,
  api_key_quota_refund: 0,
  evidence: {
    http_status: 200,
    upstream_status: 200,
    has_text: false,
    has_tool_call: false,
    has_reasoning: false,
    has_media: false,
    output_bytes: 0,
    event_count: 2,
    stream_completed: true,
    disconnect_source: 'none',
    upstream_error_kind: 'none',
    collector_version: 1,
  },
  created_at: '2026-08-07T00:00:00Z',
  updated_at: '2026-08-07T00:00:00Z',
}

describe('EmptyResponseClaimsPanel', () => {
  beforeEach(() => {
    listClaims.mockReset().mockResolvedValue({ items: [claim], total: 1, pages: 1 })
    getClaimMetrics.mockReset().mockResolvedValue({
      total_charged_requests: 100,
      total_claims: 4,
      compensated_claims: 3,
      manual_review_claims: 1,
      rejected_claims: 0,
      total_refund_amount: 2.5,
      empty_response_rate: 0.04,
      by_group: [{ id: 2, name: 'cc', charged_requests: 80, claims: 4, refund_amount: 2.5, empty_response_rate: 0.05 }],
      by_account: [{ id: 3, name: 'account-ranking', charged_requests: 10, claims: 3, refund_amount: 2, empty_response_rate: 0.3 }],
      by_model: [{ id: 0, name: 'model-ranking', charged_requests: 20, claims: 2, refund_amount: 1.5, empty_response_rate: 0.1 }],
    })
    approveClaim.mockReset().mockResolvedValue({ ...claim, status: 'compensated' })
    rejectClaim.mockReset().mockResolvedValue({ ...claim, status: 'rejected' })
    batchClaims.mockReset().mockResolvedValue({ succeeded: [1], failed: {}, claims: [] })
  })

  it('renders desktop table, mobile cards, structured evidence and metrics', async () => {
    const wrapper = mount(EmptyResponseClaimsPanel, { props: { startDate: '2026-08-01', endDate: '2026-08-07' } })
    await flushPromises()

    expect(wrapper.get('[data-testid="claim-desktop-table"]').classes()).toContain('md:block')
    expect(wrapper.get('[data-testid="claim-mobile-cards"]').classes()).toContain('md:hidden')
    expect(wrapper.text()).toContain('claude-opus-4-6')
    expect(wrapper.text()).toContain('HTTP 200')
    expect(wrapper.text()).toContain('4.00%')
    expect(wrapper.text()).toContain('account-ranking')
    expect(wrapper.text()).toContain('model-ranking')
    expect(wrapper.text()).toContain('admin.usage.emptyResponseClaims.warning')
    expect(wrapper.text()).not.toContain('response body')
  })

  it('approves one claim and normalizes batch actions for the API', async () => {
    const wrapper = mount(EmptyResponseClaimsPanel, { props: { startDate: '2026-08-01', endDate: '2026-08-07' } })
    await flushPromises()

    await wrapper.get('[data-testid="approve-claim-1"]').trigger('click')
    await wrapper.get('[data-testid="submit-claim-review"]').trigger('click')
    await flushPromises()
    expect(approveClaim).toHaveBeenCalledWith(1, { note: '' })

    await wrapper.findAll('input[type="checkbox"]')[0].setValue(true)
    await wrapper.get('[data-testid="batch-approve-claims"]').trigger('click')
    await wrapper.get('[data-testid="submit-claim-review"]').trigger('click')
    await flushPromises()
    expect(batchClaims).toHaveBeenCalledWith({ ids: [1], action: 'approved', note: '' })
  })

  it('applies status filter and requires a rejection note', async () => {
    const wrapper = mount(EmptyResponseClaimsPanel, { props: { startDate: '2026-08-01', endDate: '2026-08-07' } })
    await flushPromises()
    await wrapper.get('[data-testid="claim-status-filter"]').setValue('manual_review')
    await flushPromises()
    expect(listClaims).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'manual_review' }))

    await wrapper.get('[data-testid="reject-claim-1"]').trigger('click')
    await flushPromises()
    const submit = wrapper.get('[data-testid="submit-claim-review"]')
    expect(submit.attributes('disabled')).toBeDefined()
    await wrapper.get('textarea').setValue('not an empty response')
    expect(wrapper.get('[data-testid="submit-claim-review"]').attributes('disabled')).toBeUndefined()
  })
})
