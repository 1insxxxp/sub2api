import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'
import type { GeneratedRedeemCode, RedeemHistoryItem } from '@/api/redeem'

const {
  authState,
  getHistory,
  getGenerated,
  generateBalanceTransferCode,
  generateBalanceTransferCodes,
  deleteGenerated,
  deleteGeneratedBatch,
  getPublicSettings,
  refreshUser,
  showError,
  showSuccess,
  clipboardWriteText
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
  deleteGeneratedBatch: vi.fn(),
  getPublicSettings: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  clipboardWriteText: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getHistory,
    getGenerated,
    generateBalanceTransferCode,
    generateBalanceTransferCodes,
    deleteGenerated,
    deleteGeneratedBatch,
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
        Icon: true,
        Pagination: {
          props: ['page', 'total', 'pageSize'],
          template: '<div data-test="redeem-pagination">{{ page }} / {{ total }} / {{ pageSize }}</div>'
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
    deleteGeneratedBatch.mockReset()
    getPublicSettings.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    clipboardWriteText.mockReset()

    getHistory.mockResolvedValue(paginated<RedeemHistoryItem>([]))
    getGenerated.mockResolvedValue(paginated<GeneratedRedeemCode>([]))
    getPublicSettings.mockResolvedValue({ contact_info: '' })
    refreshUser.mockResolvedValue(undefined)
    clipboardWriteText.mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: clipboardWriteText },
      configurable: true
    })
  })

  it('hides the generator for users without permission', async () => {
    const wrapper = mountRedeemView()
    await flushPromises()

    expect(wrapper.find('[data-test="balance-transfer-panel"]').exists()).toBe(false)
    expect(getGenerated).not.toHaveBeenCalled()
    expect(getHistory).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('loads generated codes and recent activity with independent pagination', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getHistory.mockResolvedValue(paginated<RedeemHistoryItem>([
      {
        id: 33,
        code: 'HISTORY-CODE',
        type: 'balance',
        value: 1,
        status: 'used',
        used_at: '2026-08-20T12:00:00Z',
        created_at: '2026-08-20T12:00:00Z'
      }
    ], 21))
    getGenerated.mockResolvedValue(paginated<GeneratedRedeemCode>([
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
    ], 12))

    const wrapper = mountRedeemView()
    await flushPromises()

    expect(getHistory).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(getGenerated).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.findAll('[data-test="redeem-pagination"]')).toHaveLength(2)
  })

  it('batch-generates balance redeem codes and refreshes the account state', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getGenerated.mockResolvedValueOnce(paginated<GeneratedRedeemCode>([])).mockResolvedValueOnce(
      paginated<GeneratedRedeemCode>([
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
    )
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
    expect(getGenerated).toHaveBeenNthCalledWith(1, { page: 1, page_size: 10 })
    expect(getGenerated).toHaveBeenNthCalledWith(2, { page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('SEND-THIS-CODE')
    expect(wrapper.get('[data-test="generated-codes-modal"]').text()).toContain('SEND-SECOND-CODE')
    expect(wrapper.text()).toContain('redeem.balanceTransfer.singleUseBadge')
    expect(showSuccess).toHaveBeenCalledWith('redeem.balanceTransfer.generated')
  })

  it('copies and downloads all generated codes from the success modal', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getGenerated
      .mockResolvedValueOnce(paginated<GeneratedRedeemCode>([]))
      .mockResolvedValueOnce(paginated<GeneratedRedeemCode>([]))
    generateBalanceTransferCodes.mockResolvedValue([
      {
        id: 88,
        code: 'FIRST-CODE',
        type: 'balance',
        value: 3,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 1
      },
      {
        id: 89,
        code: 'SECOND-CODE',
        type: 'balance',
        value: 3,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        source: 'user_balance_transfer',
        created_by: 1
      }
    ])

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('[data-test="balance-transfer-amount"]').setValue('3')
    await wrapper.get('[data-test="balance-transfer-count"]').setValue('2')
    await wrapper.get('[data-test="balance-transfer-form"]').trigger('submit')
    await flushPromises()

    await wrapper.get('[data-test="generated-codes-copy-all"]').trigger('click')
    expect(clipboardWriteText).toHaveBeenCalledWith('FIRST-CODE\nSECOND-CODE')

    let exportedBlob: Blob | null = null
    let downloadedText = ''
    const OriginalBlob = globalThis.Blob
    vi.stubGlobal('Blob', vi.fn((parts: BlobPart[], options?: BlobPropertyBag) => {
      downloadedText = parts.map((part) => String(part)).join('')
      return new OriginalBlob(parts, options)
    }))
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:redeem-codes'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL

    await wrapper.get('[data-test="generated-codes-download"]').trigger('click')

    expect(exportedBlob).not.toBeNull()
    expect(downloadedText).toBe('FIRST-CODE\nSECOND-CODE')
    expect(clickSpy).toHaveBeenCalled()

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    vi.unstubAllGlobals()
    clickSpy.mockRestore()
  })

  it('deletes an unused generated code and refreshes balance state', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getGenerated.mockResolvedValueOnce(
      paginated<GeneratedRedeemCode>([
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
      ])
    ).mockResolvedValueOnce(paginated<GeneratedRedeemCode>([]))
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
    expect(getGenerated).toHaveBeenNthCalledWith(1, { page: 1, page_size: 10 })
    expect(getGenerated).toHaveBeenNthCalledWith(2, { page: 1, page_size: 10 })
    expect(showSuccess).toHaveBeenCalledWith('redeem.balanceTransfer.deleted')

    confirmSpy.mockRestore()
  })

  it('batch deletes selected unused generated codes and refreshes balance state', async () => {
    authState.user = {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    }
    getGenerated.mockResolvedValueOnce(
      paginated<GeneratedRedeemCode>([
        {
          id: 88,
          code: 'DELETE-FIRST',
          type: 'balance',
          value: 7.5,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-08-20T12:00:00Z',
          source: 'user_balance_transfer',
          created_by: 1
        },
        {
          id: 89,
          code: 'DELETE-SECOND',
          type: 'balance',
          value: 3,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-08-20T12:00:00Z',
          source: 'user_balance_transfer',
          created_by: 1
        }
      ])
    ).mockResolvedValueOnce(paginated<GeneratedRedeemCode>([]))
    deleteGeneratedBatch.mockResolvedValue([])
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('[data-test="select-generated-code-88"]').setValue(true)
    await wrapper.get('[data-test="select-generated-code-89"]').setValue(true)
    await wrapper.get('[data-test="delete-selected-generated-codes"]').trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith('redeem.balanceTransfer.batchDeleteConfirm')
    expect(deleteGeneratedBatch).toHaveBeenCalledWith([88, 89])
    expect(refreshUser).toHaveBeenCalled()
    expect(getGenerated).toHaveBeenCalledTimes(2)
    expect(getGenerated).toHaveBeenNthCalledWith(1, { page: 1, page_size: 10 })
    expect(getGenerated).toHaveBeenNthCalledWith(2, { page: 1, page_size: 10 })
    expect(showSuccess).toHaveBeenCalledWith('redeem.balanceTransfer.batchDeleted')

    confirmSpy.mockRestore()
  })
})
