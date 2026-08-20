import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'

const {
  authState,
  getHistory,
  getGenerated,
  generateBalanceTransferCode,
  generateBalanceTransferCodes,
  deleteGenerated,
  getPublicSettings,
  refreshUser,
  showError,
  showSuccess
} = vi.hoisted(() => ({
  authState: {
    user: {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: false
    } as Record<string, unknown>
  },
  getHistory: vi.fn(),
  getGenerated: vi.fn(),
  generateBalanceTransferCode: vi.fn(),
  generateBalanceTransferCodes: vi.fn(),
  deleteGenerated: vi.fn(),
  getPublicSettings: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getHistory,
    getGenerated,
    generateBalanceTransferCode,
    generateBalanceTransferCodes,
    deleteGenerated,
    redeem: vi.fn()
  },
  authAPI: { getPublicSettings }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get user() {
      return authState.user
    },
    refreshUser
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning: vi.fn()
  })
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const mountRedeemView = () =>
  mount(RedeemView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true
      }
    }
  })

describe('user RedeemView balance transfer code generator', () => {
  beforeEach(() => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: false
    }
    getHistory.mockReset()
    getGenerated.mockReset()
    generateBalanceTransferCode.mockReset()
    generateBalanceTransferCodes.mockReset()
    deleteGenerated.mockReset()
    getPublicSettings.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getHistory.mockResolvedValue([])
    getGenerated.mockResolvedValue([])
    getPublicSettings.mockResolvedValue({ contact_info: '' })
    refreshUser.mockResolvedValue(undefined)
  })

  it('hides the generator for users without permission', async () => {
    const wrapper = mountRedeemView()
    await flushPromises()

    expect(wrapper.find('[data-test="balance-transfer-panel"]').exists()).toBe(false)
    expect(getGenerated).not.toHaveBeenCalled()
  })

  it('batch-generates balance redeem codes and refreshes the account state', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getGenerated.mockResolvedValueOnce([]).mockResolvedValueOnce([
      {
        id: 88,
        code: 'SEND-THIS-CODE',
        type: 'balance',
        value: 7.5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 1
      }
    ])
    generateBalanceTransferCodes.mockResolvedValue([
      {
        id: 88,
        code: 'SEND-THIS-CODE',
        type: 'balance',
        value: 7.5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 1,
        single_use_per_user: true
      },
      {
        id: 89,
        code: 'SEND-SECOND-CODE',
        type: 'balance',
        value: 7.5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 1,
        single_use_per_user: true
      }
    ])

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('[data-test="balance-transfer-amount"]').setValue('7.5')
    await wrapper.get('[data-test="balance-transfer-count"]').setValue('2')
    await wrapper.get('[data-test="balance-transfer-expiry"]').setValue('14')
    await wrapper.get('[data-test="balance-transfer-notes"]').setValue('for teammate')
    await wrapper.get('[data-test="balance-transfer-single-use"]').setValue(true)
    await wrapper.get('[data-test="balance-transfer-form"]').trigger('submit')
    await flushPromises()

    expect(generateBalanceTransferCodes).toHaveBeenCalledWith({
      amount: 7.5,
      count: 2,
      expires_in_days: 14,
      notes: 'for teammate',
      single_use_per_user: true
    })
    expect(refreshUser).toHaveBeenCalled()
    expect(getGenerated).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('SEND-THIS-CODE')
    expect(wrapper.text()).toContain('redeem.balanceTransfer.singleUseBadge')
    expect(showSuccess).toHaveBeenCalledWith('redeem.balanceTransfer.generated')
  })

  it('deletes an unused generated code and refreshes balance state', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getGenerated.mockResolvedValueOnce([
      {
        id: 88,
        code: 'DELETE-THIS-CODE',
        type: 'balance',
        value: 7.5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 1
      }
    ]).mockResolvedValueOnce([])
    deleteGenerated.mockResolvedValue({ message: 'deleted' })
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('[data-test="delete-generated-code-88"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith('redeem.balanceTransfer.deleteConfirm')
    expect(deleteGenerated).toHaveBeenCalledWith(88)
    expect(refreshUser).toHaveBeenCalled()
    expect(getGenerated).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith('redeem.balanceTransfer.deleted')

    confirmSpy.mockRestore()
  })
})
