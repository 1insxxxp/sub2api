import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { computed, h, reactive } from 'vue'
import { useAuthStore } from '@/stores/auth'
import AdminWorkbenchView from '../AdminWorkbenchView.vue'

const { state, route, replace, refresh, getWorkbenchAccess, balanceMounted } = vi.hoisted(() => ({
  state: { id: 7, role: 'sub_admin', enabled: false, token: 'session-one' },
  route: { query: {} as Record<string, string>, fullPath: '/admin/workbench' },
  replace: vi.fn(), refresh: vi.fn(), getWorkbenchAccess: vi.fn(), balanceMounted: vi.fn(),
}))
vi.mock('@/api/admin/userSupport', () => ({ getWorkbenchAccess }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => { const current = reactive(state); return { get user() { return { id: current.id, role: current.role } }, get token() { return current.token }, get isAdmin() { return current.role === 'admin' } } } }))
vi.mock('@/composables/useUserSupportAccess', () => ({ useUserSupportAccess: () => ({ canAccess: computed(() => state.role === 'admin' || (state.role === 'sub_admin' && state.enabled)), refresh }) }))
vi.mock('vue-router', async original => ({ ...(await original<typeof import('vue-router')>()), useRoute: () => reactive(route), useRouter: () => ({ replace }) }))
vi.mock('vue-i18n', async original => ({ ...(await original<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))
const createView = () => mount(AdminWorkbenchView, { global: { stubs: {
  AppLayout: { template: '<main data-test="layout"><slot /></main>' }, Icon: true,
  SubAdminCommissionPanel: { template: '<div data-test="commission" />' },
  BalanceTransferPanel: { setup() { const user = useAuthStore().user; const owner = `REDEEM-CODE-${user?.id}-${user?.role}`; balanceMounted(); return () => h('div', [h('span', { 'data-test': 'cached-balance-owner' }, owner)]) } }, AdminAffiliateLeaderboardPanel: true,
  UserSupportPanel: { template: '<div data-test="support-panel" />' },
} } })
beforeEach(() => { state.id = 7; state.token = 'session-one'; state.role = 'sub_admin'; state.enabled = false; route.query = {}; route.fullPath = '/admin/workbench'; vi.clearAllMocks(); refresh.mockResolvedValue(false); getWorkbenchAccess.mockImplementation(async () => ({ commission: state.role === 'admin', affiliate_leaderboard: state.role === 'admin', user_support: state.role === 'admin' || state.enabled, balance_transfer: true })) })

describe('User Support inside admin workbench', () => {
  it('shows the support tab to the primary admin without another layout or page title', async () => {
    state.role = 'admin'
    const view = createView(); await flushPromises()
    expect(view.find('[data-test="workbench-tab-user-support"]').exists()).toBe(true)
    await view.get('[data-test="workbench-tab-user-support"]').trigger('click')
    expect(view.find('[data-test="support-panel"]').exists()).toBe(true)
    expect(view.findAll('[data-test="layout"]')).toHaveLength(1)
    expect(view.find('h1').exists()).toBe(false)
    expect(replace).toHaveBeenCalledWith({ query: { tab: 'user-support' } })
    view.unmount()
  })
  it('rechecks tab permissions when navigating within the workbench', async () => {
    const view = createView(); await flushPromises()
    const calls = getWorkbenchAccess.mock.calls.length
    reactive(route).fullPath = '/admin/workbench?tab=balance-transfer'
    await flushPromises()
    expect(getWorkbenchAccess).toHaveBeenCalledTimes(calls + 1)
    view.unmount()
  })
  it('requires explicit permission for sub-admins', async () => {
    const denied = createView(); await flushPromises()
    expect(denied.find('[data-test="workbench-tab-user-support"]').exists()).toBe(false)
    expect(denied.find('[data-test="workbench-tab-commission"]').exists()).toBe(false)
    expect(denied.find('[data-test="workbench-tab-affiliate-leaderboard"]').exists()).toBe(false)
    denied.unmount()
    state.enabled = true; route.query = { tab: 'user-support' }
    const allowed = createView(); await flushPromises()
    expect(allowed.find('[data-test="support-panel"]').exists()).toBe(true)
    expect(allowed.find('[data-test="commission"]').exists()).toBe(false)
    expect(getWorkbenchAccess).toHaveBeenCalled()
    allowed.unmount()
  })
  it('keeps support-only staff out of earnings and ranking panels', async () => {
    state.enabled = true
    const view = createView(); await flushPromises()
    expect(view.findAll('[role="tab"]').map(tab => tab.attributes('data-test'))).toEqual(['workbench-tab-balance-transfer', 'workbench-tab-user-support'])
    expect(view.find('[data-test="commission"]').exists()).toBe(false)
    await view.get('[data-test="workbench-tab-balance-transfer"]').trigger('keydown', { key: 'End' })
    await flushPromises()
    expect(view.get('[data-test="workbench-tab-user-support"]').attributes('aria-selected')).toBe('true')
    view.unmount()
  })

  it('fails closed for all tabs when the access request fails', async () => {
    getWorkbenchAccess.mockRejectedValue({ status: 403, message: 'https://secret.invalid/sk-secret' })
    const view = createView(); await flushPromises()
    expect(view.findAll('[role="tab"]')).toHaveLength(0)
    expect(view.find('[data-test="cached-balance-owner"]').exists()).toBe(false)
    expect(view.text()).toContain('userSupport.errors.accessUnavailable')
    expect(view.html()).not.toContain('secret.invalid')
    view.unmount()
  })

  it.each([
    { change: { id: 8 }, expected: 'REDEEM-CODE-8-sub_admin' },
    { change: { role: 'admin' }, expected: 'REDEEM-CODE-7-admin' },
    { change: { token: 'session-two' }, expected: 'REDEEM-CODE-7-sub_admin' },
  ])('destroys cached redeem-code panels when actor state changes: $change', async ({ change, expected }) => {
    route.query = { tab: 'balance-transfer' }
    const view = createView(); await flushPromises()
    expect(view.get('[data-test="cached-balance-owner"]').text()).toBe('REDEEM-CODE-7-sub_admin')
    Object.assign(reactive(state), change)
    await flushPromises()
    expect(view.get('[data-test="cached-balance-owner"]').text()).toBe(expected)
    expect(balanceMounted).toHaveBeenCalledTimes(2)
    view.unmount()
  })

  it('rejects ordinary users even with an enabled response or a support query', async () => {
    state.role = 'user'; state.enabled = true; route.query = { tab: 'user-support' }; refresh.mockResolvedValue(true)
    const view = createView(); await flushPromises()
    expect(view.find('[data-test="workbench-tab-user-support"]').exists()).toBe(false)
    expect(view.find('[data-test="support-panel"]').exists()).toBe(false)
    view.unmount()
  })
})
