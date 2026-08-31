import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import PricingEntryCard from '../PricingEntryCard.vue'
import type { PricingFormEntry } from '../types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const entry: PricingFormEntry = {
  models: ['gpt-5.5'],
  billing_mode: 'per_request',
  input_price: null,
  output_price: null,
  cache_write_price: null,
  cache_read_price: null,
  image_input_price: null,
  image_output_price: null,
  per_request_price: 0.05,
  intervals: [],
}

describe('PricingEntryCard', () => {
  it('explains successful-request charging and token fallback for per-request models', async () => {
    const wrapper = mount(PricingEntryCard, {
      props: { entry, platform: 'openai' },
      global: {
        stubs: {
          Icon: true,
          Select: true,
          ModelTagInput: true,
          IntervalRow: true,
        },
      },
    })

    await wrapper.get('.flex.cursor-pointer').trigger('click')

    const hint = wrapper.get('[data-test="per-request-billing-hint"]')
    expect(hint.text()).toContain('admin.channels.form.perRequestSuccessHint')
    expect(hint.text()).toContain('admin.channels.form.perRequestFallbackHint')
  })

  it('opens and marks entries that still need pricing', () => {
    const wrapper = mount(PricingEntryCard, {
      props: { entry, platform: 'openai', requiredModels: ['gpt-5.5'] },
      global: {
        stubs: {
          Icon: true,
          Select: true,
          ModelTagInput: true,
          IntervalRow: true,
        },
      },
    })

    expect(wrapper.get('[data-testid="pricing-entry-required"]').text()).toContain(
      'admin.groups.modelPricing.requiredHint'
    )
  })
})
