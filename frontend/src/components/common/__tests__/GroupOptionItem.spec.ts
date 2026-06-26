import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import GroupOptionItem from '../GroupOptionItem.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const mountOption = (props: Record<string, unknown>) =>
  mount(GroupOptionItem, {
    props: {
      name: 'Test Group',
      platform: 'openai',
      ...props
    },
    global: {
      stubs: {
        GroupBadge: {
          template: '<span>{{ name }}</span>',
          props: ['name']
        }
      }
    }
  })

describe('GroupOptionItem', () => {
  it('formats custom rate multipliers with at most two decimals', () => {
    const wrapper = mountOption({
      rateMultiplier: 1,
      userRateMultiplier: 0.0635
    })

    expect(wrapper.text()).toContain('1x')
    expect(wrapper.text()).toContain('0.06x')
    expect(wrapper.text()).not.toContain('0.0635x')
  })

  it('trims trailing zeros from standard rate labels', () => {
    const wrapper = mountOption({
      rateMultiplier: 1.5
    })

    expect(wrapper.text()).toContain('1.5x')
    expect(wrapper.text()).not.toContain('1.50x')
  })
})
