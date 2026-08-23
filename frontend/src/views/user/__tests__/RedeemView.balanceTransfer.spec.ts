import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'
import type { RedeemHistoryItem } from '@/api/redeem'

const { authState, getHistory, getGenerated, getPublicSettings } = vi.hoisted(() => ({
  authState: {
    user: {
      id: 1,
      email: 'user@example.com',
      balance: 20,
      concurrency: 5,
      balance_redeem_code_enabled: true
    } as Record<string, unknown>
  },
  getHistory: vi.fn(),
  getGenerated: vi.fn(),
  getPublicSettings: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getHistory,
    getGenerated,
    redeem: vi.fn()
  },
  authAPI: { getPublicSettings }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get user() {
      return authState.user
    },
    refreshUser: vi.fn()
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    fetchActiveSubscriptions: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
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

const mountRedeemView = () =>
  mount(RedeemView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true
      }
    }
  })

describe('user RedeemView balance transfer migration', () => {
  beforeEach(() => {
    getHistory.mockReset()
    getGenerated.mockReset()
    getPublicSettings.mockReset()
    getHistory.mockResolvedValue(paginated<RedeemHistoryItem>([]))
    getPublicSettings.mockResolvedValue({ contact_info: '' })
  })

  it('does not expose balance-to-code generation on the user redeem page', async () => {
    const wrapper = mountRedeemView()
    await flushPromises()

    expect(wrapper.find('[data-test="balance-transfer-panel"]').exists()).toBe(false)
    expect(getGenerated).not.toHaveBeenCalled()
    expect(getHistory).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('renders empty-response compensation as balance activity', async () => {
    getHistory.mockResolvedValueOnce(
      paginated<RedeemHistoryItem>([
        {
          id: 901,
          code: 'EMPTY-COMP-50',
          type: 'empty_response',
          value: 1.25,
          status: 'used',
          used_at: '2026-08-22T10:00:00Z',
          created_at: '2026-08-22T10:00:00Z'
        }
      ])
    )

    const wrapper = mountRedeemView()
    await flushPromises()

    expect(wrapper.text()).toContain('redeem.emptyResponseRefund')
    expect(wrapper.text()).toContain('+$1.25')
    expect(wrapper.text()).toContain('redeem.emptyResponseRefundDetail')
    expect(wrapper.text()).not.toContain('EMPTY-CO...')
  })
})
