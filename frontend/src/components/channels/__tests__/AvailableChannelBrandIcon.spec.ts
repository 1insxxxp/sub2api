import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AvailableChannelBrandIcon from '../AvailableChannelBrandIcon.vue'
import { resolveAvailableChannelBrand } from '../availableChannelBrand'

describe('available channel platform branding', () => {
  it.each([
    ['openai', 'openai'],
    ['OpenAI Compatible', 'openai'],
    ['anthropic', 'anthropic'],
    ['Claude', 'anthropic'],
    ['gemini', 'gemini'],
    ['Google AI Studio', 'gemini'],
  ])('maps %s to the %s brand', (platform, brand) => {
    expect(resolveAvailableChannelBrand(platform).key).toBe(brand)
  })

  it('uses distinct colorful tokens and a deterministic fallback', () => {
    const openai = resolveAvailableChannelBrand('openai')
    const anthropic = resolveAvailableChannelBrand('anthropic')
    const gemini = resolveAvailableChannelBrand('gemini')
    expect(new Set([openai.accentClass, anthropic.accentClass, gemini.accentClass]).size).toBe(3)
    expect(resolveAvailableChannelBrand('Custom Relay')).toEqual(resolveAvailableChannelBrand('custom relay'))
    expect(resolveAvailableChannelBrand('Custom Relay').key).toBe('generic')
  })

  it('renders a colored icon with an accessible platform label', () => {
    const wrapper = mount(AvailableChannelBrandIcon, { props: { platform: 'Gemini' } })
    expect(wrapper.attributes('aria-label')).toBe('Gemini')
    expect(wrapper.attributes('data-brand')).toBe('gemini')
    expect(wrapper.classes().join(' ')).toContain('bg-')
    expect(wrapper.find('svg').exists()).toBe(true)
  })
})
