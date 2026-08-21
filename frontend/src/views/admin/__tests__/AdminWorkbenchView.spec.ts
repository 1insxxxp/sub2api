import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdminWorkbenchView from '../AdminWorkbenchView.vue'
import type { GeneratedRedeemCode } from '@/api/redeem'

const {
  authState,
  getGenerated,
  generateBalanceTransferCodes,
  deleteGenerated,
  refreshUser,
  showError,
  showSuccess,
  clipboardWriteText
} = vi.hoisted(() => ({
  authState: {
    user: {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    } as Record<string, unknown>
  },
  getGenerated: vi.fn(),
  generateBalanceTransferCodes: vi.fn(),
  deleteGenerated: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  clipboardWriteText: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getGenerated,
    generateBalanceTransferCodes,
    deleteGenerated,
    deleteGeneratedBatch: vi.fn()
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get user() {
      return authState.user
    },
    get canAccessAdminWorkbench() {
      return authState.user?.role === 'admin' || authState.user?.role === 'sub_admin'
    },
    refreshUser
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
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

const paginated = <T,>(items: T[], total = items.length, page = 1, pageSize = 10) => ({
  items,
  total,
  page,
  page_size: pageSize,
  pages: Math.max(1, Math.ceil(total / pageSize))
})

const mountWorkbench = () =>
  mount(AdminWorkbenchView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true
      }
    }
  })

describe('AdminWorkbenchView balance transfer codes', () => {
  beforeEach(() => {
    getGenerated.mockReset()
    generateBalanceTransferCodes.mockReset()
    deleteGenerated.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    clipboardWriteText.mockReset()
    getGenerated.mockResolvedValue(paginated<GeneratedRedeemCode>([]))
    refreshUser.mockResolvedValue(undefined)
    clipboardWriteText.mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: clipboardWriteText },
      configurable: true
    })
  })

  it('loads generated codes for the workbench owner', async () => {
    getGenerated.mockResolvedValueOnce(
      paginated<GeneratedRedeemCode>([
        {
          id: 88,
          code: 'WORKBENCH-CODE',
          type: 'balance',
          value: 8,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-08-20T12:00:00Z',
          created_by: 7,
          source: 'user_balance_transfer'
        }
      ])
    )

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getGenerated).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('WORKBENCH-CODE')
  })

  it('generates balance redeem codes and refreshes account state', async () => {
    generateBalanceTransferCodes.mockResolvedValue([
      {
        id: 89,
        code: 'NEW-CODE',
        type: 'balance',
        value: 5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        created_by: 7,
        source: 'user_balance_transfer'
      }
    ])

    const wrapper = mountWorkbench()
    await flushPromises()

    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('5')
    await wrapper.get('[data-test="workbench-transfer-count"]').setValue('2')
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()

    expect(generateBalanceTransferCodes).toHaveBeenCalledWith({
      amount: 5,
      count: 2,
      expires_in_days: 30,
      notes: '',
      single_use_per_user: false
    })
    expect(refreshUser).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.generated')
    expect(wrapper.text()).toContain('NEW-CODE')
  })
})
