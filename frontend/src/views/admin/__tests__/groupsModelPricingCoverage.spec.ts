import { describe, expect, it } from 'vitest'

import type { PricingFormEntry } from '@/components/admin/channel/types'
import {
  advertisedModelsForCoverage,
  appendPendingPricingEntries,
  isCoverageResolved,
  normalizeCoverageModels,
  requiredPricingModels
} from '../groupsModelPricingCoverage'

const priceEntry = (model: string): PricingFormEntry => ({
  models: [model],
  billing_mode: 'token',
  input_price: null,
  output_price: null,
  cache_write_price: null,
  cache_read_price: null,
  image_input_price: null,
  image_output_price: null,
  per_request_price: null,
  intervals: [],
  time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: false, periods: [] }
})

describe('groupsModelPricingCoverage', () => {
  it('normalizes and de-duplicates advertised model names', () => {
    expect(normalizeCoverageModels([' GPT-5 ', 'gpt-5', '', 'Claude-4'])).toEqual([
      'GPT-5',
      'Claude-4'
    ])
    expect(
      advertisedModelsForCoverage({
        enabled: true,
        savedModels: [],
        items: [
          { id: ' GPT-5 ', selected: true },
          { id: 'gpt-5', selected: true },
          { id: 'claude-4', selected: false }
        ]
      })
    ).toEqual(['GPT-5'])
  })

  it('adds only missing pricing entries and preserves administrator edits', () => {
    const existing = [{ ...priceEntry('kept-model'), input_price: 12 }]
    const merged = appendPendingPricingEntries(
      existing,
      {
        models: [
          { model: 'kept-model', status: 'missing' },
          { model: 'new-model', status: 'missing' },
          { model: 'invalid-model', status: 'invalid' }
        ]
      },
      priceEntry
    )

    expect(merged).toHaveLength(3)
    expect(merged[0].input_price).toBe(12)
    expect(merged.slice(1).map((entry) => entry.models[0])).toEqual(['new-model', 'invalid-model'])
    expect(requiredPricingModels({ models: [{ model: 'new-model', status: 'missing' }] })).toEqual([
      'new-model'
    ])
  })

  it('requires every advertised model to be priced before save', () => {
    const models = ['priced-model', 'missing-model']
    expect(
      isCoverageResolved(models, {
        models: [
          { model: 'priced-model', status: 'priced' },
          { model: 'missing-model', status: 'missing' }
        ]
      })
    ).toBe(false)
    expect(
      isCoverageResolved(models, {
        models: [
          { model: 'priced-model', status: 'priced' },
          { model: 'missing-model', status: 'priced' }
        ]
      })
    ).toBe(true)
  })
})
