import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { AdminGroup } from '@/types'
import GroupSelector from '../GroupSelector.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))

const group = {
  id: 1,
  name: '按量【国模渠道】这是需要在手机端完整显示的很长分组名称',
  platform: 'openai',
  subscription_type: 'standard',
  rate_multiplier: 1,
  account_count: 3
} as AdminGroup

describe('GroupSelector mobile names', () => {
  it('uses one column and wraps the full group name on mobile', () => {
    const wrapper = mount(GroupSelector, { props: { modelValue: [], groups: [group] } })

    expect(wrapper.get('.grid').classes()).toContain('grid-cols-1')
    expect(wrapper.get('.grid').classes()).toContain('sm:grid-cols-2')
    const name = wrapper.get('[data-test="group-badge-name"]')
    expect(name.text()).toBe(group.name)
    expect(name.classes()).toContain('whitespace-normal')
    expect(name.classes()).toContain('[overflow-wrap:anywhere]')
    expect(name.classes()).not.toContain('truncate')
    wrapper.unmount()
  })

  it('keeps checkbox selection working with long names', async () => {
    const wrapper = mount(GroupSelector, { props: { modelValue: [], groups: [group] } })

    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.emitted('update:modelValue')).toEqual([[[group.id]]])
    wrapper.unmount()
  })
})
