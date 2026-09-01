import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdminWorkbenchView from '../AdminWorkbenchView.vue'
import type { GeneratedRedeemCode } from '@/api/redeem'

const {
  authState,
  getGeneratedBalanceRedeemCodes,
  generateBalanceRedeemCodes,
  refreshUser,
  showSuccess
} = vi.hoisted(() => ({
  authState: {
    user: {
      id: 7,
      email: 'operator@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 2
    } as Record<string, unknown>
  },
  getGeneratedBalanceRedeemCodes: vi.fn(),
  generateBalanceRedeemCodes: vi.fn(),
  refreshUser: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getGenerated: getGeneratedBalanceRedeemCodes,
    generateBalanceTransferCodes: generateBalanceRedeemCodes,
    deleteGenerated: vi.fn(),
    deleteGeneratedBatch: vi.fn()
  }
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
    showError: vi.fn(),
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

const mountWorkbench = () =>
  mount(AdminWorkbenchView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: {
          props: ['page', 'total', 'pageSize'],
          template: '<div data-test="workbench-pagination">{{ page }} / {{ total }} / {{ pageSize }}</div>'
        }
      }
    }
  })

const paginated = <T,>(items: T[], total = items.length, page = 1, pageSize = 10) => ({
  items,
  total,
  page,
  page_size: pageSize,
  pages: Math.max(1, Math.ceil(total / pageSize))
})

describe('AdminWorkbenchView balance redeem codes', () => {
  beforeEach(() => {
    getGeneratedBalanceRedeemCodes.mockReset()
    generateBalanceRedeemCodes.mockReset()
    refreshUser.mockReset()
    showSuccess.mockReset()

    getGeneratedBalanceRedeemCodes.mockResolvedValue(paginated<GeneratedRedeemCode>([]))
    refreshUser.mockResolvedValue(undefined)
  })

  it('loads generated balance redeem codes on mount', async () => {
    mountWorkbench()
    await flushPromises()

    expect(getGeneratedBalanceRedeemCodes).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('generates balance redeem codes from the logged-in operator balance', async () => {
    generateBalanceRedeemCodes.mockResolvedValue([
      {
        id: 88,
        code: 'WORKBENCH-CODE',
        type: 'balance',
        value: 7.5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 7,
        single_use_per_user: true
      }
    ] satisfies GeneratedRedeemCode[])

    const wrapper = mountWorkbench()
    await flushPromises()

    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('7.5')
    await wrapper.get('[data-test="workbench-transfer-count"]').setValue('1')
    await wrapper.get('[data-test="workbench-transfer-expiry"]').setValue('14')
    await wrapper.get('[data-test="workbench-transfer-single-use"]').setValue(true)
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()

    expect(generateBalanceRedeemCodes).toHaveBeenCalledWith({
      amount: 7.5,
      count: 1,
      expires_in_days: 14,
      notes: '',
      single_use_per_user: true,
      threshold_exempt: false
    })
    expect(refreshUser).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.generated')
    expect(wrapper.text()).toContain('WORKBENCH-CODE')
  })
})
