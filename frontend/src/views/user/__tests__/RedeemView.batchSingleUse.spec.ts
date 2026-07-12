import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'

const { getHistory, redeem, getPublicSettings, showError } = vi.hoisted(() => ({
  getHistory: vi.fn(),
  redeem: vi.fn(),
  getPublicSettings: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: { getHistory, redeem },
  authAPI: { getPublicSettings }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 1, email: 'user@example.com', balance: 12, concurrency: 5 },
    refreshUser: vi.fn()
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
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
  const messages: Record<string, string> = {
    'redeem.redeemFailed': '兑换失败',
    'redeem.failedToRedeem': '兑换失败，请检查兑换码后重试。',
    'redeem.batchSingleUse': '活动兑换码一人限用一次'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

describe('user RedeemView batch single-use error', () => {
  beforeEach(() => {
    getHistory.mockReset()
    redeem.mockReset()
    getPublicSettings.mockReset()
    showError.mockReset()

    getHistory.mockResolvedValue([])
    getPublicSettings.mockResolvedValue({ contact_info: '' })
  })

  it('shows the activity limit message for flattened API errors', async () => {
    redeem.mockRejectedValue({ code: 'REDEEM_BATCH_USER_LIMIT' })

    const wrapper = mount(RedeemView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('#code').setValue('BATCH-CODE-2')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.text()).toContain('活动兑换码一人限用一次')
    expect(showError).toHaveBeenCalledWith('活动兑换码一人限用一次')
  })
})
