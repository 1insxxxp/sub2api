import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubAdminCommissionManagement from '../SubAdminCommissionManagement.vue'

const { api, showError } = vi.hoisted(() => ({
  api: { getSettings: vi.fn(), listGrants: vi.fn(), getAll: vi.fn(), listExclusions: vi.fn(), addExclusion: vi.fn(), removeExclusion: vi.fn(), listUsers: vi.fn(), getUser: vi.fn(), updateSettings: vi.fn(), replaceGrants: vi.fn() },
  showError: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  subAdminCommission: api, groups: { getAll: api.getAll }, users: { list: api.listUsers, getById: api.getUser },
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
const page = (items: unknown[], total = items.length, current = 1) => ({ items, total, page: current, page_size: 10, pages: Math.max(1, Math.ceil(total / 10)) })
const exclusion = { id: 1, user_id: 42, user_email: 'client@example.com', username: 'Client', started_at: '2026-10-18T10:00:00Z' }
const wrappers: Array<ReturnType<typeof mount>> = []
const create = (realDialog = false) => {
  const wrapper = mount(SubAdminCommissionManagement, { attachTo: realDialog ? document.body : undefined, global: { stubs: {
    Icon: true,
    BaseDialog: realDialog ? false : { name: 'BaseDialog', props: ['show', 'title', 'appearance', 'width'], template: '<div v-if="show" data-test="exclusions-dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /><button data-test="dialog-close" @click="$emit(\'close\')">Close</button></div>' },
    Pagination: { props: ['page', 'total'], emits: ['update:page'], template: '<button data-test="exclusions-next-page" @click="$emit(\'update:page\', 2)">Next</button>' },
  } } })
  wrappers.push(wrapper)
  return wrapper
}
async function open(wrapper: ReturnType<typeof create>) {
  await wrapper.get('[data-test="commission-exclusions-open"]').trigger('click')
  await flushPromises()
}
async function openSettings(wrapper: ReturnType<typeof create>) {
  await wrapper.get('[data-test="commission-settings-open"]').trigger('click')
  await flushPromises()
}
async function search(wrapper: ReturnType<typeof create>, value: string) {
  await wrapper.get('[data-test="commission-exclusions-search"]').setValue(value)
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
beforeEach(() => {
  vi.useFakeTimers()
  Object.values(api).forEach((mock) => mock.mockReset())
  showError.mockReset()
  api.getSettings.mockResolvedValue({ commission_rate: 0.12 })
  api.listGrants.mockResolvedValue([])
  api.getAll.mockResolvedValue([])
  api.listExclusions.mockResolvedValue(page([exclusion]))
  api.listUsers.mockResolvedValue(page([{ id: 43, email: 'new@example.com', username: 'New' }]))
  api.getUser.mockResolvedValue({ id: 43, email: 'new@example.com', username: 'New' })
  api.addExclusion.mockResolvedValue({ ...exclusion, user_id: 43 })
  api.removeExclusion.mockResolvedValue({ message: 'success' })
  api.updateSettings.mockResolvedValue({ commission_rate: 0.2 })
  api.replaceGrants.mockResolvedValue([])
})
afterEach(() => { wrappers.splice(0).forEach((wrapper) => wrapper.unmount()); vi.useRealTimers() })

describe('commission exclusion management', () => {
  it('loads the blacklist only when its neutral dialog is opened', async () => {
    const wrapper = create()
    await flushPromises()
    expect(api.listExclusions).not.toHaveBeenCalled()
    expect(api.listUsers).not.toHaveBeenCalled()
    await open(wrapper)
    expect(api.listExclusions).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.getComponent({ name: 'BaseDialog' }).props('appearance')).toBe('neutral')
    expect(wrapper.text()).toContain('client@example.com')
    expect(wrapper.get('[data-test="commission-exclusion-42"]').text()).toContain('42')
    expect(wrapper.get('[data-test="commission-exclusion-42"]').text()).toContain('2026')
    expect(wrapper.text()).toContain('adminWorkbench.commission.exclusionsHint')
    expect(api.getSettings).not.toHaveBeenCalled()
    expect(api.getAll).not.toHaveBeenCalled()
    expect(api.listGrants).not.toHaveBeenCalled()
  })

  it('identifies excluded customers by labeled username and administrator notes', async () => {
    api.listExclusions.mockResolvedValueOnce(page([{ ...exclusion, notes: 'VIP customer\nInvoice contact <b>finance</b>' }]))
    const wrapper = create()
    await open(wrapper)
    const customer = wrapper.get('[data-test="commission-exclusion-42"]')
    expect(customer.text()).toContain('admin.users.username：Client')
    expect(customer.text()).toContain('admin.users.notes：VIP customer\nInvoice contact <b>finance</b>')
    expect(customer.find('b').exists()).toBe(false)
    expect(api.getUser).not.toHaveBeenCalled()
  })

  it('shows explicit empty values for an excluded customer without a username or notes', async () => {
    api.listExclusions.mockResolvedValueOnce(page([{ ...exclusion, username: '', notes: '' }]))
    const wrapper = create()
    await open(wrapper)
    const customer = wrapper.get('[data-test="commission-exclusion-42"]')
    expect(customer.text()).toContain('admin.users.username：—')
    expect(customer.text()).toContain('admin.users.notes：—')
  })

  it('shows username and notes in search results before selecting a customer', async () => {
    api.listUsers.mockResolvedValueOnce(page([{ id: 43, email: 'new@example.com', username: 'New', notes: 'Corporate contact' }]))
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, 'new@')
    const customer = wrapper.get('[data-test="commission-exclusion-add-43"]').element.closest('li')!
    expect(customer.textContent).toContain('admin.users.username：New')
    expect(customer.textContent).toContain('admin.users.notes：Corporate contact')
    expect(api.getUser).not.toHaveBeenCalled()
  })

  it('searches a small page, then adds a selected customer and refreshes summaries', async () => {
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, 'new@')
    expect(api.listUsers).toHaveBeenCalledWith(1, 10, { search: 'new@' }, { signal: expect.any(AbortSignal) })
    await wrapper.get('[data-test="commission-exclusion-add-43"]').trigger('click')
    await flushPromises()
    expect(api.addExclusion).toHaveBeenCalledWith({ user_id: 43 })
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect(api.listExclusions).toHaveBeenCalledTimes(2)
    expect((wrapper.get('[data-test="commission-exclusions-search"]').element as HTMLInputElement).value).toBe('')
  })

  it('resolves a numeric ID precisely without loading all users', async () => {
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, '43')
    expect(api.getUser).toHaveBeenCalledWith(43)
    expect(api.listUsers).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="commission-exclusion-add-43"]').text()).toContain('adminWorkbench.commission.addExclusion')
  })

  it('finds numeric email prefixes when no matching user ID exists', async () => {
    api.getUser.mockRejectedValueOnce({ status: 404, message: 'user not found' })
    api.listUsers.mockResolvedValueOnce(page([{ id: 86, email: '123456789@example.com', username: 'Client' }]))
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, '123456789')
    expect(api.listUsers).toHaveBeenCalledWith(1, 10, { search: '123456789' }, { signal: expect.any(AbortSignal) })
    expect(wrapper.get('[data-test="commission-exclusion-add-86"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('user not found')
  })

  it('shows no matching data when neither numeric ID nor text search finds a customer', async () => {
    api.getUser.mockRejectedValueOnce({ status: 404, message: 'user not found' })
    api.listUsers.mockResolvedValueOnce(page([]))
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, '123456789')
    expect(wrapper.text()).toContain('common.noData')
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('does not hide permission or server errors behind numeric text search', async () => {
    api.getUser.mockRejectedValueOnce({ status: 403, message: 'forbidden' })
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, '43')
    expect(api.listUsers).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toBe('forbidden')
  })

  it('searches long numeric email prefixes without losing precision', async () => {
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, '9007199254740993')
    expect(api.getUser).not.toHaveBeenCalled()
    expect(api.listUsers).toHaveBeenCalledWith(1, 10, { search: '9007199254740993' }, { signal: expect.any(AbortSignal) })
    expect(wrapper.find('[data-test="commission-exclusion-add-43"]').exists()).toBe(true)
  })

  it('does not start a fallback search after a numeric lookup is superseded', async () => {
    let reject!: (reason: unknown) => void
    api.getUser.mockReturnValueOnce(new Promise((_resolve, fail) => { reject = fail }))
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, '123456789')
    await search(wrapper, 'new@')
    reject({ status: 404, message: 'user not found' })
    await flushPromises()
    expect(api.listUsers).toHaveBeenCalledTimes(1)
    expect(api.listUsers).toHaveBeenCalledWith(1, 10, { search: 'new@' }, { signal: expect.any(AbortSignal) })
    expect(wrapper.find('[data-test="commission-exclusion-add-43"]').exists()).toBe(true)
  })

  it('ignores stale searches and cancels pending searches when closed', async () => {
    const stale = deferred<ReturnType<typeof page>>()
    api.listUsers.mockReturnValueOnce(stale.promise)
    const wrapper = create()
    await open(wrapper)
    await search(wrapper, 'old@')
    await search(wrapper, 'new@')
    stale.resolve(page([{ id: 44, email: 'old@example.com' }]))
    await flushPromises()
    expect(wrapper.find('[data-test="commission-exclusion-add-44"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="commission-exclusion-add-43"]').exists()).toBe(true)
    await wrapper.get('[data-test="commission-exclusions-search"]').setValue('queued@')
    await wrapper.get('[data-test="dialog-close"]').trigger('click')
    await vi.advanceTimersByTimeAsync(300)
    expect(api.listUsers).toHaveBeenCalledTimes(2)
  })

  it('paginates the blacklist and removes a customer without changing their status', async () => {
    api.listExclusions.mockResolvedValueOnce(page([exclusion], 21)).mockResolvedValueOnce(page([exclusion], 21, 2))
    const wrapper = create()
    await open(wrapper)
    await wrapper.get('[data-test="exclusions-next-page"]').trigger('click')
    await flushPromises()
    expect(api.listExclusions).toHaveBeenLastCalledWith({ page: 2, page_size: 10 })
    await wrapper.get('[data-test="commission-exclusion-remove-42"]').trigger('click')
    await flushPromises()
    expect(api.removeExclusion).toHaveBeenCalledWith(42)
    expect(wrapper.emitted('changed')).toHaveLength(1)
  })

  it('does not report a successful change when a mutation fails', async () => {
    api.removeExclusion.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create()
    await open(wrapper)
    await wrapper.get('[data-test="commission-exclusion-remove-42"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(wrapper.find('[data-test="commission-exclusion-42"]').exists()).toBe(true)
    expect(showError).toHaveBeenCalled()
  })

  it('moves back from an empty last page after removal', async () => {
    api.listExclusions
      .mockResolvedValueOnce(page([exclusion], 11))
      .mockResolvedValueOnce(page([exclusion], 11, 2))
      .mockResolvedValueOnce(page([], 10, 2))
      .mockResolvedValueOnce(page([exclusion], 10))
    const wrapper = create()
    await open(wrapper)
    await wrapper.get('[data-test="exclusions-next-page"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="commission-exclusion-remove-42"]').trigger('click')
    await flushPromises()
    expect(api.listExclusions).toHaveBeenLastCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.find('[data-test="commission-exclusion-42"]').exists()).toBe(true)
  })

  it('shows a retry action after a list error without searching all customers', async () => {
    api.listExclusions.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create()
    await open(wrapper)
    expect(wrapper.get('[role="alert"]').text()).toBe('offline')
    const retry = wrapper.findAll('button').find((button) => button.text() === 'common.refresh')!
    await retry.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="commission-exclusion-42"]').exists()).toBe(true)
    expect(api.listUsers).not.toHaveBeenCalled()
  })

  it('loads settings only when the wide neutral settings dialog is opened', async () => {
    const wrapper = create()
    await flushPromises()
    expect(api.getSettings).not.toHaveBeenCalled()
    expect(api.getAll).not.toHaveBeenCalled()
    expect(api.listGrants).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="sub-admin-commission-rate"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'BaseDialog' }).exists()).toBe(false)
    await openSettings(wrapper)
    expect(api.getSettings).toHaveBeenCalledTimes(1)
    expect(api.getAll).toHaveBeenCalledTimes(1)
    expect(api.listGrants).toHaveBeenCalledTimes(1)
    const dialog = wrapper.getComponent({ name: 'BaseDialog' })
    expect(dialog.props()).toMatchObject({ title: 'adminWorkbench.commission.settings', appearance: 'neutral', width: 'wide' })
    expect((wrapper.get('[data-test="sub-admin-commission-rate"]').element as HTMLInputElement).value).toBe('0.12')
    expect(api.listExclusions).not.toHaveBeenCalled()
  })

  it('cycles keyboard focus across the real settings dialog and returns it to the opener on close', async () => {
    const wrapper = create(true)
    const opener = wrapper.get('[data-test="commission-settings-open"]').element as HTMLButtonElement
    opener.focus()
    await openSettings(wrapper)
    const dialog = document.body.querySelector<HTMLElement>('[role="dialog"]')!
    expect(dialog).not.toBeNull()
    const first = dialog.querySelector<HTMLButtonElement>('.modal-header button')!
    const last = dialog.querySelector<HTMLButtonElement>('.modal-footer button')!
    expect(document.activeElement).toBe(first)

    const backwards = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true })
    first.dispatchEvent(backwards)
    expect(backwards.defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(last)
    const forwards = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
    last.dispatchEvent(forwards)
    expect(forwards.defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(first)

    const input = dialog.querySelector<HTMLInputElement>('[data-test="sub-admin-commission-rate"]')!
    input.focus()
    const middle = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
    input.dispatchEvent(middle)
    expect(middle.defaultPrevented).toBe(false)
    last.click()
    await flushPromises()
    expect(document.body.querySelector('[role="dialog"]')).toBeNull()
    expect(document.activeElement).toBe(opener)
    const closed = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true })
    opener.dispatchEvent(closed)
    expect(closed.defaultPrevented).toBe(false)
  })

  it('preserves unsaved rate and group choices when settings are reopened without refetching', async () => {
    api.getAll.mockResolvedValueOnce([{ id: 9, name: 'Shared group' }])
    const wrapper = create()
    await openSettings(wrapper)
    await wrapper.get('[data-test="sub-admin-commission-rate"]').setValue('0.27')
    await wrapper.get('[data-test="sub-admin-commission-group-9"]').setValue(true)
    await wrapper.get('[data-test="dialog-close"]').trigger('click')
    expect(wrapper.find('[data-test="sub-admin-commission-rate"]').exists()).toBe(false)
    await openSettings(wrapper)
    expect((wrapper.get('[data-test="sub-admin-commission-rate"]').element as HTMLInputElement).value).toBe('0.27')
    expect((wrapper.get('[data-test="sub-admin-commission-group-9"]').element as HTMLInputElement).checked).toBe(true)
    expect(api.getSettings).toHaveBeenCalledTimes(1)
    expect(api.getAll).toHaveBeenCalledTimes(1)
    expect(api.listGrants).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    await flushPromises()
    expect(api.replaceGrants).toHaveBeenCalledWith({ group_ids: [9] })
  })

  it('waits for all initial settings reads and reuses an in-flight load after reopening', async () => {
    const pending = deferred<unknown[]>()
    api.getAll.mockReturnValueOnce(pending.promise)
    const wrapper = create()
    await openSettings(wrapper)
    expect(wrapper.get('[data-test="sub-admin-commission-save-settings"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="sub-admin-commission-save-grants"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="sub-admin-commission-save-settings"]').trigger('click')
    expect(api.updateSettings).not.toHaveBeenCalled()
    await wrapper.get('[data-test="dialog-close"]').trigger('click')
    await openSettings(wrapper)
    expect(api.getSettings).toHaveBeenCalledTimes(1)
    expect(api.getAll).toHaveBeenCalledTimes(1)
    expect(api.listGrants).toHaveBeenCalledTimes(1)
    pending.resolve([])
    await flushPromises()
    expect(wrapper.get('[data-test="sub-admin-commission-save-settings"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-test="sub-admin-commission-save-grants"]').attributes('disabled')).toBeUndefined()
  })

  it('disables saving after a failed initial load and retries on the next open', async () => {
    api.getSettings.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create()
    await openSettings(wrapper)
    expect(wrapper.get('[role="alert"]').text()).toBe('offline')
    expect(wrapper.get('[data-test="sub-admin-commission-save-settings"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="sub-admin-commission-save-grants"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="sub-admin-commission-save-settings"]').trigger('click')
    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    expect(api.updateSettings).not.toHaveBeenCalled()
    expect(api.replaceGrants).not.toHaveBeenCalled()
    await wrapper.get('[data-test="dialog-close"]').trigger('click')
    await openSettings(wrapper)
    expect(api.getSettings).toHaveBeenCalledTimes(2)
    expect(api.getAll).toHaveBeenCalledTimes(2)
    expect(api.listGrants).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="sub-admin-commission-save-settings"]').attributes('disabled')).toBeUndefined()
  })

  it('keeps failed save drafts and only requests summary refresh after a successful save', async () => {
    api.getAll.mockResolvedValueOnce([{ id: 9, name: 'Shared group' }])
    api.updateSettings.mockRejectedValueOnce(new Error('offline'))
    api.replaceGrants.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create()
    await openSettings(wrapper)
    await wrapper.get('[data-test="sub-admin-commission-rate"]').setValue('0.27')
    await wrapper.get('[data-test="sub-admin-commission-group-9"]').setValue(true)
    await wrapper.get('[data-test="sub-admin-commission-save-settings"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('changed')).toBeUndefined()
    expect(showError).toHaveBeenCalledTimes(2)
    await wrapper.get('[data-test="dialog-close"]').trigger('click')
    await openSettings(wrapper)
    expect((wrapper.get('[data-test="sub-admin-commission-rate"]').element as HTMLInputElement).value).toBe('0.27')
    expect((wrapper.get('[data-test="sub-admin-commission-group-9"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-test="sub-admin-commission-save-settings"]').trigger('click')
    await flushPromises()
    expect(api.updateSettings).toHaveBeenLastCalledWith({ commission_rate: 0.27 })
    expect(wrapper.emitted('changed')).toHaveLength(1)
    expect(api.getSettings).toHaveBeenCalledTimes(1)
    expect(api.getAll).toHaveBeenCalledTimes(1)
    expect(api.listGrants).toHaveBeenCalledTimes(1)
  })

  it('also notifies the panel after rate and assignment changes', async () => {
    const wrapper = create()
    await openSettings(wrapper)
    await wrapper.get('[data-test="sub-admin-commission-save-settings"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('changed')).toHaveLength(2)
  })
})
