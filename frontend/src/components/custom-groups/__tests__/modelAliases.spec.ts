import { describe, expect, it } from 'vitest'

import { sourceMappingKey, suggestCallName, validateCallNames } from '../modelAliases'

describe('custom group model aliases', () => {
  it('allows the same real model from different source groups', () => {
    expect(sourceMappingKey(11, 'Claude-Sonnet-4-5')).not.toBe(sourceMappingKey(12, 'claude-sonnet-4-5'))
  })

  it('uses the real model first and adds a readable source suffix on collision', () => {
    expect(suggestCallName('claude-sonnet-4-5', '余额 Pro', [])).toBe('claude-sonnet-4-5')
    expect(suggestCallName('claude-sonnet-4-5', '余额 Pro', ['CLAUDE-SONNET-4-5'])).toBe('claude-sonnet-4-5-余额-pro')
  })

  it('rejects blank, oversized, and case-insensitive duplicate call names', () => {
    const errors = validateCallNames([
      { key: 'a', callName: 'Same' },
      { key: 'b', callName: ' same ' },
      { key: 'c', callName: ' ' },
      { key: 'd', callName: 'x'.repeat(201) },
    ])
    expect(errors.get('a')).toContain('重复')
    expect(errors.get('b')).toContain('重复')
    expect(errors.get('c')).toContain('不能为空')
    expect(errors.get('d')).toContain('200')
  })
})
