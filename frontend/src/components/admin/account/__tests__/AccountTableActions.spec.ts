import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountTableActions from '../AccountTableActions.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('AccountTableActions', () => {
  it('keeps refresh and create accessible when their labels are compact', async () => {
    const wrapper = mount(AccountTableActions, { props: { loading: false } })
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await wrapper.get('button[aria-label="admin.accounts.createAccount"]').trigger('click')
    expect(wrapper.get('button[aria-label="admin.accounts.createAccount"]').text()).toContain('common.create')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('create')).toHaveLength(1)

    await wrapper.setProps({ loading: true })
    expect(wrapper.get('button[aria-label="common.refresh"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('button[aria-label="admin.accounts.createAccount"]').attributes('disabled')).toBeUndefined()
  })

  it('preserves every extension slot and its control order', () => {
    const wrapper = mount(AccountTableActions, {
      props: { loading: false },
      slots: {
        before: '<button>before</button>',
        after: '<button>after refresh</button>',
        beforeCreate: '<button>before create</button>',
        afterCreate: '<button>after create</button>'
      }
    })
    expect(wrapper.findAll('button').map(button => button.attributes('aria-label') || button.text())).toEqual([
      'before', 'common.refresh', 'after refresh', 'before create', 'admin.accounts.createAccount', 'after create'
    ])
  })
})
