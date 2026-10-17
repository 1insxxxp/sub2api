import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import EmptyResponseClaimDialog from '../EmptyResponseClaimDialog.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const log = {
  id: 42,
  model: 'claude-opus-4-6',
  created_at: '2026-08-07T01:00:00Z',
  actual_cost: 12.5,
  output_tokens: 10,
  compensated_cost: 0,
  net_actual_cost: 12.5,
  compensation_eligible: true,
  compensation_eligibility: 'eligible',
  compensation_estimated_refund: 5,
} as any

describe('EmptyResponseClaimDialog', () => {
  it('uses a mobile bottom sheet and presents server-projected refund details', () => {
    const wrapper = mount(EmptyResponseClaimDialog, {
      props: { show: true, log, submitting: false },
      global: { stubs: { Teleport: true } },
    })

    const panel = wrapper.get('[data-testid="empty-response-claim-panel"]')
    expect(panel.classes()).toContain('max-sm:rounded-b-none')
    expect(panel.classes()).toContain('max-sm:w-full')
    expect(wrapper.text()).toContain('claude-opus-4-6')
    expect(wrapper.text()).toContain('$12.500000')
    expect(wrapper.text()).toContain('usage.emptyResponse.outputTokens')
    expect(wrapper.text()).toContain('10')
    expect(wrapper.text()).toContain('usage.emptyResponse.rules')
    expect(wrapper.find('[data-testid="compensation-input-tokens"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="compensation-rate"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="compensation-estimated-refund"]').text()).toBe('$5.000000')
    expect(wrapper.get('[data-testid="empty-response-cap-rules"]').text()).toContain('usage.emptyResponse.capHint')
    expect(wrapper.get('[data-testid="submit-empty-response-claim"]').attributes('disabled')).toBeUndefined()
  })

  it.each([
    { compensation_estimated_refund: undefined },
    { compensation_estimated_refund: 0 },
    { compensation_estimated_refund: Number.NaN },
  ])('disables submission when the server quote is missing or has no refund', async (quote) => {
    const wrapper = mount(EmptyResponseClaimDialog, {
      props: { show: true, log: { ...log, ...quote }, submitting: false },
      global: { stubs: { Teleport: true } },
    })
    expect(wrapper.get('[data-testid="submit-empty-response-claim"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="submit-empty-response-claim"]').trigger('click')
    expect(wrapper.emitted('submit')).toBeUndefined()
    if (quote.compensation_estimated_refund === undefined) {
      expect(wrapper.get('[data-testid="compensation-estimated-refund"]').text()).toBe('—')
    }
  })


  it('displays the exact server quote without deriving or capping money in the client', () => {
    const wrapper = mount(EmptyResponseClaimDialog, {
      props: { show: true, log: { ...log, compensation_estimated_refund: 4.125 }, submitting: false },
      global: { stubs: { Teleport: true } },
    })
    expect(wrapper.get('[data-testid="compensation-estimated-refund"]').text()).toBe('$4.125000')
  })

  it('emits only a human reason and never accepts a client refund amount', async () => {
    const wrapper = mount(EmptyResponseClaimDialog, {
      props: { show: true, log, submitting: false },
      global: { stubs: { Teleport: true } },
    })
    await wrapper.get('textarea').setValue('stream ended without an answer')
    await wrapper.get('[data-testid="submit-empty-response-claim"]').trigger('click')

    expect(wrapper.emitted('submit')).toEqual([['stream ended without an answer']])
  })
})
