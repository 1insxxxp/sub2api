import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminGroup } from '@/types'
import GroupAccountUserExclusionsModal from '../GroupAccountUserExclusionsModal.vue'

const mocks = vi.hoisted(() => ({
  listAccountUserExclusions: vi.fn(), addAccountUserExclusion: vi.fn(), removeAccountUserExclusion: vi.fn(),
  listAccounts: vi.fn(), listUsers: vi.fn(), getUser: vi.fn(), showSuccess: vi.fn(), showError: vi.fn()
}))
const auth = reactive({ isAdmin: true })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.showSuccess, showError: mocks.showError }) }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  groups: { listAccountUserExclusions: mocks.listAccountUserExclusions, addAccountUserExclusion: mocks.addAccountUserExclusion, removeAccountUserExclusion: mocks.removeAccountUserExclusion },
  accounts: { list: mocks.listAccounts }, users: { list: mocks.listUsers, getById: mocks.getUser }
} }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

const group = (id: number) => ({ id, name: `Group ${id}`, platform: 'anthropic' } as AdminGroup)
const user = { id: 86, email: 'customer@example.com', username: 'Customer name', notes: 'Customer note' }
const entry = { id: 7, group_id: 1, account_id: 10, account_name: 'Private upstream', user_id: 86, user_email: user.email, username: user.username, notes: user.notes, created_at: '2026-10-21T00:00:00Z' }
const pagination = (items: unknown[], total = items.length) => ({ items, total, page: 1, page_size: 10, pages: Math.ceil(total / 10) })
function deferred() {
  let resolve!: (value: unknown) => void
  const promise = new Promise(res => { resolve = res })
  return { promise, resolve }
}
function create(show = true) {
  return mount(GroupAccountUserExclusionsModal, {
    props: { show, group: group(1) },
    global: { stubs: {
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
      Select: { props: ['modelValue', 'options'], emits: ['update:modelValue'], template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', Number($event.target.value))"><option value="">Choose</option><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>' },
      Pagination: { props: ['page'], emits: ['update:page'], template: '<button @click="$emit(\'update:page\', page + 1)">Next</button>' }
    } }
  })
}
beforeEach(() => {
  vi.resetAllMocks()
  auth.isAdmin = true
  mocks.listAccountUserExclusions.mockResolvedValue(pagination([entry]))
  mocks.listAccounts.mockResolvedValue(pagination([{ id: 10, name: 'Private upstream' }]))
  mocks.listUsers.mockResolvedValue(pagination([user]))
  mocks.getUser.mockResolvedValue(user)
  mocks.addAccountUserExclusion.mockResolvedValue(entry)
  mocks.removeAccountUserExclusion.mockResolvedValue(undefined)
})
afterEach(() => vi.useRealTimers())

describe('group account user exclusions', () => {
  it('does not fetch data while closed and only loads accounts in the opened group', async () => {
    const wrapper = create(false)
    await flushPromises()
    expect(mocks.listAccountUserExclusions).not.toHaveBeenCalled()
    expect(mocks.listAccounts).not.toHaveBeenCalled()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(mocks.listAccountUserExclusions).toHaveBeenCalledWith(1, 1, 10, expect.any(Object))
    expect(mocks.listAccounts).toHaveBeenCalledWith(1, 20, { group: '1', lite: 'true', search: undefined }, expect.any(Object))
    expect(mocks.listUsers).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Customer name')
    expect(wrapper.text()).toContain('Customer note')
    expect(wrapper.text()).toContain('Private upstream')
  })

  it('hides private details and sends no requests for non-super-admin roles', async () => {
    auth.isAdmin = false
    const wrapper = create()
    await flushPromises()
    expect(wrapper.text()).toBe('')
    expect(mocks.listAccountUserExclusions).not.toHaveBeenCalled()
    expect(mocks.listAccounts).not.toHaveBeenCalled()
    expect(mocks.listUsers).not.toHaveBeenCalled()
  })

  it('clears loaded private information immediately when super-admin access is lost', async () => {
    const wrapper = create()
    await flushPromises()
    auth.isAdmin = false
    await flushPromises()
    expect(wrapper.text()).toBe('')
    expect(mocks.removeAccountUserExclusion).not.toHaveBeenCalled()
  })

  it('searches a bounded user page and adds only the selected group/account/user tuple', async () => {
    mocks.listAccountUserExclusions.mockResolvedValue(pagination([]))
    const wrapper = create()
    await flushPromises()
    await wrapper.get('select').setValue('10')
    vi.useFakeTimers()
    await wrapper.get('[data-test="group-account-exclusions-user-search"]').setValue('customer@')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mocks.listUsers).toHaveBeenCalledWith(1, 10, { search: 'customer@' }, expect.any(Object))
    await wrapper.get('[data-test="group-account-exclusion-add-86"]').trigger('click')
    await flushPromises()
    expect(mocks.addAccountUserExclusion).toHaveBeenCalledWith(1, { account_id: 10, user_id: 86 })
    expect(mocks.listAccountUserExclusions).toHaveBeenCalledTimes(2)
  })

  it('uses exact numeric user ID lookup without dumping the users list', async () => {
    const wrapper = create()
    await flushPromises()
    vi.useFakeTimers()
    await wrapper.get('[data-test="group-account-exclusions-user-search"]').setValue('86')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mocks.getUser).toHaveBeenCalledWith(86)
    expect(mocks.listUsers).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain(user.email)
  })

  it('deletes the exclusion ID scoped to the current group and reloads', async () => {
    const wrapper = create()
    await flushPromises()
    await wrapper.get('[data-test="group-account-exclusion-remove-7"]').trigger('click')
    await flushPromises()
    expect(mocks.removeAccountUserExclusion).toHaveBeenCalledWith(1, 7)
    expect(mocks.listAccountUserExclusions).toHaveBeenCalledTimes(2)
  })

  it('paginates exclusions through the server', async () => {
    mocks.listAccountUserExclusions.mockResolvedValue(pagination([entry], 21))
    const wrapper = create()
    await flushPromises()
    await wrapper.get('[data-test="group-account-exclusions-pagination"]').trigger('click')
    await flushPromises()
    expect(mocks.listAccountUserExclusions).toHaveBeenLastCalledWith(1, 2, 10, expect.any(Object))
  })

  it('ignores stale previous-group list responses and mutations cannot target that group', async () => {
    const old = deferred()
    mocks.listAccountUserExclusions.mockReturnValueOnce(old.promise).mockResolvedValueOnce(pagination([{ ...entry, id: 8, group_id: 2, account_name: 'Current upstream' }]))
    const wrapper = create()
    await wrapper.setProps({ group: group(2) })
    await flushPromises()
    old.resolve(pagination([entry]))
    await flushPromises()
    expect(wrapper.get('[data-test="group-account-exclusion-8"]').text()).toContain('Current upstream')
    expect(wrapper.find('[data-test="group-account-exclusion-7"]').exists()).toBe(false)
    await wrapper.get('[data-test="group-account-exclusion-remove-8"]').trigger('click')
    await flushPromises()
    expect(mocks.removeAccountUserExclusion).toHaveBeenCalledWith(2, 8)
  })

  it('cancels a queued user search when closed', async () => {
    const wrapper = create()
    await flushPromises()
    vi.useFakeTimers()
    await wrapper.get('[data-test="group-account-exclusions-user-search"]').setValue('customer@')
    await wrapper.setProps({ show: false })
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(mocks.listUsers).not.toHaveBeenCalled()
    expect(wrapper.text()).toBe('')
  })

  it('discards a previous group account response so it cannot be selected in the next group', async () => {
    const old = deferred()
    mocks.listAccounts.mockReturnValueOnce(old.promise).mockResolvedValueOnce(pagination([{ id: 20, name: 'Next group upstream' }]))
    const wrapper = create()
    await wrapper.setProps({ group: group(2) })
    await flushPromises()
    old.resolve(pagination([{ id: 10, name: 'Previous group upstream' }]))
    await flushPromises()
    expect(wrapper.get('select').text()).toContain('Next group upstream')
    expect(wrapper.get('select').text()).not.toContain('Previous group upstream')
    expect(mocks.listAccounts).toHaveBeenLastCalledWith(1, 20, { group: '2', lite: 'true', search: undefined }, expect.any(Object))
  })

  it('ignores user search results that finish after closing and reopening another group', async () => {
    const old = deferred()
    mocks.listUsers.mockReturnValueOnce(old.promise)
    const wrapper = create()
    await flushPromises()
    vi.useFakeTimers()
    await wrapper.get('[data-test="group-account-exclusions-user-search"]').setValue('old@')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, group: group(2) })
    await flushPromises()
    old.resolve(pagination([{ ...user, id: 91, email: 'old@example.com' }]))
    await flushPromises()
    expect(wrapper.find('[data-test="group-account-exclusion-add-91"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('old@example.com')
  })

  it('does not reload or notify the newly opened group after an older deletion completes', async () => {
    const deletion = deferred()
    mocks.removeAccountUserExclusion.mockReturnValueOnce(deletion.promise)
    const wrapper = create()
    await flushPromises()
    await wrapper.get('[data-test="group-account-exclusion-remove-7"]').trigger('click')
    await wrapper.setProps({ group: group(2) })
    await flushPromises()
    deletion.resolve(undefined)
    await flushPromises()
    expect(mocks.listAccountUserExclusions).toHaveBeenCalledTimes(2)
    expect(mocks.showSuccess).not.toHaveBeenCalled()
  })

  it('shows a retryable list error and preserves separate account loading', async () => {
    mocks.listAccountUserExclusions.mockRejectedValueOnce(new Error('Cannot load exclusions'))
    const wrapper = create()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Cannot load exclusions')
    await wrapper.get('[data-test="group-account-exclusions-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain(user.email)
  })
})
