import { describe, expect, it } from 'vitest'
import { createModelAliasFormRow, modelAliasesToAPI, validateModelAliases } from '../groupModelAliases'

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

  it('serializes complete aliases and omits disabled policies', () => {
    const row = createModelAliasFormRow({ alias: ' client ', upstream_model: ' upstream ' })
    const policyRow = createModelAliasFormRow({ alias: 'limited', upstream_model: 'model' })
    policyRow.anti_truncation = {
      enabled: true,
      max_rounds: 2,
      max_output_tokens: null,
      max_duration_seconds: 45,
    }
    expect(modelAliasesToAPI([row, policyRow])).toEqual([
      { alias: 'client', upstream_model: 'upstream' },
      {
        alias: 'limited',
        upstream_model: 'model',
        anti_truncation: { enabled: true, max_rounds: 2, max_duration_seconds: 45 },
      },
    ])
  })

  it('rejects incomplete, duplicate and invalid policies without dropping rows', () => {
    expect(validateModelAliases([createModelAliasFormRow({ alias: 'incomplete' })])).toBe('modelAliasRequired')
    expect(validateModelAliases([
      createModelAliasFormRow({ alias: 'a', upstream_model: 'model' }),
      createModelAliasFormRow({ alias: 'A', upstream_model: 'model' }),
    ])).toBe('modelAliasDuplicate')
    const row = createModelAliasFormRow({ alias: 'a', upstream_model: 'model', anti_truncation: { enabled: true, max_rounds: 4 } })
    expect(validateModelAliases([row])).toBe('modelAliasLimitsInvalid')
    row.anti_truncation.max_rounds = 1
    row.anti_truncation.max_output_tokens = 0
    expect(validateModelAliases([row])).toBeNull()
    expect(validateModelAliases([
      createModelAliasFormRow({ alias: 'long', upstream_model: 'model' }),
      createModelAliasFormRow({ alias: 'MODEL', upstream_model: 'other' }),
    ])).toBe('modelAliasTargetConflict')
  })
})
