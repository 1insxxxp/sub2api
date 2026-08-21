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
  actual_cost: 1.25,
  output_tokens: 10,
  compensated_cost: 0,
  net_actual_cost: 1.25,
  compensation_eligible: true,
  compensation_eligibility: 'eligible',
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
    expect(wrapper.text()).toContain('$1.250000')
    expect(wrapper.text()).toContain('usage.emptyResponse.outputTokens')
    expect(wrapper.text()).toContain('10')
    expect(wrapper.text()).toContain('usage.emptyResponse.rules')
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
