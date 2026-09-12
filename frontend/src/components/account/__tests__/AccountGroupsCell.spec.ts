import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { Group } from '@/types'
import AccountGroupsCell from '../AccountGroupsCell.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))

const groups = Array.from({ length: 5 }, (_, index) => ({
  id: index + 1,
  name: `按量【国模渠道】手机端应该完整显示的长分组名称-${index + 1}`,
  platform: 'openai',
  subscription_type: 'standard',
  rate_multiplier: 1
})) as Group[]

describe('AccountGroupsCell mobile names', () => {
  it('does not crop long names or wrapped rows on mobile', () => {
    const wrapper = mount(AccountGroupsCell, { props: { groups: groups.slice(0, 2) } })

    expect(wrapper.classes()).toContain('max-w-full')
    const list = wrapper.get('.flex.flex-wrap')
    expect(list.classes()).not.toContain('max-h-14')
    expect(list.classes()).not.toContain('overflow-hidden')
    expect(list.classes()).toContain('sm:max-h-14')
    for (const name of wrapper.findAll('[data-test="group-badge-name"]')) {
      expect(name.classes()).toContain('whitespace-normal')
      expect(name.classes()).not.toContain('truncate')
      expect(name.element.parentElement?.classList.contains('max-w-24')).toBe(false)
    }
    expect(wrapper.text()).toContain(groups[0]!.name)
    expect(wrapper.text()).toContain(groups[1]!.name)
    wrapper.unmount()
  })

  it('shows all long names in a viewport-constrained overflow popover', async () => {
    const wrapper = mount(AccountGroupsCell, {
      props: { groups },
      global: { stubs: { teleport: true } }
    })

    await wrapper.get('button').trigger('click')
    const popover = wrapper.get('.fixed.z-50')
    expect(popover.classes()).toContain('max-w-[calc(100vw-1rem)]')
    const names = popover.findAll('[data-test="group-badge-name"]')
    expect(names.map((name) => name.text())).toEqual(groups.map((group) => group.name))
    for (const name of names) {
      expect(name.classes()).toContain('[overflow-wrap:anywhere]')
      expect(name.classes()).not.toContain('truncate')
    }
    wrapper.unmount()
  })
})
