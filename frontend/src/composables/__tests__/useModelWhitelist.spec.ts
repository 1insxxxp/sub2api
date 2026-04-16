import { describe, expect, it, vi } from 'vitest'

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

import {
  buildModelMappingObject,
  getModelsByPlatform,
  getPresetMappingsByPlatform
} from '../useModelWhitelist'

describe('useModelWhitelist', () => {
  it('keeps current OpenAI models and drops retired preview aliases', () => {
    const models = getModelsByPlatform('openai')

    expect(models).toContain('gpt-5.4')
    expect(models).toContain('gpt-5.4-mini')
    expect(models).toContain('gpt-5.4-nano')
    expect(models).toContain('gpt-5.4-2026-03-05')
    expect(models).toContain('gpt-4o-audio-preview')
    expect(models).toContain('gpt-4o-realtime-preview')

    expect(models).not.toContain('gpt-4-turbo-preview')
    expect(models).not.toContain('gpt-4.5-preview')
    expect(models).not.toContain('o1-preview')
    expect(models).not.toContain('o1-mini')
    expect(models).not.toContain('chatgpt-4o-latest')
  })

  it('keeps supported Anthropic IDs and removes retired direct models', () => {
    const models = getModelsByPlatform('claude')

    expect(models).toContain('claude-sonnet-4-6')
    expect(models).toContain('claude-opus-4-6')
    expect(models).toContain('claude-haiku-4-5-20251001')
    expect(models).toContain('claude-3-haiku-20240307')

    expect(models).not.toContain('claude-3-5-sonnet-20241022')
    expect(models).not.toContain('claude-3-5-sonnet-20240620')
    expect(models).not.toContain('claude-3-5-haiku-20241022')
    expect(models).not.toContain('claude-3-opus-20240229')
    expect(models).not.toContain('claude-3-sonnet-20240229')
    expect(models).not.toContain('claude-3-7-sonnet-20250219')
    expect(models).not.toContain('claude-2.1')
    expect(models).not.toContain('claude-2.0')
    expect(models).not.toContain('claude-instant-1.2')
  })

  it('replaces the retired Gemini 3 Pro preview with the 3.1 Pro preview family', () => {
    const models = getModelsByPlatform('gemini')

    expect(models).toContain('gemini-3.1-pro-preview')
    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.0-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
    expect(models).not.toContain('gemini-3-pro-preview')
  })

  it('keeps antigravity image compatibility entries available', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models).toContain('gemini-3-pro-image')
    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash-lite'))
  })

  it('drops retired Anthropic preset shortcuts while keeping current presets', () => {
    const presets = getPresetMappingsByPlatform('claude')
    const sources = presets.map(preset => preset.from)

    expect(sources).toContain('claude-sonnet-4-6')
    expect(sources).toContain('claude-opus-4-6')
    expect(sources).toContain('claude-haiku-4-5-20251001')
    expect(sources).not.toContain('claude-3-5-haiku-20241022')
  })

  it('ignores wildcard entries when building whitelist mappings', () => {
    const mapping = buildModelMappingObject('whitelist', ['claude-*', 'gemini-3.1-flash-image'], [])

    expect(mapping).toEqual({
      'gemini-3.1-flash-image': 'gemini-3.1-flash-image'
    })
  })

  it('keeps exact GPT-5.4 variants in whitelist mode', () => {
    const mapping = buildModelMappingObject('whitelist', ['gpt-5.4-2026-03-05', 'gpt-5.4-mini', 'gpt-5.4-nano'], [])

    expect(mapping).toEqual({
      'gpt-5.4-2026-03-05': 'gpt-5.4-2026-03-05',
      'gpt-5.4-mini': 'gpt-5.4-mini',
      'gpt-5.4-nano': 'gpt-5.4-nano'
    })
  })
})
