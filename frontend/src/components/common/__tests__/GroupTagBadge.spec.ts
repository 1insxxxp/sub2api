import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import GroupTagBadge from '../GroupTagBadge.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('GroupTagBadge', () => {
  it('renders custom labels as text and applies the selected color', () => {
    const wrapper = mount(GroupTagBadge, { props: { tag: '<b>高速</b>', color: '#16A34A' } })
    expect(wrapper.text()).toBe('<b>高速</b>')
    expect(wrapper.find('b').exists()).toBe(false)
    expect(wrapper.attributes('style')).toContain('22, 163, 74')
  })

  it.each(['chat', 'image', 'airp'])('preserves legacy %s labels', tag => {
    const wrapper = mount(GroupTagBadge, { props: { tag } })
    expect(wrapper.text()).toBe(`common.groupTags.${tag}`)
  })

  it('keeps custom colors readable on light and dark backgrounds', () => {
    const light = mount(GroupTagBadge, { props: { tag: 'light', color: '#FFFFFF' } })
    const dark = mount(GroupTagBadge, { props: { tag: 'dark', color: '#000000' } })
    expect((light.element as HTMLElement).style.color).toBe('rgb(17, 24, 39)')
    expect((dark.element as HTMLElement).style.color).toBe('rgb(255, 255, 255)')
  })

  it('ignores malformed colors and hides empty labels', () => {
    const wrapper = mount(GroupTagBadge, { props: { tag: '自定义', color: 'url(https://invalid.test)' } })
    expect(wrapper.text()).toBe('自定义')
    expect(wrapper.attributes('style')).not.toContain('url(')
    expect(mount(GroupTagBadge, { props: { tag: '' } }).find('[data-test="group-tag"]').exists()).toBe(false)
  })
})
