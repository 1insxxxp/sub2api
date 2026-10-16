import { describe, expect, it } from 'vitest'
import { createModelAliasFormRow, modelAliasesToAPI } from '../groupModelAliases'

describe('group model aliases', () => {
  it('hydrates saved policies into editable rows', () => {
    expect(createModelAliasFormRow({
      alias: 'client-model',
      upstream_model: 'provider-model',
      anti_truncation: { enabled: true, max_rounds: 3, max_output_tokens: 1200 },
    })).toEqual({
      alias: 'client-model',
      upstream_model: 'provider-model',
      anti_truncation: {
        enabled: true,
        max_rounds: 3,
        max_output_tokens: 1200,
        max_duration_seconds: null,
      },
    })
  })

  it('serializes complete aliases, omits disabled policies, and drops incomplete rows', () => {
    const row = createModelAliasFormRow({ alias: ' client ', upstream_model: ' upstream ' })
    const policyRow = createModelAliasFormRow({ alias: 'limited', upstream_model: 'model' })
    policyRow.anti_truncation = {
      enabled: true,
      max_rounds: 2,
      max_output_tokens: null,
      max_duration_seconds: 45,
    }
    expect(modelAliasesToAPI([row, policyRow, createModelAliasFormRow({ alias: 'incomplete' })])).toEqual([
      { alias: 'client', upstream_model: 'upstream' },
      {
        alias: 'limited',
        upstream_model: 'model',
        anti_truncation: { enabled: true, max_rounds: 2, max_duration_seconds: 45 },
      },
    ])
  })
})
