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
  redeemAPI: {
    getHistory,
    redeem
  },
  authAPI: {
    getPublicSettings
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: {
      id: 1,
      email: 'user@example.com',
      balance: 12,
      concurrency: 5
    }
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn()
  })
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    fetchUserSubscriptions: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'redeem.currentBalance': 'Current Balance',
    'redeem.concurrency': 'Concurrency',
    'redeem.requests': 'requests',
    'redeem.redeemCodeLabel': 'Redeem code',
    'redeem.redeemCodePlaceholder': 'Enter code',
    'redeem.redeemCodeHint': 'Case sensitive',
    'redeem.redeeming': 'Redeeming',
    'redeem.redeemButton': 'Redeem',
    'redeem.aboutCodes': 'About codes',
    'redeem.codeRule1': 'Rule 1',
    'redeem.codeRule2': 'Rule 2',
    'redeem.codeRule3': 'Rule 3',
    'redeem.codeRule4': 'Rule 4',
    'redeem.recentActivity': 'Recent Activity',
    'redeem.balanceAddedRedeem': 'Balance Added (Redeem)',
    'redeem.balanceAddedAdmin': 'Balance Added (Admin)',
    'redeem.balanceDeductedAdmin': 'Balance Deducted (Admin)',
    'redeem.checkinReward': 'Daily Check-in Reward',
    'redeem.concurrencyAddedRedeem': 'Concurrency Added',
    'redeem.concurrencyAddedAdmin': 'Concurrency Added (Admin)',
    'redeem.concurrencyReducedAdmin': 'Concurrency Reduced (Admin)',
    'redeem.subscriptionAssigned': 'Subscription Assigned',
    'redeem.adminAdjustment': 'Admin Adjustment',
    'common.unknown': 'Unknown'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

describe('user RedeemView check-in reward history', () => {
  beforeEach(() => {
    getHistory.mockReset()
    redeem.mockReset()
    getPublicSettings.mockReset()
    showError.mockReset()

    getPublicSettings.mockResolvedValue({ contact_info: '' })
    getHistory.mockResolvedValue([
      {
        id: 101,
        code: '890eb190-checkin',
        type: 'checkin_reward',
        value: 2,
        status: 'used',
        used_at: '2026-06-15T02:44:13Z',
        created_at: '2026-06-15T02:44:13Z',
        notes: 'daily check-in reward 2026-06-15'
      }
    ])
  })

  it('renders check-in rewards as USD balance activity instead of request activity', async () => {
    const wrapper = mount(RedeemView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Daily Check-in Reward')
    expect(wrapper.text()).toContain('+$2.00')
    expect(wrapper.text()).not.toContain('Unknown')
    expect(wrapper.text()).not.toContain('+2 requests')
  })
})
