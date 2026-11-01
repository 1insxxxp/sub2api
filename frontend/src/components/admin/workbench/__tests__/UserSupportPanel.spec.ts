import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import UserSupportPanel from '../UserSupportPanel.vue'

const { lookup, refresh, revoke, auth } = vi.hoisted(() => ({
  lookup: vi.fn(), refresh: vi.fn(), revoke: vi.fn(),
  auth: { user: { id: 5, role: 'sub_admin' }, token: 'first-session', isAdmin: false, isSubAdmin: true },
}))
vi.mock('@/api/admin/userSupport', () => ({ lookup }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => reactive(auth) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/composables/useUserSupportAccess', () => ({
  useUserSupportAccess: () => ({ canAccess: ref(true), refreshing: ref(false), refresh, revoke }),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))

const result = () => ({
  user: { id: 7147, email: 'customer@example.com', balance: 17.5 },
  trial: { status: 'granted', amount: 15, legacy: false, granted_at: '2026-10-31T04:00:00Z' },
  usage: { status: 'ok', items: Array.from({ length: 10 }, (_, i) => ({
    id: 100 + i, created_at: '2026-10-31T04:00:00Z', model: 'gemini-4-argon',
    group_name: i === 1 ? '' : 'Gemini <VIP>',
    input_tokens: 200 + i, output_tokens: 12, cache_read_tokens: 0,
    charged_amount: 0.1, duration_ms: 900, first_token_ms: 200, status: 'completed', stream: true,
    account: { name: 'SECRET-UPSTREAM' }, upstream_model: 'https://secret.invalid/provider',
  })) },
  errors: { status: 'ok', items: Array.from({ length: 10 }, (_, i) => ({
    id: 200 + i, created_at: '2026-10-31T04:00:00Z', model: 'gemini-4-argon', category: 'service_unavailable',
    group_name: i === 1 ? '' : 'Gemini <VIP>', status_code: i === 1 ? null : 502, error_kind: 'upstream',
    message: 'https://secret.invalid sk-secret SECRET-UPSTREAM',
  })) },
})
const createView = () => mount(UserSupportPanel, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } } })

beforeEach(() => { vi.clearAllMocks(); auth.token = 'first-session'; refresh.mockResolvedValue(true); lookup.mockResolvedValue(result()) })

describe('user support panel', () => {
  it('renders error group, recorded HTTP status and a fixed error-kind label', async () => {
    const view = createView()
    try {
      await flushPromises()
      await view.get('[data-test="support-account"]').setValue('7147')
      await view.get('form').trigger('submit'); await flushPromises()
      const rows = view.findAll('[data-test="support-error-row"]')
      expect(rows[0].get('td[data-label="userSupport.columns.group"]').text()).toBe('Gemini <VIP>')
      expect(rows[0].find('vip').exists()).toBe(false)
      expect(rows[0].get('td[data-label="userSupport.columns.httpStatus"]').text()).toBe('502')
      expect(rows[0].get('td[data-label="userSupport.columns.errorKind"]').text()).toBe('userSupport.errorKinds.upstream')
      expect(rows[0].get('td[data-label="userSupport.columns.error"]').text()).toBe('userSupport.categories.service_unavailable')
      expect(rows[1].get('td[data-label="userSupport.columns.group"]').text()).toBe('—')
      expect(rows[1].get('td[data-label="userSupport.columns.httpStatus"]').text()).toBe('—')
    } finally { view.unmount() }
  })

  it('does not render arbitrary error kinds or invalid HTTP status values', async () => {
    const data = result()
    data.errors.items = [0, 99, 600, 429.5, null].map((status, i) => ({
      ...data.errors.items[i], status_code: status, error_kind: 'https://secret.invalid sk-secret',
    }))
    lookup.mockResolvedValue(data)
    const view = createView()
    try {
      await flushPromises()
      await view.get('[data-test="support-account"]').setValue('7147')
      await view.get('form').trigger('submit'); await flushPromises()
      for (const row of view.findAll('[data-test="support-error-row"]')) {
        expect(row.get('td[data-label="userSupport.columns.httpStatus"]').text()).toBe('—')
        expect(row.get('td[data-label="userSupport.columns.errorKind"]').text()).toBe('userSupport.errorKinds.unknown')
      }
      expect(view.html()).not.toMatch(/secret\.invalid|sk-secret|SECRET-UPSTREAM/)
    } finally { view.unmount() }
  })

  it('only grants after explicit submit, and renders two bounded safe lists', async () => {
    const view = createView()
    await flushPromises()
    expect(lookup).not.toHaveBeenCalled()
    await view.get('[data-test="support-account"]').setValue(' customer@example.com ')
    expect(lookup).not.toHaveBeenCalled()
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(lookup).toHaveBeenCalledWith('customer@example.com')
    expect(view.findAll('[data-test="support-usage-row"]')).toHaveLength(10)
    expect(view.findAll('[data-test="support-error-row"]')).toHaveLength(10)
    expect(view.text()).toContain('customer@example.com')
    expect(view.text()).toContain('userSupport.trial.granted')
    const rows = view.findAll('[data-test="support-usage-row"]')
    expect(rows[0].get('td[data-label="userSupport.columns.group"]').text()).toBe('Gemini <VIP>')
    expect(rows[0].find('vip').exists()).toBe(false)
    expect(rows[1].get('td[data-label="userSupport.columns.group"]').text()).toBe('—')
    expect(view.html()).not.toMatch(/SECRET-UPSTREAM|secret\.invalid|sk-secret/)
    view.unmount()
  })

  it('uses fixed error text without rendering server messages', async () => {
    lookup.mockRejectedValue({ status: 502, message: 'https://secret.invalid sk-secret', metadata: { account: 'SECRET-UPSTREAM' } })
    const view = createView()
    await flushPromises()
    await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.text()).toContain('userSupport.errors.unavailable')
    expect(view.html()).not.toMatch(/secret\.invalid|sk-secret|SECRET-UPSTREAM/)
    view.unmount()
  })

  it('shows historical claims as already received, and clears stale customer data on input change', async () => {
    const data = result(); data.trial.status = 'already_received'; data.trial.legacy = true
    lookup.mockResolvedValue(data)
    const view = createView()
    await flushPromises()
    await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit'); await flushPromises()
    expect(view.text()).toContain('userSupport.trial.alreadyReceived')
    await view.get('[data-test="support-account"]').setValue('other@example.com')
    expect(view.text()).not.toContain('customer@example.com')
    view.unmount()
  })

  it('does not call failed record queries an empty history or undo a granted trial', async () => {
    const data = result(); data.usage.status = 'unavailable'; data.usage.items = []
    lookup.mockResolvedValue(data)
    const view = createView()
    await flushPromises()
    await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit'); await flushPromises()
    expect(view.get('[data-test="support-usage"]').text()).toContain('userSupport.recordsUnavailable')
    expect(view.get('[data-test="support-usage"]').text()).not.toContain('userSupport.noUsage')
    expect(view.text()).toContain('userSupport.trial.granted')
    view.unmount()
  })

  it('revokes permission and clears all results on a forbidden response', async () => {
    lookup.mockRejectedValue({ status: 403, code: 'FORBIDDEN', message: 'secret' })
    const view = createView(); await flushPromises()
    await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit'); await flushPromises()
    expect(revoke).toHaveBeenCalled()
    expect(view.emitted('access-denied')).toHaveLength(1)
    expect(view.text()).toContain('userSupport.errors.forbidden')
    expect(view.find('[data-test="support-result"]').exists()).toBe(false)
    view.unmount()
  })

  it('ignores duplicate submits while a lookup is in progress', async () => {
    let complete!: (data: ReturnType<typeof result>) => void
    lookup.mockReturnValueOnce(new Promise(resolve => { complete = resolve }))
    const view = createView(); await flushPromises()
    await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit')
    await view.get('form').trigger('submit')
    expect(lookup).toHaveBeenCalledTimes(1)
    expect(view.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    complete(result()); await flushPromises()
    expect(view.find('[data-test="support-result"]').exists()).toBe(true)
    view.unmount()
  })

  it('clears the customer snapshot and restores access after same-user token refresh', async () => {
    const view = createView(); await flushPromises()
    await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit'); await flushPromises()
    expect(view.find('[data-test="support-result"]').exists()).toBe(true)
    useAuthStore().token = 'refreshed-session'
    await flushPromises()
    expect(refresh).toHaveBeenCalledTimes(2)
    expect(view.find('[data-test="support-result"]').exists()).toBe(false)
    expect(view.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    expect(view.text()).not.toContain('userSupport.errors.forbidden')
    view.unmount()
  })
})
