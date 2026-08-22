import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdminWorkbenchView from '../AdminWorkbenchView.vue'
import type { GeneratedRedeemCode } from '@/api/redeem'

const {
  authState,
  getGenerated,
  generateBalanceTransferCodes,
  deleteGenerated,
  deleteGeneratedBatch,
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
  deleteGeneratedBatch: vi.fn(),
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
    deleteGeneratedBatch
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

const defaultWindowInnerWidth = window.innerWidth
let getBoundingClientRectSpy: ReturnType<typeof vi.spyOn> | undefined
let confirmSpy: ReturnType<typeof vi.spyOn> | undefined

function mockRect(height: number): DOMRect {
  return {
    x: 0,
    y: 0,
    width: 420,
    height,
    top: 0,
    left: 0,
    right: 420,
    bottom: height,
    toJSON: () => ({})
  } as DOMRect
}

describe('AdminWorkbenchView balance transfer codes', () => {
  beforeEach(() => {
    getGenerated.mockReset()
    generateBalanceTransferCodes.mockReset()
    deleteGenerated.mockReset()
    deleteGeneratedBatch.mockReset()
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

  afterEach(() => {
    getBoundingClientRectSpy?.mockRestore()
    getBoundingClientRectSpy = undefined
    confirmSpy?.mockRestore()
    confirmSpy = undefined
    Object.defineProperty(window, 'innerWidth', {
      value: defaultWindowInnerWidth,
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
          source: 'user_balance_transfer',
          notes: 'customer campaign A'
        }
      ])
    )

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getGenerated).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('WORKBENCH-CODE')
    expect(wrapper.text()).toContain('customer campaign A')
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
        source: 'user_balance_transfer',
        notes: 'new batch note'
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
    expect(wrapper.text()).toContain('new batch note')
  })

  it('keeps the latest generated batch in a fixed-height scroll panel', async () => {
    Object.defineProperty(window, 'innerWidth', {
      value: 1280,
      configurable: true
    })
    getBoundingClientRectSpy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function () {
      if ((this as HTMLElement).dataset.test === 'workbench-transfer-form') {
        return mockRect(486)
      }
      return mockRect(0)
    })

    const generatedBatch = Array.from({ length: 25 }, (_, index) => ({
      id: 900 + index,
      code: `BATCH-CODE-${String(index + 1).padStart(2, '0')}`,
      type: 'balance',
      value: 1,
      status: 'unused',
      used_by: null,
      used_at: null,
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7,
      source: 'user_balance_transfer'
    })) as GeneratedRedeemCode[]
    generateBalanceTransferCodes.mockResolvedValue(generatedBatch)

    const wrapper = mountWorkbench()
    await flushPromises()

    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('1')
    await wrapper.get('[data-test="workbench-transfer-count"]').setValue('25')
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()

    const resultCard = wrapper.get('[data-test="workbench-generated-now-card"]')
    const resultList = wrapper.get('[data-test="workbench-generated-results"]')
    expect(wrapper.findAll('[data-test="workbench-generated-code"]')).toHaveLength(25)
    expect(resultCard.classes()).toContain('overflow-hidden')
    expect(resultCard.attributes('style')).toContain('height: 486px')
    expect(resultList.classes()).toEqual(expect.arrayContaining(['min-h-0', 'flex-1', 'overflow-y-auto']))
    expect(wrapper.text()).toContain('BATCH-CODE-25')
  })

  it('batch deletes selected deletable generated codes', async () => {
    const unusedCode: GeneratedRedeemCode = {
      id: 101,
      code: 'UNUSED-CODE',
      type: 'balance',
      value: 3,
      status: 'unused',
      used_by: null,
      used_at: null,
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7,
      source: 'user_balance_transfer'
    }
    const usedCode: GeneratedRedeemCode = {
      id: 102,
      code: 'USED-CODE',
      type: 'balance',
      value: 4,
      status: 'used',
      used_by: 22,
      used_at: '2026-08-20T13:00:00Z',
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7,
      source: 'user_balance_transfer'
    }
    getGenerated.mockResolvedValueOnce(paginated<GeneratedRedeemCode>([unusedCode, usedCode]))
    deleteGeneratedBatch.mockResolvedValueOnce([unusedCode])
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountWorkbench()
    await flushPromises()

    const batchDeleteButton = wrapper.get('[data-test="delete-selected-generated-codes"]')
    expect(batchDeleteButton.attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="select-generated-code-101"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="select-generated-code-102"]').exists()).toBe(false)

    await wrapper.get('[data-test="select-generated-code-101"]').setValue(true)
    expect(wrapper.text()).toContain('adminWorkbench.balanceTransfer.selectedCount')
    expect(batchDeleteButton.attributes('disabled')).toBeUndefined()

    await batchDeleteButton.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.batchDeleteConfirm')
    expect(deleteGeneratedBatch).toHaveBeenCalledWith([101])
    expect(refreshUser).toHaveBeenCalled()
    expect(getGenerated).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.batchDeleted')
  })
})
