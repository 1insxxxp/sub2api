import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { platformGroupBadgeClass } from '@/utils/platformColors'
import GroupBadge from '../GroupBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null })
}))

const mountBadge = (props: Record<string, unknown>) =>
  mount(GroupBadge, {
    props: {
      name: 'Test Group',
      platform: 'openai',
      ...props
    },
    global: {
      stubs: {
        PlatformIcon: true
      }
    }
  })

describe('GroupBadge', () => {
  it('sources platform colors from the shared visual theme', () => {
    const componentDirectory = resolve(dirname(fileURLToPath(import.meta.url)), '..')

    expect(readFileSync(resolve(componentDirectory, 'GroupBadge.vue'), 'utf8'))
      .toContain("from '@/utils/platformColors'")
  })

  it.each([
    ['anthropic', ['bg-amber-50', 'text-amber-700']],
    ['openai', ['bg-green-50', 'text-green-700']],
    ['gemini', ['bg-sky-50', 'text-sky-700']],
    ['antigravity', ['bg-fuchsia-50', 'text-fuchsia-700']],
    ['grok', ['bg-zinc-100', 'text-zinc-700']],
    ['composite', ['bg-cyan-50', 'text-cyan-800']],
    ['unknown', ['bg-emerald-100', 'text-emerald-700']],
  ])('keeps the normal %s platform theme', (platform, expectedClasses) => {
    const wrapper = mountBadge({ platform, showRate: false })

    expect(wrapper.classes()).toEqual(expect.arrayContaining(expectedClasses))
    expect(wrapper.classes()).toEqual(expect.arrayContaining(platformGroupBadgeClass(platform).split(' ')))
  })

  it('formats visible rate multipliers with at most two decimals', () => {
    const wrapper = mountBadge({
      rateMultiplier: 1,
      userRateMultiplier: 0.0635
    })

    expect(wrapper.text()).toContain('1x')
    expect(wrapper.text()).toContain('0.06x')
    expect(wrapper.text()).not.toContain('0.0635x')
  })

  it('trims trailing zeros from standard rate labels', () => {
    const wrapper = mountBadge({
      rateMultiplier: 1.5
    })

    expect(wrapper.text()).toContain('1.5x')
    expect(wrapper.text()).not.toContain('1.50x')
  })
})
