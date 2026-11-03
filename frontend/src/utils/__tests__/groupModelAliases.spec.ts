import { describe, expect, it } from 'vitest'
import { createModelAliasFormRow, modelAliasesToAPI, validateModelAliases } from '../groupModelAliases'

describe('group model aliases', () => {
  it('preserves a saved alias without carrying its retired continuation policy into the form or API', () => {
    const savedAlias = {
      alias: ' client-model ',
      upstream_model: ' provider-model ',
      anti_truncation: { enabled: true, max_rounds: 3, max_output_tokens: 1200 },
    }
    expect(createModelAliasFormRow(savedAlias)).toEqual({
      alias: ' client-model ',
      upstream_model: ' provider-model ',
    })
    expect(modelAliasesToAPI([savedAlias])).toEqual([
      { alias: 'client-model', upstream_model: 'provider-model' },
    ])
  })

  it('rejects incomplete, duplicate and conflicting aliases', () => {
    expect(validateModelAliases([createModelAliasFormRow({ alias: 'incomplete' })])).toBe('modelAliasRequired')
    expect(validateModelAliases([
      createModelAliasFormRow({ alias: 'a', upstream_model: 'model' }),
      createModelAliasFormRow({ alias: 'A', upstream_model: 'model' }),
    ])).toBe('modelAliasDuplicate')
    expect(validateModelAliases([
      createModelAliasFormRow({ alias: 'long', upstream_model: 'model' }),
      createModelAliasFormRow({ alias: 'MODEL', upstream_model: 'other' }),
    ])).toBe('modelAliasTargetConflict')
  })
})
