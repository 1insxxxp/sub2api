import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

const { getCheckinStatus, submitCheckin, showSuccess, showError, refreshUser } = vi.hoisted(() => ({
  getCheckinStatus: vi.fn(),
  submitCheckin: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  refreshUser: vi.fn()
}))

const routeState = reactive({
  name: 'Dashboard',
  meta: {
    titleKey: 'dashboard.title',
    descriptionKey: 'dashboard.welcomeMessage'
  },
  params: {}
})

const authStore = reactive({
  user: {
    id: 12,
    username: 'alice',
    email: 'alice@example.com',
    role: 'user',
    balance: 10,
    avatar_url: ''
  },
  isAdmin: false,
  isSimpleMode: false,
  logout: vi.fn(),
  refreshUser
})

vi.mock('@/api/checkin', () => ({
  getCheckinStatus,
  submitCheckin
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    contactInfo: '',
    docUrl: '',
    cachedPublicSettings: null,
    toggleMobileSidebar: vi.fn(),
    showSuccess,
    showError
  }),
  useAuthStore: () => authStore,
  useOnboardingStore: () => ({
    replay: vi.fn()
  })
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: []
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  }),
  useRoute: () => routeState
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'checkin.action': '签到',
    'checkin.checked': '已签到',
    'checkin.loading': '加载中',
    'checkin.success': '签到成功，获得 $3.00',
    'checkin.failed': '签到失败',
    'checkin.eligibilityTitle': 'Check-in eligibility',
    'checkin.eligibilitySatisfied': 'Cumulative spend reached {min}; current {current}',
    'checkin.eligibilityPending': 'Check-in unlocks at {min} cumulative spend; current {current}',
    'checkin.rewardBreakdown': 'Reward breakdown',
    'checkin.baseReward': 'Base reward',
    'checkin.streakBonus': 'Streak bonus',
    'checkin.noStreakBonusToday': 'No streak bonus today',
    'checkin.streakBonusTitle': 'Streak bonus rules',
    'checkin.nextStreakBonus': 'Streak day {day} earns an extra {amount}',
    'dashboard.title': 'Dashboard',
    'dashboard.welcomeMessage': 'Welcome'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'checkin.success') {
          return `签到成功，获得 $${Number(params?.amount ?? 0).toFixed(2)}`
        }
        const template = messages[key] ?? key
        return template.replace(/\{(\w+)\}/g, (_, name: string) => String(params?.[name] ?? ''))
      }
    })
  }
})

const mountHeader = async () => {
  const AppHeader = (await import('../AppHeader.vue')).default
  const wrapper = mount(AppHeader, {
    global: {
      stubs: {
        AnnouncementBell: true,
        LocaleSwitcher: true,
        SubscriptionProgressMini: true,
        RouterLink: {
          props: ['to'],
          template: '<a><slot /></a>'
        }
      }
    }
  })
  await flushPromises()
  return wrapper
}

describe('AppHeader available token estimate', () => {
  it('does not render or import the temporary available token estimate', () => {
    expect(componentSource).not.toContain('availableTokensLabel')
    expect(componentSource).not.toContain('availableTokensTooltip')
    expect(componentSource).not.toContain('@/utils/tokenBalance')
  })
})

describe('AppHeader daily check-in entry', () => {
  beforeEach(() => {
    getCheckinStatus.mockReset()
    submitCheckin.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    refreshUser.mockReset()
    authStore.user = {
      id: 12,
      username: 'alice',
      email: 'alice@example.com',
      role: 'user',
      balance: 10,
      avatar_url: ''
    }
  })

  it('hides the check-in button for blacklisted users', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      checked_in: false,
      blacklisted: true,
      checkin_date: '2026-06-05',
      reward_amount: null
    })

    const wrapper = await mountHeader()

    expect(getCheckinStatus).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="daily-checkin-button"]').exists()).toBe(false)
  })

  it('marks the button checked and refreshes balance after a successful check-in', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-05',
      reward_amount: null
    })
    submitCheckin.mockResolvedValue({
      enabled: true,
      blacklisted: false,
      checked_in: true,
      already_checked_in: false,
      reward_amount: 3,
      balance_before: 10,
      balance_after: 13,
      checkin_date: '2026-06-05'
    })
    refreshUser.mockImplementation(async () => {
      authStore.user = { ...authStore.user, balance: 13 }
      return authStore.user
    })

    const wrapper = await mountHeader()
    const button = wrapper.get('[data-test="daily-checkin-button"]')

    expect(button.text()).toContain('签到')

    await button.trigger('click')
    await flushPromises()

    expect(submitCheckin).toHaveBeenCalledTimes(1)
    expect(refreshUser).toHaveBeenCalledTimes(1)
    expect(showSuccess).toHaveBeenCalledWith('签到成功，获得 $3.00')
    expect(wrapper.get('[data-test="daily-checkin-button"]').text()).toContain('已签到')
    expect(wrapper.text()).toContain('$13.00')
  })

  it('explains eligibility and streak rewards in the hover panel', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: true,
      blacklisted: false,
      checkin_date: '2026-06-15',
      reward_amount: 2,
      base_reward_amount: 2,
      bonus_reward_amount: 0,
      total_reward_amount: 2,
      current_streak: 1,
      lifetime_checkin_days: 1,
      min_total_usage_usd: 10,
      total_usage_usd: 18.5,
      next_streak_rule: {
        day: 7,
        bonus_amount: 10
      },
      recent_records: []
    })

    const wrapper = await mountHeader()
    const text = wrapper.text()

    expect(text).toContain('Check-in eligibility')
    expect(text).toContain('Cumulative spend reached $10.00; current $18.50')
    expect(text).toContain('Reward breakdown')
    expect(text).toContain('Base reward')
    expect(text).toContain('No streak bonus today')
    expect(text).toContain('Streak bonus rules')
    expect(text).toContain('Streak day 7 earns an extra $10.00')
  })

  it('renders the eligibility progress bar based on cumulative spend and caps at 100%', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 0,
      min_total_usage_usd: 50,
      total_usage_usd: 73.77,
      recent_records: []
    })

    const wrapper = await mountHeader()
    const progress = wrapper.get('[data-test="daily-checkin-eligibility-progress"]')

    expect(progress.attributes('style')).toContain('width: 100%')
  })
})
