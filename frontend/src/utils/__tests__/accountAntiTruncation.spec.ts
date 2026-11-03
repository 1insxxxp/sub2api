import { describe, expect, it } from 'vitest'
import { accountAntiTruncationFromExtra, mergeAccountAntiTruncationExtra, validateAccountAntiTruncation } from '../accountAntiTruncation'

describe('account anti-truncation configuration', () => {
  it('keeps absent policy distinct from an explicit empty rule list', () => {
    expect(accountAntiTruncationFromExtra({ unrelated: true })).toBeNull()
    expect(accountAntiTruncationFromExtra({ anti_truncation: { version: 1, rules: [] } })).toEqual({ version: 1, rules: [] })
  })

  it('represents malformed stored rules as explicitly off without crashing the editor', () => {
    for (const rules of [[null], [{ model: 7, enabled: true }], [{ model: 'x' }]]) {
      expect(accountAntiTruncationFromExtra({ anti_truncation: { version: 1, rules } })).toEqual({ version: 1, rules: [] })
    }
  })

  it('copies a policy without mutating the account snapshot', () => {
    const extra = { anti_truncation: { version: 1, rules: [{ model: 'alias', enabled: true, max_rounds: 2 }] } }
    const policy = accountAntiTruncationFromExtra(extra)!
    policy.rules[0]!.model = 'changed'
    expect(extra.anti_truncation.rules[0]!.model).toBe('alias')
  })

  it('preserves unrelated extra and does not write an unconfigured policy', () => {
    expect(mergeAccountAntiTruncationExtra(undefined, null)).toBeUndefined()
    expect(mergeAccountAntiTruncationExtra({ quota_limit: 12 }, null)).toEqual({ quota_limit: 12 })
    expect(mergeAccountAntiTruncationExtra({ quota_limit: 12 }, { version: 1, rules: [] })).toEqual({ quota_limit: 12, anti_truncation: { version: 1, rules: [] } })
  })

  it('validates exact models, duplicates and limits while retaining zero as no added cap', () => {
    expect(validateAccountAntiTruncation(null)).toBeNull()
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: '防截断/gemini-4-argon', enabled: true, max_output_tokens: 0 }] })).toBeNull()
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: '', enabled: true }] })).toBe('modelRequired')
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: 'has space', enabled: true }] })).toBe('modelInvalid')
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: 'a', enabled: true }, { model: 'a', enabled: false }] })).toBe('modelDuplicate')
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: 'a', enabled: true, max_rounds: 4 }] })).toBe('limitsInvalid')
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: 'a', enabled: true, max_duration_seconds: -1 }] })).toBe('limitsInvalid')
    expect(validateAccountAntiTruncation({ version: 1, rules: [{ model: 'a', enabled: false, max_rounds: 1.2 }] })).toBe('limitsInvalid')
  })
})
