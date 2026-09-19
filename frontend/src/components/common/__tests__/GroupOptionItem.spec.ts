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
  it.each(['chat', 'image', 'airp'])('shows the configured %s tag without replacing the group name', (tag) => {
    const wrapper = mountOption({ tag })
    const badge = wrapper.get('[data-test="group-tag"]')
    expect(badge.text()).toBe(`common.groupTags.${tag}`)
    expect(badge.classes()).toContain('group-option-tag')
    expect(wrapper.text()).toContain('Test Group')
  })

  it('does not show a tag for an untagged group', () => {
    expect(mountOption({ tag: '' }).find('[data-test="group-tag"]').exists()).toBe(false)
  })

  it('passes a custom label and color through to the corner badge', () => {
    const wrapper = mountOption({ tag: '专属线路', tagColor: '#16A34A' })
    const badge = wrapper.get('[data-test="group-tag"]')
    expect(badge.text()).toBe('专属线路')
    expect(badge.attributes('style')).toContain('22, 163, 74')
  })

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
    const descriptionElement = wrapper.get('[data-test="group-option-description"]')
    expect(descriptionElement.text()).toBe(description)
    expect(descriptionElement.classes()).toContain('whitespace-pre-line')
    expect(descriptionElement.classes()).toContain('[overflow-wrap:anywhere]')
    expect(descriptionElement.classes()).toContain('line-clamp-3')
    expect(descriptionElement.attributes('title')).toBe(description)
  })

  it('keeps the complete group name available alongside its description and rate', () => {
    const wrapper = mountOption({
      name: '余额 [Pro稳定号池] 综合低至 ¥0.24 / 刀 very-long-unbroken-group-name',
      description: 'A production group description',
      rateMultiplier: 1.5,
    })

    const name = wrapper.get('[data-test="group-option-name"]')
    expect(name.text()).toBe('余额 [Pro稳定号池] 综合低至 ¥0.24 / 刀 very-long-unbroken-group-name')
    expect(name.attributes('title')).toBe(name.text())
    expect(name.classes()).not.toContain('truncate')
    expect(wrapper.get('[data-test="group-option-rate"]').text()).toContain('1.5x')
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

    expect(wrapper.get('[data-test="group-option-name"]').classes()).toContain('font-semibold')
  })
})
