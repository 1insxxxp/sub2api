import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'
import type { RedeemHistoryItem } from '@/api/redeem'

const {
  getHistory,
  redeem,
  getPublicSettings,
  refreshUser,
  fetchActiveSubscriptions,
  routerPush
} = vi.hoisted(() => ({
  getHistory: vi.fn(),
  redeem: vi.fn(),
  getPublicSettings: vi.fn(),
  refreshUser: vi.fn(),
  fetchActiveSubscriptions: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getHistory,
    redeem
  },
  authAPI: { getPublicSettings }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 77, email: 'user@example.com', balance: 12, concurrency: 5 },
    refreshUser
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
    fetchActiveSubscriptions
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: routerPush
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'redeem.subscriptionGuide.title': '订阅卡使用说明',
    'redeem.subscriptionGuide.subtitle': '酒馆cc月卡 已开通',
    'redeem.subscriptionGuide.stepApiKeyTitle': '切换 API 密钥分组',
    'redeem.subscriptionGuide.stepApiKeyDesc': '到 API 密钥页面把密钥切换到对应订阅分组后才可使用。',
    'redeem.subscriptionGuide.stepQuotaTitle': '每日限额刷新',
    'redeem.subscriptionGuide.stepQuotaDesc': '每日限额每天 0 点刷新，当日用完为止。',
    'redeem.subscriptionGuide.stepUsageTitle': '使用当前密钥调用',
    'redeem.subscriptionGuide.stepUsageDesc': '切换完成后继续使用原 API 地址和密钥。',
    'redeem.subscriptionGuide.goToKeys': '去 API 密钥页面',
    'redeem.subscriptionGuide.acknowledge': '我知道了',
    'redeem.subscriptionGuide.groupLabel': '订阅分组',
    'redeem.subscriptionGuide.daysLabel': '有效天数'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const value = messages[key] ?? key
        return value.replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? ''))
      }
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

describe('user RedeemView subscription guide', () => {
  beforeEach(() => {
    getHistory.mockReset()
    redeem.mockReset()
    getPublicSettings.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset()
    routerPush.mockReset()
    window.localStorage.clear()

    getHistory.mockResolvedValue(paginated<RedeemHistoryItem>([]))
    getPublicSettings.mockResolvedValue({ contact_info: '' })
    refreshUser.mockResolvedValue(undefined)
    fetchActiveSubscriptions.mockResolvedValue(undefined)
  })

  it('shows the subscription usage guide after the first subscription redeem', async () => {
    redeem.mockResolvedValue({
      message: 'ok',
      type: 'subscription',
      value: 30,
      group_name: '酒馆cc月卡',
      validity_days: 30
    })

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('#code').setValue('SUB-CARD-001')
    await wrapper.get('form.redeem-form').trigger('submit')
    await flushPromises()

    const guide = wrapper.get('[data-test="subscription-redeem-guide"]')
    expect(guide.text()).toContain('订阅卡使用说明')
    expect(guide.text()).toContain('API 密钥页面')
    expect(guide.text()).toContain('每天 0 点刷新')
    expect(guide.text()).toContain('当日用完为止')
    expect(guide.text()).toContain('酒馆cc月卡')
  })

  it('acknowledges the guide per user and can jump to API keys', async () => {
    redeem.mockResolvedValue({
      message: 'ok',
      type: 'subscription',
      value: 30,
      group_name: '酒馆cc月卡',
      validity_days: 30
    })

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('#code').setValue('SUB-CARD-001')
    await wrapper.get('form.redeem-form').trigger('submit')
    await flushPromises()

    await wrapper.get('[data-test="subscription-guide-go-keys"]').trigger('click')

    expect(window.localStorage.getItem('passionapi.subscriptionRedeemGuideSeen.77')).toBe('1')
    expect(routerPush).toHaveBeenCalledWith('/keys')
  })

  it('does not show the guide again after it has been acknowledged', async () => {
    window.localStorage.setItem('passionapi.subscriptionRedeemGuideSeen.77', '1')
    redeem.mockResolvedValue({
      message: 'ok',
      type: 'subscription',
      value: 30,
      group_name: '酒馆cc月卡',
      validity_days: 30
    })

    const wrapper = mountRedeemView()
    await flushPromises()

    await wrapper.get('#code').setValue('SUB-CARD-002')
    await wrapper.get('form.redeem-form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('[data-test="subscription-redeem-guide"]').exists()).toBe(false)
  })
})
