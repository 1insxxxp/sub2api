import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
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
