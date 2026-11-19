import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import AccountActionMenu from '../AccountActionMenu.vue'
import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'
import type { Account } from '@/types'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
describe('acceptance entry permissions', () => {
  it('only enables the full-admin single account acceptance entry and keeps connection test', async () => {
    const account = { id: 7, platform: 'gemini', type: 'apikey' } as Account
    const wrapper = mount(AccountActionMenu, { props: { show: true, account, anchorRect: new DOMRect(), canAccept: false }, global: { stubs: { teleport: true, Icon: true } } })
    expect(wrapper.find('[data-testid="account-acceptance"]').exists()).toBe(false)
    await wrapper.setProps({ canAccept: true })
    await wrapper.get('[data-testid="account-acceptance"]').trigger('click')
    expect(wrapper.emitted('acceptance')).toEqual([[account]])
    expect(wrapper.text()).toContain('admin.accounts.testConnection')
  })
  it('only offers batch acceptance to full admins with selected accounts', async () => {
    const wrapper = mount(AccountBulkActionsBar, { props: { selectedIds: [], totalResults: 2, selectingAll: false, allResultsSelected: false, canAccept: true } })
    expect(wrapper.find('[data-testid="bulk-acceptance"]').exists()).toBe(false)
    await wrapper.setProps({ selectedIds: [7] })
    await wrapper.get('[data-testid="bulk-acceptance"]').trigger('click')
    expect(wrapper.emitted('acceptance')).toHaveLength(1)
    await wrapper.setProps({ canAccept: false })
    expect(wrapper.find('[data-testid="bulk-acceptance"]').exists()).toBe(false)
  })
})
