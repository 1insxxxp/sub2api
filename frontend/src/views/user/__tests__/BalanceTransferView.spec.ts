import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'

import BalanceTransferView from '../BalanceTransferView.vue'
import type { GeneratedRedeemCode } from '@/api/redeem'

const { authState, userApi, workbenchApi, refreshUser, showError } = vi.hoisted(() => ({
  authState: { user: {} as Record<string, unknown> },
  userApi: { generate: vi.fn(), list: vi.fn(), remove: vi.fn(), removeBatch: vi.fn() },
  workbenchApi: { generate: vi.fn(), list: vi.fn(), remove: vi.fn(), removeBatch: vi.fn() },
  refreshUser: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    generateUserBalanceTransferCodes: userApi.generate,
    getUserGenerated: userApi.list,
    deleteUserGenerated: userApi.remove,
    deleteUserGeneratedBatch: userApi.removeBatch,
    generateBalanceTransferCodes: workbenchApi.generate,
    getGenerated: workbenchApi.list,
    deleteGenerated: workbenchApi.remove,
    deleteGeneratedBatch: workbenchApi.removeBatch
  }
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ get user() { return authState.user }, refreshUser })
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess: vi.fn() })
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const code: GeneratedRedeemCode = {
  id: 12, code: 'OWN-TRANSFER-CODE', type: 'balance', value: 2,
  status: 'unused', created_at: '2026-10-25T00:00:00Z',
  source: 'user_balance_transfer', created_by: 8,
  used_by: null, used_at: null
}
const paginated = (items: GeneratedRedeemCode[]) => ({ items, total: items.length, page: 1, page_size: 10, pages: 1 })
const mounted: Array<ReturnType<typeof mount>> = []
function mountView() {
  const wrapper = mount(BalanceTransferView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, Pagination: true } }
  })
  mounted.push(wrapper)
  return wrapper
}

describe('ordinary user balance transfer', () => {
  beforeEach(() => {
    Object.values(userApi).forEach(fn => fn.mockReset())
    Object.values(workbenchApi).forEach(fn => fn.mockReset())
    refreshUser.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    authState.user = reactive({ id: 8, role: 'user', balance: 30, balance_redeem_code_enabled: true })
    userApi.list.mockResolvedValue(paginated([code]))
    userApi.generate.mockResolvedValue([code])
    userApi.remove.mockResolvedValue(code)
    userApi.removeBatch.mockResolvedValue({ deleted_count: 1, refunded_amount: 2 })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })
  afterEach(() => {
    mounted.splice(0).forEach(wrapper => wrapper.unmount())
    vi.restoreAllMocks()
  })

  it('generates through ordinary user APIs and refreshes the current balance', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('2')
    await wrapper.get('[data-test="workbench-transfer-count"]').setValue('3')
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()

    expect(userApi.generate).toHaveBeenCalledWith({
      amount: 2, count: 3, expires_in_days: 30, notes: '', single_use_per_user: false, threshold_exempt: false
    })
    expect(userApi.list).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(refreshUser).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-test="admin-workbench-tabs"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="workbench-commission-panel"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="workbench-affiliate-leaderboard-panel"]').exists()).toBe(false)
    Object.values(workbenchApi).forEach(fn => expect(fn).not.toHaveBeenCalled())
  })

  it('keeps a disabled ordinary user limited to their existing code history', async () => {
    authState.user.balance_redeem_code_enabled = false
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="workbench-transfer-form"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="balance-transfer-permission-notice"]').text()).toContain('adminWorkbench.balanceTransfer.generationNotEnabled')
    expect(wrapper.text()).toContain(code.code)
    await wrapper.get('[data-test="generated-code-delete-12"]').trigger('click')
    await flushPromises()
    expect(userApi.remove).toHaveBeenCalledWith(12)
    expect(userApi.generate).not.toHaveBeenCalled()
    Object.values(workbenchApi).forEach(fn => expect(fn).not.toHaveBeenCalled())
  })

  it('removes generation when permission is revoked while retaining history', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-test="workbench-transfer-form"]').exists()).toBe(true)

    authState.user.balance_redeem_code_enabled = false
    await flushPromises()

    expect(wrapper.find('[data-test="workbench-transfer-form"]').exists()).toBe(false)
    expect(wrapper.text()).toContain(code.code)
    expect(userApi.generate).not.toHaveBeenCalled()
  })

  it('recovers selected own codes through ordinary user APIs after permission is revoked', async () => {
    authState.user.balance_redeem_code_enabled = false
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-generated-code-12"]').setValue(true)
    await wrapper.get('[data-test="delete-selected-generated-codes"]').trigger('click')
    await flushPromises()

    expect(userApi.removeBatch).toHaveBeenCalledWith([12])
    expect(refreshUser).toHaveBeenCalledOnce()
    expect(userApi.list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="workbench-transfer-form"]').exists()).toBe(false)
    Object.values(workbenchApi).forEach(fn => expect(fn).not.toHaveBeenCalled())
  })

  it.each(['admin', 'sub_admin'])('preserves %s ability to generate without the user switch', async role => {
    authState.user.role = role
    authState.user.balance_redeem_code_enabled = false
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-test="workbench-transfer-form"]').exists()).toBe(true)
  })

  it('rejects insufficient balance before making a generation request', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('31')
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()
    expect(userApi.generate).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.insufficientBalance')
  })
})
