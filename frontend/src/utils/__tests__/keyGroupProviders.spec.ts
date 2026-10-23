import { describe, expect, it } from 'vitest'

import type { GroupPlatform } from '@/types'
import { KEY_GROUP_PROVIDERS, getKeyGroupProvider } from '../keyGroupProviders'

describe('API key group display category', () => {
  it('keeps the requested category order', () => {
    expect(KEY_GROUP_PROVIDERS).toEqual(['anthropic', 'openai', 'gemini', 'grok', 'domestic', 'image', 'other'])
  })

  it.each(['anthropic', 'openai', 'gemini', 'grok', 'domestic', 'image', 'other'] as const)(
    'prefers the explicit %s category over the platform',
    (key_display_category) => {
      expect(getKeyGroupProvider({ platform: 'composite', tag: 'image', key_display_category })).toBe(key_display_category)
    }
  )

  it.each([undefined, '', 'future-category'])('falls back to the existing platform mapping for %s', (key_display_category) => {
    const cases: [GroupPlatform, string][] = [
      ['anthropic', 'anthropic'], ['openai', 'openai'],
      ['kimi', 'domestic'], ['zhipu', 'domestic'], ['deepseek', 'domestic'], ['minimax', 'domestic'],
      ['gemini', 'gemini'], ['grok', 'grok'], ['antigravity', 'other'],
      ['composite', 'other'], ['opencode_go', 'other'], ['typesafe', 'other'],
    ]
    for (const [platform, expected] of cases) {
      expect(getKeyGroupProvider({ platform, key_display_category })).toBe(expected)
    }
  })

  it.each([undefined, '', 'future-category'])('recognizes the image preset tag without an explicit category (%s)', (key_display_category) => {
    expect(getKeyGroupProvider({ platform: 'gemini', tag: 'image', key_display_category })).toBe('image')
  })

  it('does not infer an image category from a capability switch or an ambiguous name', () => {
    const group = { platform: 'openai' as const, tag: 'chat', allow_image_generation: true, name: 'Gemini image Grok' }
    expect(getKeyGroupProvider(group)).toBe('openai')
  })
})
