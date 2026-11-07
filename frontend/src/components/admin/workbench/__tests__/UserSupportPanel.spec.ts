import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { computed, reactive, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import UserSupportPanel from '../UserSupportPanel.vue'

const { lookup, search, grant, refresh, revoke, auth, permission } = vi.hoisted(() => ({
  lookup: vi.fn(), search: vi.fn(), grant: vi.fn(), refresh: vi.fn(), revoke: vi.fn(),
  auth: { user: { id: 5, role: 'sub_admin' }, token: 'first-session' }, permission: { enabled: true },
}))
vi.mock('@/api/admin/userSupport', () => ({ lookup, search, grant }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => reactive(auth) }))
vi.mock('@/composables/useUserSupportAccess', () => ({ useUserSupportAccess: () => ({ canAccess: computed(() => reactive(permission).enabled), refreshing: ref(false), refresh, revoke }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))
const originalMessage = 'Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.'
const result = () => ({
  user: { id: 7147, email: 'customer@example.com', balance: 17.5 },
  trial: { status: 'eligible', amount: 15, legacy: false, granted_at: '' },
  usage: { status: 'ok', items: Array.from({ length: 12 }, (_, i) => ({ id: 100 + i, created_at: '2026-10-31T04:00:00Z', model: 'gemini-4-argon', group_name: i === 1 ? '' : 'Gemini <VIP>', input_tokens: 200 + i, output_tokens: 12, cache_read_tokens: 0, charged_amount: 0.1, duration_ms: 900, first_token_ms: 200, status: 'completed', stream: true, account: { name: 'SECRET-UPSTREAM' } })) },
  errors: { status: 'ok', items: Array.from({ length: 12 }, (_, i) => ({ id: 200 + i, created_at: '2026-10-31T04:00:00Z', model: 'gemini-4-argon', category: 'service_unavailable', group_name: i === 1 ? '' : 'Gemini <VIP>', status_code: i === 1 ? null : 502, error_kind: 'upstream', original_message: originalMessage, message: 'https://secret.invalid sk-secret SECRET-UPSTREAM' })) },
})
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(yes => { resolve = yes }); return { promise, resolve } }
const views: VueWrapper[] = []
function createView() { const view = mount(UserSupportPanel, { attachTo: document.body, global: { stubs: { Icon: true } } }); views.push(view); return view }
async function query(view: VueWrapper, value = '7147') { await flushPromises(); await view.get('[data-test="support-account"]').setValue(value); await view.get('form').trigger('submit'); await flushPromises() }
async function showErrors(view: VueWrapper) { await view.get('[data-test="support-tab-errors"]').trigger('click') }
beforeEach(() => {
  vi.resetAllMocks(); Object.assign(reactive(auth), { user: { id: 5, role: 'sub_admin' }, token: 'first-session' }); reactive(permission).enabled = true
  refresh.mockResolvedValue(true); search.mockResolvedValue({ items: [{ id: 7147, email: 'customer@example.com' }, { id: 81, email: 'second@example.com' }] }); lookup.mockResolvedValue(result())
  grant.mockResolvedValue({ user: { ...result().user, balance: 32.5 }, trial: { ...result().trial, status: 'granted' } })
})
afterEach(() => { views.splice(0).forEach(view => view.unmount()); document.body.innerHTML = ''; vi.useRealTimers() })

describe('support query and explicit credit grant', () => {
  it('debounces eligible email fragments and single numeric IDs without lookup or grant', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    await input.setValue('c'); await vi.advanceTimersByTimeAsync(250); expect(search).not.toHaveBeenCalled()
    await input.setValue('cu'); await vi.advanceTimersByTimeAsync(249); expect(search).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1); expect(search).toHaveBeenCalledWith('cu'); expect(lookup).not.toHaveBeenCalled(); expect(grant).not.toHaveBeenCalled()
    await input.setValue('7'); await vi.advanceTimersByTimeAsync(250); expect(search).toHaveBeenLastCalledWith('7')
  })
  it('discards stale suggestions and selects by numeric ID without issuing credit', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); const pending = deferred<{ items: { id: number; email: string }[] }>(); search.mockReturnValueOnce(pending.promise)
    const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    await input.setValue('old'); await vi.advanceTimersByTimeAsync(250); await input.setValue('new'); await vi.advanceTimersByTimeAsync(250)
    pending.resolve({ items: [{ id: 999, email: 'stale@example.com' }] }); await flushPromises(); expect(view.text()).not.toContain('stale@example.com')
    await view.get('[role="option"]').trigger('click'); await flushPromises(); expect(lookup).toHaveBeenCalledWith('7147'); expect(grant).not.toHaveBeenCalled()
    expect(view.find('[role="listbox"]').exists()).toBe(false); await vi.advanceTimersByTimeAsync(500); expect(search).toHaveBeenCalledTimes(2)
  })
  it('keeps a clicked non-first suggestion email visible while looking up its exact ID', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const selected = result(); selected.user = { id: 81, email: 'second@example.com', balance: 17.5 }; lookup.mockResolvedValue(selected)
    const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    await input.setValue('example'); await vi.advanceTimersByTimeAsync(250)
    await view.findAll('[role="option"]')[1].trigger('click'); await flushPromises()
    expect(lookup).toHaveBeenLastCalledWith('81'); expect(grant).not.toHaveBeenCalled()
    await input.trigger('blur'); await input.trigger('keydown', { key: 'Escape' }); await input.trigger('keydown', { key: 'Tab' })
    await view.get('form').trigger('submit'); await flushPromises()
    expect(lookup).toHaveBeenLastCalledWith('81'); expect((input.element as HTMLInputElement).value).toBe('second@example.com')
  })
  it('shows the keyboard-selected non-first email while querying its exact ID', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const selected = result(); selected.user = { id: 81, email: 'second@example.com', balance: 17.5 }; lookup.mockResolvedValue(selected)
    const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    await input.setValue('example'); await vi.advanceTimersByTimeAsync(250)
    await input.trigger('keydown', { key: 'ArrowDown' }); await input.trigger('keydown', { key: 'ArrowDown' }); await input.trigger('keydown', { key: 'Enter' }); await flushPromises()
    expect(lookup).toHaveBeenLastCalledWith('81'); expect((input.element as HTMLInputElement).value).toBe('second@example.com'); expect(grant).not.toHaveBeenCalled()
  })
  it('releases the selected ID when the user manually edits email or numeric text', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const selected = result(); selected.user = { id: 81, email: 'second@example.com', balance: 17.5 }; lookup.mockResolvedValue(selected)
    const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    for (const edited of ['customer@example.com', '7147']) {
      await input.setValue('example'); await vi.advanceTimersByTimeAsync(250)
      await view.findAll('[role="option"]')[1].trigger('click'); await flushPromises(); expect(lookup).toHaveBeenLastCalledWith('81')
      await input.setValue(edited); await view.get('form').trigger('submit'); await flushPromises()
      expect(lookup).toHaveBeenLastCalledWith(edited); expect((input.element as HTMLInputElement).value).toBe(edited)
    }
    expect(grant).not.toHaveBeenCalled()
  })
  it('reselects the same visible email by ID during an older lookup and ignores that old response', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const pending = deferred<ReturnType<typeof result>>()
    const selected = result(); selected.user = { id: 81, email: 'second@example.com', balance: 42.25 }
    lookup.mockReturnValueOnce(pending.promise).mockResolvedValueOnce(selected)
    const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    await input.setValue('second@example.com'); await view.get('form').trigger('submit'); await flushPromises()
    await input.trigger('focus'); await vi.advanceTimersByTimeAsync(250)
    await view.findAll('[role="option"]')[1].trigger('click'); await flushPromises()
    expect(lookup.mock.calls.map(call => call[0])).toEqual(['second@example.com', '81'])
    const old = result(); old.user = { ...selected.user, balance: 1 }; pending.resolve(old); await flushPromises()
    expect(view.text()).toContain('42.25'); expect((input.element as HTMLInputElement).value).toBe('second@example.com'); expect(grant).not.toHaveBeenCalled()
  })
  it('supports keyboard suggestions and Escape, Tab and outside-pointer dismissal', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); const view = createView(); await flushPromises(); const input = view.get('[data-test="support-account"]')
    await input.setValue('cu'); await vi.advanceTimersByTimeAsync(250); expect(input.attributes('role')).toBe('combobox'); expect(input.attributes('aria-expanded')).toBe('true')
    await input.trigger('keydown', { key: 'ArrowDown' }); expect(input.attributes('aria-activedescendant')).toBe(view.findAll('[role="option"]')[0].attributes('id'))
    await input.trigger('keydown', { key: 'ArrowDown' }); await input.trigger('keydown', { key: 'ArrowUp' }); await input.trigger('keydown', { key: 'Enter' }); await flushPromises()
    expect(lookup).toHaveBeenCalledWith('7147'); expect(grant).not.toHaveBeenCalled()
    for (const key of ['Escape', 'Tab']) { await input.setValue(`cu${key}`); await vi.advanceTimersByTimeAsync(250); await input.trigger('keydown', { key }); expect(input.attributes('aria-expanded')).toBe('false') }
    await input.setValue('other'); await vi.advanceTimersByTimeAsync(250); document.body.dispatchEvent(new Event('pointerdown', { bubbles: true })); await flushPromises(); expect(input.attributes('aria-expanded')).toBe('false')
  })
  it('selects the last suggestion on initial ArrowUp and scrolls only the suggestion list', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); const view = createView(); await flushPromises()
    const input = view.get('[data-test="support-account"]'); await input.setValue('cu'); await vi.advanceTimersByTimeAsync(250)
    const last = view.findAll('[role="option"]')[1]; const scroller = view.get('.support-suggestions').element
    Object.defineProperty(scroller, 'clientHeight', { value: 44 }); Object.defineProperty(last.element, 'offsetTop', { value: 44 }); Object.defineProperty(last.element, 'offsetHeight', { value: 44 })
    await input.trigger('keydown', { key: 'ArrowUp' })
    expect(input.attributes('aria-activedescendant')).toBe(last.attributes('id')); expect(scroller.scrollTop).toBe(44)
    await input.trigger('keydown', { key: 'Enter' }); await flushPromises()
    expect(lookup).toHaveBeenLastCalledWith('81'); expect((input.element as HTMLInputElement).value).toBe('second@example.com')
  })
  it('looks up without a grant and exposes a separate eligible action', async () => {
    const view = createView(); await query(view, ' customer@example.com '); expect(lookup).toHaveBeenCalledWith('customer@example.com'); expect(grant).not.toHaveBeenCalled()
    expect(view.get('[data-test="support-grant"]').text()).toBe('userSupport.trial.send'); expect(view.text()).toContain('userSupport.trial.eligibleShort')
    expect(view.findAll('[data-test="support-usage-row"]')).toHaveLength(10); expect(view.get('[data-test="support-errors"]').isVisible()).toBe(false); expect(view.html()).not.toMatch(/SECRET-UPSTREAM|secret\.invalid|sk-secret/)
  })
  it('grants once to the current result ID, updates balance and preserves usage', async () => {
    const pending = deferred<unknown>(); grant.mockReturnValueOnce(pending.promise); const view = createView(); await query(view)
    await view.get('[data-test="support-grant"]').trigger('click'); await view.get('[data-test="support-grant"]').trigger('click'); expect(grant).toHaveBeenCalledTimes(1); expect(grant).toHaveBeenCalledWith(7147)
    expect(view.get('[data-test="support-grant"]').attributes('disabled')).toBeDefined(); pending.resolve({ user: { ...result().user, balance: 32.5 }, trial: { ...result().trial, status: 'granted' } }); await flushPromises()
    expect(view.text()).toContain('32.50'); expect(view.text()).toContain('userSupport.trial.grantedShort'); expect(view.find('[data-test="support-grant"]').exists()).toBe(false); expect(view.findAll('[data-test="support-usage-row"]')).toHaveLength(10)
  })
  it('does not offer another grant for historical claims', async () => {
    const data = result(); data.trial.status = 'already_received'; data.trial.legacy = true; lookup.mockResolvedValue(data); const view = createView(); await query(view)
    expect(view.text()).toContain('userSupport.trial.receivedShort'); expect(view.text()).toContain('userSupport.trial.historicalShort'); expect(view.find('[data-test="support-grant"]').exists()).toBe(false)
    expect(view.get('.support-trial-outcome').attributes('title')).toContain('userSupport.trial.alreadyReceived')
    expect(view.get('.support-trial-outcome').attributes('title')).toContain('userSupport.trial.historical')
  })
  it('keeps the same customer grant disabled if a lookup is repeated while sending', async () => {
    const pending = deferred<unknown>(); grant.mockReturnValueOnce(pending.promise)
    const granted = result(); granted.user.balance = 32.5; granted.trial.status = 'granted'
    lookup.mockResolvedValueOnce(result()).mockResolvedValueOnce(result()).mockResolvedValueOnce(granted)
    const view = createView(); await query(view); await view.get('[data-test="support-grant"]').trigger('click')
    await view.get('form').trigger('submit'); await flushPromises()
    expect(view.get('[data-test="support-grant"]').attributes('disabled')).toBeDefined()
    await view.get('[data-test="support-grant"]').trigger('click'); expect(grant).toHaveBeenCalledTimes(1)
    pending.resolve({ user: result().user, trial: { ...result().trial, status: 'granted' } }); await flushPromises()
    expect(view.text()).toContain('32.50'); expect(view.text()).toContain('userSupport.trial.grantedShort')
    expect(view.find('[data-test="support-grant"]').exists()).toBe(false); expect(lookup).toHaveBeenCalledTimes(3)
  })
  it('refreshes a pre-grant lookup snapshot that arrives after the grant succeeds', async () => {
    const pendingGrant = deferred<unknown>(); const pendingLookup = deferred<ReturnType<typeof result>>()
    grant.mockReturnValueOnce(pendingGrant.promise)
    const granted = result(); granted.user.balance = 32.5; granted.trial.status = 'granted'
    lookup.mockResolvedValueOnce(result()).mockReturnValueOnce(pendingLookup.promise).mockResolvedValueOnce(granted)
    const view = createView(); await query(view); await view.get('[data-test="support-grant"]').trigger('click')
    await view.get('form').trigger('submit'); await flushPromises()
    pendingGrant.resolve({ user: { ...result().user, balance: 32.5 }, trial: granted.trial }); await flushPromises()
    pendingLookup.resolve(result()); await flushPromises()
    expect(view.text()).toContain('32.50'); expect(view.text()).toContain('userSupport.trial.grantedShort')
    expect(view.find('[data-test="support-grant"]').exists()).toBe(false); expect(lookup).toHaveBeenCalledTimes(3); expect(grant).toHaveBeenCalledTimes(1)
  })
  it('keeps the successful receipt if a post-grant refresh is temporarily unavailable', async () => {
    const pending = deferred<unknown>(); grant.mockReturnValueOnce(pending.promise)
    lookup.mockResolvedValueOnce(result()).mockResolvedValueOnce(result()).mockRejectedValueOnce({ status: 502, message: 'SECRET-UPSTREAM' })
    const view = createView(); await query(view); await view.get('[data-test="support-grant"]').trigger('click')
    await view.get('form').trigger('submit'); await flushPromises()
    pending.resolve({ user: { ...result().user, balance: 32.5 }, trial: { ...result().trial, status: 'granted' } }); await flushPromises()
    expect(view.text()).toContain('32.50'); expect(view.text()).toContain('userSupport.trial.grantedShort')
    expect(view.text()).toContain('userSupport.errors.refreshAfterGrant'); expect(view.find('[data-test="support-grant"]').exists()).toBe(false)
    expect(view.text()).not.toContain('SECRET-UPSTREAM')
  })
  it('ignores old grant responses after switching customers', async () => {
    const pending = deferred<unknown>(); grant.mockReturnValueOnce(pending.promise); const view = createView(); await query(view); await view.get('[data-test="support-grant"]').trigger('click')
    await view.get('[data-test="support-account"]').setValue('81'); expect(view.find('[data-test="support-result"]').exists()).toBe(false)
    const next = result(); next.user = { id: 81, email: 'second@example.com', balance: 2 }; lookup.mockResolvedValueOnce(next); await view.get('form').trigger('submit'); await flushPromises()
    pending.resolve({ user: { ...result().user, balance: 999 }, trial: { ...result().trial, status: 'granted' } }); await flushPromises(); expect(view.text()).toContain('second@example.com'); expect(view.text()).not.toContain('customer@example.com')
    await view.get('[data-test="support-grant"]').trigger('click'); expect(grant).toHaveBeenLastCalledWith(81)
  })
  it('deduplicates submits and ignores old lookup responses after input changes', async () => {
    const pending = deferred<ReturnType<typeof result>>(); lookup.mockReturnValueOnce(pending.promise); const view = createView(); await flushPromises(); await view.get('[data-test="support-account"]').setValue('7147')
    await view.get('form').trigger('submit'); await view.get('form').trigger('submit'); expect(lookup).toHaveBeenCalledTimes(1)
    await view.get('[data-test="support-account"]').setValue('81'); pending.resolve(result()); await flushPromises(); expect(view.find('[data-test="support-result"]').exists()).toBe(false); expect(grant).not.toHaveBeenCalled()
  })
  it('discards pending grants and customer data when the login session changes', async () => {
    const pending = deferred<unknown>(); grant.mockReturnValueOnce(pending.promise); const view = createView(); await query(view); await view.get('[data-test="support-grant"]').trigger('click')
    useAuthStore().token = 'second-session'; await flushPromises(); expect(refresh).toHaveBeenCalledTimes(2); expect(view.find('[data-test="support-result"]').exists()).toBe(false)
    pending.resolve({ user: result().user, trial: { ...result().trial, status: 'granted' } }); await flushPromises(); expect(view.find('[data-test="support-result"]').exists()).toBe(false)
  })
  it('does not start a reconciliation lookup after the panel has unmounted', async () => {
    const pending = deferred<unknown>(); grant.mockReturnValueOnce(pending.promise)
    const view = createView(); await query(view); await view.get('[data-test="support-grant"]').trigger('click')
    views.splice(views.indexOf(view), 1); view.unmount()
    pending.resolve({ user: { ...result().user, balance: 32.5 }, trial: { ...result().trial, status: 'granted' } }); await flushPromises()
    expect(lookup).toHaveBeenCalledTimes(1)
  })
  it('clears customer data on external permission revocation', async () => {
    const view = createView(); await query(view); reactive(permission).enabled = false; await flushPromises(); expect(view.find('[data-test="support-result"]').exists()).toBe(false)
    expect(view.get('[data-test="support-account"]').attributes('disabled')).toBeDefined(); expect(view.emitted('access-denied')).toHaveLength(1)
  })
  it.each(['lookup', 'grant', 'search'])('fails closed for a forbidden %s response', async method => {
    const rejection = { status: 403, code: 'FORBIDDEN', message: 'https://secret.invalid sk-secret' }; const view = createView(); await flushPromises()
    if (method === 'grant') { await query(view); grant.mockRejectedValueOnce(rejection); await view.get('[data-test="support-grant"]').trigger('click') }
    else if (method === 'search') { vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] }); search.mockRejectedValueOnce(rejection); await view.get('[data-test="support-account"]').setValue('cu'); await vi.advanceTimersByTimeAsync(250) }
    else { lookup.mockRejectedValueOnce(rejection); await query(view) }
    await flushPromises(); expect(revoke).toHaveBeenCalled(); expect(view.find('[data-test="support-result"]').exists()).toBe(false); expect(view.text()).toContain('userSupport.errors.forbidden'); expect(view.html()).not.toMatch(/secret\.invalid|sk-secret/)
  })
  it('rejects a grant response that belongs to another user', async () => {
    const view = createView(); await query(view); grant.mockResolvedValueOnce({ user: { id: 81, email: 'wrong@example.com', balance: 999 }, trial: { ...result().trial, status: 'granted' } })
    await view.get('[data-test="support-grant"]').trigger('click'); await flushPromises(); expect(view.text()).toContain('customer@example.com'); expect(view.text()).not.toContain('wrong@example.com'); expect(view.text()).toContain('userSupport.errors.grantUnavailable')
  })
})

describe('support request records', () => {
  it('defaults to usage, preserves all fields and supports keyboard tabs', async () => {
    const view = createView(); await query(view); const usageTab = view.get('[data-test="support-tab-usage"]'); const errorsTab = view.get('[data-test="support-tab-errors"]')
    expect(usageTab.attributes('aria-selected')).toBe('true'); expect(errorsTab.attributes('tabindex')).toBe('-1'); const row = view.findAll('[data-test="support-usage-row"]')[0]
    for (const tab of [usageTab, errorsTab]) expect(view.find(`#${tab.attributes('aria-controls')}`).exists()).toBe(true)
    expect(row.get('h3').text()).toBe('gemini-4-argon'); expect(row.text()).toContain('Gemini <VIP>'); expect(row.find('vip').exists()).toBe(false)
    expect(row.get('time').text()).toMatch(/\d{2}:\d{2}:\d{2}/); expect(row.get('time').attributes('datetime')).toBe('2026-10-31T04:00:00Z')
    for (const text of ['userSupport.stream', 'userSupport.status.completed']) expect(row.text()).toContain(text)
    for (const [field, label, values] of [
      ['tokens', 'userSupport.columns.tokens', ['200', '12']], ['cache', 'userSupport.cache', ['0']],
      ['charge', 'userSupport.columns.charge', ['0.10']], ['duration', 'userSupport.columns.duration', ['0.90s']],
      ['first-token', 'userSupport.firstToken', ['0.20s']],
    ] as const) {
      const metric = row.get(`[data-field="${field}"]`)
      expect(metric.isVisible()).toBe(true); expect(metric.get('dt').text()).toBe(label)
      for (const value of values) expect(metric.get('dd').text()).toContain(value)
    }
    await usageTab.trigger('keydown', { key: 'ArrowRight' }); expect(errorsTab.attributes('aria-selected')).toBe('true'); expect(document.activeElement).toBe(errorsTab.element); expect(view.findAll('[data-test="support-error-row"]')).toHaveLength(10)
    await errorsTab.trigger('keydown', { key: 'Home' }); expect(usageTab.attributes('aria-selected')).toBe('true'); await usageTab.trigger('keydown', { key: 'End' }); expect(errorsTab.attributes('aria-selected')).toBe('true')
    const visiblePanel = view.findAll('[role="tabpanel"]').filter(panel => panel.isVisible())
    expect(visiblePanel).toHaveLength(1); expect(visiblePanel[0].attributes('aria-labelledby')).toBe(errorsTab.attributes('id'))
  })
  it('shows the original error with model, group, full date and recorded HTTP/category status', async () => {
    const data = result(); data.errors.items[1].original_message = '<img src=x onerror="alert(1)"> raw'; lookup.mockResolvedValue(data); const view = createView(); await query(view); await showErrors(view)
    const rows = view.findAll('[data-test="support-error-row"]'); expect(rows[0].text()).toMatch(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}/); expect(rows[0].text()).toContain('gemini-4-argon'); expect(rows[0].text()).toContain('Gemini <VIP>')
    expect(rows[0].get('[data-test="support-error-status"]').text()).toBe('HTTP 502｜userSupport.categories.service_unavailable'); expect(rows[0].text()).toContain('userSupport.errorKinds.upstream'); expect(rows[0].get('[data-test="support-error-original"]').text()).toContain(originalMessage)
    expect(rows[1].get('[data-test="support-error-original"]').text()).toContain('<img src=x onerror="alert(1)"> raw'); expect(rows[1].find('img').exists()).toBe(false); expect(view.html()).not.toMatch(/SECRET-UPSTREAM|secret\.invalid|sk-secret/)
  })
  it('omits invalid HTTP prefixes and explains missing original messages', async () => {
    const data = result(); data.errors.items = [0, 99, 600, 429.5, null].map((status, i) => ({ ...data.errors.items[i], status_code: status, error_kind: 'arbitrary', original_message: i ? '' : '  ' })); lookup.mockResolvedValue(data)
    const view = createView(); await query(view); await showErrors(view)
    for (const row of view.findAll('[data-test="support-error-row"]')) { expect(row.get('[data-test="support-error-status"]').text()).toBe('userSupport.categories.service_unavailable'); expect(row.text()).toContain('userSupport.errorKinds.unknown'); expect(row.text()).toContain('userSupport.noOriginalError') }
  })
  it('distinguishes unavailable records from empty history without changing trial eligibility', async () => {
    const data = result(); data.usage.status = 'unavailable'; data.usage.items = []; lookup.mockResolvedValue(data); const view = createView(); await query(view)
    expect(view.get('[data-test="support-usage"]').text()).toContain('userSupport.recordsUnavailable'); expect(view.text()).not.toContain('userSupport.noUsage'); expect(view.find('[data-test="support-grant"]').exists()).toBe(true); expect(grant).not.toHaveBeenCalled()
  })
})

it('labels and preserves full public requested model names in both panels', async () => {
  const records = result()
  records.usage.items[0].model = '按量vet/gemini-4-argon'
  records.errors.items[0].model = '我的专用/新模型别名'
  lookup.mockResolvedValue(records)
  const view = createView(); await query(view)
  const usage = view.findAll('[data-test="support-usage-row"]')[0]
  expect(usage.get('h3').text()).toBe('按量vet/gemini-4-argon')
  expect(usage.text()).toContain('userSupport.columns.model')
  await showErrors(view)
  const error = view.findAll('[data-test="support-error-row"]')[0]
  expect(error.get('h3').text()).toBe('我的专用/新模型别名')
  expect(error.text()).toContain('userSupport.columns.model')
})
