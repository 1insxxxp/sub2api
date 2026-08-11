import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { GroupPlatform } from '@/types'
import { platformGroupBadgeClass } from '@/utils/platformColors'
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
    ['antigravity', 'antigravity'],
    ['grok', 'grok'],
    ['xAI', 'grok'],
    ['composite', 'composite'],
  ])('maps %s to the %s brand', (platform, brand) => {
    expect(resolveAvailableChannelBrand(platform).key).toBe(brand)
    expect(resolveAvailableChannelBrand(platform).platform).toBe(brand)
  })

  it('uses a deterministic generic fallback for unknown platforms', () => {
    expect(resolveAvailableChannelBrand('Custom Relay')).toEqual(resolveAvailableChannelBrand('custom relay'))
    expect(resolveAvailableChannelBrand('Custom Relay').key).toBe('generic')
    expect(resolveAvailableChannelBrand('Custom Relay').platform).toBeUndefined()
  })

  it.each([
    ['Anthropic', 'anthropic'],
    ['OpenAI', 'openai'],
    ['Gemini', 'gemini'],
    ['Antigravity', 'antigravity'],
    ['Grok', 'grok'],
    ['Composite', 'composite'],
    ['Custom Relay', undefined],
  ] as const)('renders the shared %s platform icon and normal theme', (input, platform) => {
    const wrapper = mount(AvailableChannelBrandIcon, {
      props: { platform: input },
      global: {
        stubs: {
          PlatformIcon: {
            props: ['platform', 'size'],
            template: '<i data-platform-icon :data-platform="platform" :data-size="size" />',
          },
        },
      },
    })

    const icon = wrapper.get('[data-platform-icon]')
    expect(icon.attributes('data-platform')).toBe(platform)
    expect(icon.attributes('data-size')).toBe('xl')
    for (const themeClass of platformGroupBadgeClass(platform as GroupPlatform | undefined).split(' ')) {
      expect(wrapper.classes()).toContain(themeClass)
    }
    expect(wrapper.classes()).toContain('size-11')
  })

  it('keeps an accessible normalized platform label', () => {
    const wrapper = mount(AvailableChannelBrandIcon, {
      props: { platform: 'Gemini' },
      global: { stubs: { PlatformIcon: true } },
    })

    expect(wrapper.attributes('aria-label')).toBe('Gemini')
    expect(wrapper.attributes('data-brand')).toBe('gemini')
  })
})
