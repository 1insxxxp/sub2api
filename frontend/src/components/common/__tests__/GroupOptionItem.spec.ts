import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import GroupOptionItem from '../GroupOptionItem.vue'
import GroupBadge from '../GroupBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ cachedPublicSettings: null }),
}))

const mountOption = (props: Record<string, unknown>) =>
  mount(GroupOptionItem, {
    props: {
      name: 'Test Group',
      platform: 'openai',
      ...props,
    },
    global: {
      stubs: {
        GroupBadge: {
          template: '<span>{{ name }}</span>',
          props: {
            name: String,
            wrapName: Boolean,
          },
        },
      },
    },
  })

const mountBadge = (wrapName: boolean) =>
  mount(GroupBadge, {
    props: {
      name: '余额 [Pro稳定号池] 综合低至 ¥0.24 / 刀 very-long-unbroken-group-name',
      platform: 'openai',
      showRate: false,
      wrapName,
    },
    global: {
      stubs: {
        PlatformIcon: true,
      },
    },
  })

const mountActualOption = (props: Record<string, unknown> = {}) =>
  mount(GroupOptionItem, {
    props: {
      name: 'Test Group',
      platform: 'openai',
      ...props,
    },
    global: {
      stubs: {
        PlatformIcon: true,
      },
    },
  })

describe('GroupOptionItem', () => {
  it('formats custom rate multipliers with at most two decimals', () => {
    const wrapper = mountOption({
      rateMultiplier: 1,
      userRateMultiplier: 0.0635,
    })

    expect(wrapper.text()).toContain('1x')
    expect(wrapper.text()).toContain('0.06x')
    expect(wrapper.text()).not.toContain('0.0635x')
  })

  it('trims trailing zeros from standard rate labels', () => {
    const wrapper = mountOption({ rateMultiplier: 1.5 })

    expect(wrapper.text()).toContain('1.5x')
    expect(wrapper.text()).not.toContain('1.50x')
  })

  it('applies multiline and overflow-safe text styles', () => {
    const description = 'First section\nvery-long-unbroken-description-value-that-must-not-overflow'
    const wrapper = mountOption({ description })
    const descriptionElement = wrapper
      .findAll('span')
      .find((element) => element.text() === description)

    expect(descriptionElement).toBeDefined()
    expect(descriptionElement?.classes()).toContain('whitespace-pre-line')
    expect(descriptionElement?.classes()).toContain('[overflow-wrap:anywhere]')
    expect(descriptionElement?.classes()).toContain('line-clamp-3')
    expect(wrapper.find('[title]').attributes('title')).toBe(description)
  })

  it('stacks mobile content and enables full group-name wrapping', () => {
    const wrapper = mountOption({
      name: '余额 [Pro稳定号池] 综合低至 ¥0.24 / 刀 very-long-unbroken-group-name',
      description: 'A production group description',
      rateMultiplier: 1.5,
    })

    const layout = wrapper.get('[data-test="group-option-layout"]')
    expect(layout.classes()).toContain('flex-col')
    expect(layout.classes()).toContain('sm:flex-row')

    const badge = wrapper.getComponent(GroupBadge)
    expect(badge.props('wrapName')).toBe(true)
  })

  it('wraps enabled badge names on mobile and truncates them on desktop', () => {
    const wrapper = mountBadge(true)
    const name = wrapper.get('[data-test="group-badge-name"]')

    expect(name.classes()).toContain('whitespace-normal')
    expect(name.classes()).toContain('[overflow-wrap:anywhere]')
    expect(name.classes()).toContain('sm:truncate')
  })

  it('keeps the default badge name compact at every viewport size', () => {
    const wrapper = mountBadge(false)
    const name = wrapper.get('[data-test="group-badge-name"]')

    expect(name.classes()).toContain('truncate')
    expect(name.classes()).not.toContain('whitespace-normal')
  })

  it('uses a stable name hook for semibold option labels', () => {
    const wrapper = mountActualOption()

    expect(wrapper.get('.groupOptionItemBadge').classes()).toContain('font-semibold')
  })
})
