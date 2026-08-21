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
})
