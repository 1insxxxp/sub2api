import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountTableFilters from '../AccountTableFilters.vue'
import Select from '@/components/common/Select.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

const filters = {
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  privacy_mode: 'training_off',
  group: '42',
  search: 'production'
}

function mountFilters() {
  return mount(AccountTableFilters, {
    props: { filters, groups: [] },
    global: { stubs: { Select: true } }
  })
}

describe('AccountTableFilters', () => {
  it('keeps only the five filter controls in the disclosure row', () => {
    const wrapper = mountFilters()
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.findAllComponents(Select)).toHaveLength(5)
    expect(wrapper.findAllComponents(Select).map(select => select.props('ariaLabel'))).toEqual([
      'admin.accounts.allPlatforms',
      'admin.accounts.allTypes',
      'admin.accounts.allStatus',
      'admin.accounts.allPrivacyModes',
      'admin.accounts.allGroups'
    ])
  })

  it.each([
    ['platform', 'anthropic'],
    ['type', 'setup-token'],
    ['status', 'temp_unschedulable'],
    ['privacy_mode', '__unset__'],
    ['group', 'ungrouped']
  ])('updates %s without dropping other filters or search', async (key, value) => {
    const wrapper = mountFilters()
    const index = ['platform', 'type', 'status', 'privacy_mode', 'group'].indexOf(key)
    const select = wrapper.findAllComponents(Select)[index]
    expect(select.props('modelValue')).toBe(filters[key as keyof typeof filters])
    select.vm.$emit('update:modelValue', value)
    select.vm.$emit('change', value)

    expect(wrapper.emitted('update:filters')).toEqual([[{ ...filters, [key]: value }]])
    expect(wrapper.emitted('change')).toHaveLength(1)
    expect(filters.search).toBe('production')
  })
})
