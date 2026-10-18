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
const create = () => {
  const wrapper = mount(SubAdminCommissionManagement, { global: { stubs: {
    Icon: true,
    BaseDialog: { name: 'BaseDialog', props: ['show', 'title', 'appearance'], template: '<div v-if="show" data-test="exclusions-dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /><button data-test="dialog-close" @click="$emit(\'close\')">Close</button></div>' },
    Pagination: { props: ['page', 'total'], emits: ['update:page'], template: '<button data-test="exclusions-next-page" @click="$emit(\'update:page\', 2)">Next</button>' },
  } } })
  wrappers.push(wrapper)
  return wrapper
}
async function open(wrapper: ReturnType<typeof create>) {
  await wrapper.get('[data-test="commission-exclusions-open"]').trigger('click')
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

  it('also notifies the panel after rate and assignment changes', async () => {
    const wrapper = create()
    await flushPromises()
    await wrapper.get('[data-test="sub-admin-commission-save-settings"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('changed')).toHaveLength(2)
  })
})
