import { describe, expect, it } from 'vitest'

import type { GroupPlatform } from '@/types'
import { getKeyGroupProvider } from '../keyGroupProviders'

describe('API key group display category', () => {
  it.each(['anthropic', 'openai', 'domestic', 'other'] as const)(
    'prefers the explicit %s category over the platform',
    (key_display_category) => {
      expect(getKeyGroupProvider({ platform: 'composite', key_display_category })).toBe(key_display_category)
    }
  )

  it.each([undefined, '', 'future-category'])('falls back to the existing platform mapping for %s', (key_display_category) => {
    const cases: [GroupPlatform, string][] = [
      ['anthropic', 'anthropic'], ['openai', 'openai'],
      ['kimi', 'domestic'], ['zhipu', 'domestic'], ['deepseek', 'domestic'], ['minimax', 'domestic'],
      ['gemini', 'other'], ['grok', 'other'], ['antigravity', 'other'],
      ['composite', 'other'], ['opencode_go', 'other'], ['typesafe', 'other'],
    ]
    for (const [platform, expected] of cases) {
      expect(getKeyGroupProvider({ platform, key_display_category })).toBe(expected)
    }
  })
})
