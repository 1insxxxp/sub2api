import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
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
    'checkin.eligibilityEitherSatisfied': 'Usage or recharge requirement reached',
    'checkin.eligibilityEitherPending': 'Reach either usage or recharge requirement',
    'checkin.usageCriterion': 'Cumulative usage',
    'checkin.rechargeCriterion': 'Cumulative recharge',
    'checkin.criterionProgress': '{current} / {min}',
    'checkin.rewardBreakdown': 'Reward breakdown',
    'checkin.baseReward': 'Random reward',
    'checkin.previousDayUsage': "Yesterday's usage",
    'checkin.usageRebate': 'Usage rebate',
    'checkin.estimatedUsageRebate': 'Estimated usage rebate',
    'checkin.creditedToday': 'Credited today',
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
      plugins: [createPinia()],
      stubs: {
        AnnouncementBell: true,
        LocaleSwitcher: true,
        SubscriptionProgressMini: true,
        RouterLink: {
          props: ['to'],
          template: '<a><slot /></a>'
        }
      },
      mocks: {
        $t: (key: string) => key
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

describe('AppHeader shared admin shell', () => {
  beforeEach(() => {
    getCheckinStatus.mockReset()
    getCheckinStatus.mockResolvedValue({
      enabled: false,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-17',
      reward_amount: null
    })
  })

  it('renders the unified header toolbar chrome and balance pill hook', async () => {
    const wrapper = await mountHeader()

    expect(wrapper.get('header').classes()).toContain('app-header-shell')
    expect(wrapper.get('.app-header-toolbar').exists()).toBe(true)
    expect(wrapper.get('.app-header-actions').exists()).toBe(true)
    expect(wrapper.get('[data-test="header-balance-pill"]').exists()).toBe(true)
  })

  it('uses the shared default avatar in the header when no custom avatar exists', async () => {
    authStore.user = {
      id: 12,
      username: 'xiapeng8618',
      email: 'xiapeng8618@example.com',
      role: 'user',
      balance: 0,
      avatar_url: ''
    }

    const wrapper = await mountHeader()

    expect(wrapper.get('[data-test="header-user-avatar"]').find('[data-testid="default-user-avatar"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="header-user-avatar"]').text()).not.toContain('XI')
  })

  it('keeps the user dropdown compact on desktop', async () => {
    const wrapper = await mountHeader()

    await wrapper.get('button[aria-label="User Menu"]').trigger('click')

    const menu = wrapper.get('.profile-menu')
    expect(menu.classes()).toContain('w-[19rem]')
    expect(menu.classes()).not.toContain('w-[22rem]')
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
    expect(button.attributes('aria-expanded')).toBe('true')

    button.element.parentElement?.dispatchEvent(new MouseEvent('mouseleave'))
    await wrapper.vm.$nextTick()
    expect(button.attributes('aria-expanded')).toBe('true')
  })

  it('keeps the check-in button visible on mobile when enabled', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-05',
      reward_amount: null
    })

    const wrapper = await mountHeader()
    const buttonWrapper = wrapper.get('[data-test="daily-checkin-button"]').element.parentElement

    expect(buttonWrapper?.classList.contains('hidden')).toBe(false)
    expect(buttonWrapper?.classList.contains('sm:inline-flex')).toBe(false)
    expect(buttonWrapper?.classList.contains('inline-flex')).toBe(true)
  })

  it('compacts the check-in label below 360px without hiding the action', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-05',
      reward_amount: null
    })

    const wrapper = await mountHeader()
    const button = wrapper.get('[data-test="daily-checkin-button"]')
    const label = button.get('[data-test="daily-checkin-label"]')

    expect(button.attributes('aria-label')).toBe('签到')
    expect(label.classes()).toContain('hidden')
    expect(label.classes()).toContain('min-[360px]:inline')
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
      min_total_recharge_usd: 0,
      total_recharge_usd: 0,
      next_streak_rule: {
        day: 7,
        bonus_amount: 10,
        bonus_rate_percent: 99
      },
      recent_records: []
    })

    const wrapper = await mountHeader()
    const text = wrapper.text()

    expect(text).toContain('Check-in eligibility')
    expect(text).toContain('Cumulative spend reached $10.00; current $18.50')
    expect(text).toContain('Reward breakdown')
    expect(text).toContain('Random reward')
    expect(text).toContain('No streak bonus today')
    expect(text).toContain('Streak bonus rules')
    expect(text).toContain('Streak day 7 earns an extra $10.00')
    expect(text).not.toContain('99%')
  })

  it('shows previous-day usage and the deterministic rebate estimate before check-in', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 4,
      previous_day_usage_amount: 50,
      estimated_usage_rebate: 4,
      min_total_usage_usd: 0,
      total_usage_usd: 50,
      min_total_recharge_usd: 0,
      total_recharge_usd: 0,
      recent_records: []
    })

    const wrapper = await mountHeader()

    expect(wrapper.get('[data-test="checkin-base-reward"]').text()).toContain('checkin.randomReward')
    expect(wrapper.get('[data-test="checkin-previous-day-usage"]').text()).toContain('$50.00')
    expect(wrapper.get('[data-test="checkin-usage-rebate"]').text()).toContain('$4.00')
  })

  it('shows the actual reward breakdown and credits the total reward after check-in', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 6,
      lifetime_checkin_days: 6,
      previous_day_usage_amount: 50,
      estimated_usage_rebate: 4,
      min_total_usage_usd: 0,
      total_usage_usd: 50,
      min_total_recharge_usd: 0,
      total_recharge_usd: 0,
      recent_records: []
    })
    submitCheckin.mockResolvedValue({
      enabled: true,
      eligible: true,
      blacklisted: false,
      checked_in: true,
      already_checked_in: false,
      checkin_date: '2026-06-16',
      reward_amount: 3,
      base_reward_amount: 0.8,
      previous_day_usage_amount: 50,
      usage_rebate_amount: 4,
      estimated_usage_rebate: 4,
      bonus_reward_amount: 0,
      reward_cap_adjustment: 0,
      total_reward_amount: 4.8,
      current_streak: 7,
      lifetime_checkin_days: 7,
      min_total_usage_usd: 0,
      total_usage_usd: 50,
      min_total_recharge_usd: 0,
      total_recharge_usd: 0,
      balance_before: 10,
      balance_after: 14.8,
      recent_records: []
    })

    const wrapper = await mountHeader()
    await wrapper.get('[data-test="daily-checkin-button"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="checkin-base-reward"]').text()).toContain('$0.80')
    expect(wrapper.get('[data-test="checkin-previous-day-usage"]').text()).toContain('$50.00')
    expect(wrapper.get('[data-test="checkin-usage-rebate"]').text()).toContain('$4.00')
    expect(wrapper.get('[data-test="checkin-streak-bonus"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-test="checkin-total-reward"]').text()).toContain('$4.80')
    expect(showSuccess).toHaveBeenCalledWith('签到成功，获得 $4.80')
  })

  it('shows zero usage, capped rebates, and auditable recent record details', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 2,
      previous_day_usage_amount: 1000,
      estimated_usage_rebate: 8,
      min_total_usage_usd: 0,
      total_usage_usd: 1000,
      min_total_recharge_usd: 0,
      total_recharge_usd: 0,
      recent_records: [
        {
          id: 1,
          user_id: 12,
          checkin_date: '2026-06-15',
          streak_day: 2,
          base_reward_amount: 0.5,
          previous_day_usage_amount: 0,
          usage_rebate_amount: 0,
          bonus_reward_amount: 0,
          reward_cap_adjustment: 0,
          total_reward_amount: 0.5,
          reward_amount: 0.5,
          balance_before: 10,
          balance_after: 10.5,
          created_at: '2026-06-15T01:00:00Z'
        }
      ]
    })

    const wrapper = await mountHeader()

    expect(wrapper.get('[data-test="checkin-usage-rebate"]').text()).toContain('$8.00')
    expect(wrapper.get('[data-test="recent-checkin-previous-day-usage"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-test="recent-checkin-usage-rebate"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-test="recent-checkin-total"]').text()).toContain('$0.50')
  })

  it('keeps the reward breakdown viewport-bound on narrow screens', () => {
    expect(componentSource).toContain('max-w-[calc(100vw-1rem)]')
    expect(componentSource).toContain('daily-checkin-popover')
    expect(componentSource).toContain('daily-checkin-popover-body')
    expect(componentSource).toContain('max-height: min(42rem, calc(100dvh - 5rem))')
    expect(componentSource).toContain('overflow-y: auto')
    expect(componentSource).toContain('@media (max-width: 420px)')
    expect(componentSource).toContain(':global(.app-header-shell)')
    expect(componentSource).toContain('backdrop-filter: none')
    expect(componentSource).toContain('position: fixed')
    expect(componentSource).toContain('top: auto')
    expect(componentSource).toContain('bottom: max(0.75rem, env(safe-area-inset-bottom, 0px))')
    expect(componentSource).toContain('max-height: min(38rem, calc(100dvh')
    expect(componentSource).toContain('grid-cols-[minmax(0,1fr),auto]')
    expect(componentSource).toContain('break-words')
    expect(componentSource).toContain('tabular-nums')
  })

  it('lets an already checked-in user open and close reward details without submitting again', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: true,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: 1,
      current_streak: 2,
      lifetime_checkin_days: 5,
      recent_records: []
    })

    const wrapper = await mountHeader()
    const button = wrapper.get('[data-test="daily-checkin-button"]')

    expect(button.attributes('disabled')).toBeUndefined()
    expect(button.attributes('aria-expanded')).toBe('false')
    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('true')
    expect(submitCheckin).not.toHaveBeenCalled()

    await wrapper.get('header').trigger('keydown', { key: 'Escape' })
    expect(button.attributes('aria-expanded')).toBe('false')
  })

  it('lets an ineligible user inspect eligibility details without submitting', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: false,
      ineligible_reason: 'insufficient_spend',
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 0,
      min_total_usage_usd: 50,
      total_usage_usd: 5,
      recent_records: []
    })

    const wrapper = await mountHeader()
    const button = wrapper.get('[data-test="daily-checkin-button"]')

    expect(button.attributes('disabled')).toBeUndefined()
    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('true')
    expect(submitCheckin).not.toHaveBeenCalled()
  })

  it('preserves desktop pointer preview without submitting', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 0,
      recent_records: []
    })

    const wrapper = await mountHeader()
    const container = wrapper.get('[data-test="daily-checkin-button"]').element.parentElement
    const button = wrapper.get('[data-test="daily-checkin-button"]')

    container?.dispatchEvent(new MouseEvent('mouseenter'))
    await wrapper.vm.$nextTick()
    expect(button.attributes('aria-expanded')).toBe('true')
    expect(submitCheckin).not.toHaveBeenCalled()

    container?.dispatchEvent(new MouseEvent('mouseleave'))
    await wrapper.vm.$nextTick()
    expect(button.attributes('aria-expanded')).toBe('false')
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
      min_total_recharge_usd: 0,
      total_recharge_usd: 0,
      recent_records: []
    })

    const wrapper = await mountHeader()
    const progress = wrapper.get('[data-test="daily-checkin-usage-progress"]')

    expect(progress.attributes('style')).toContain('width: 100%')
  })

  it('renders usage and recharge progress and enables check-in when recharge qualifies', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: true,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 0,
      min_total_usage_usd: 10,
      total_usage_usd: 4,
      min_total_recharge_usd: 20,
      total_recharge_usd: 20,
      recent_records: []
    })

    const wrapper = await mountHeader()

    expect(wrapper.get('[data-test="daily-checkin-usage-progress"]').attributes('style')).toContain('width: 40%')
    expect(wrapper.get('[data-test="daily-checkin-recharge-progress"]').attributes('style')).toContain('width: 100%')
    expect(wrapper.text()).toContain('Cumulative usage')
    expect(wrapper.text()).toContain('Cumulative recharge')
    expect(wrapper.get('[data-test="daily-checkin-button"]').attributes('disabled')).toBeUndefined()
  })

  it('omits disabled eligibility criteria', async () => {
    getCheckinStatus.mockResolvedValue({
      enabled: true,
      eligible: false,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-16',
      reward_amount: null,
      current_streak: 0,
      lifetime_checkin_days: 0,
      min_total_usage_usd: 0,
      total_usage_usd: 100,
      min_total_recharge_usd: 20,
      total_recharge_usd: 5,
      ineligible_reason: 'insufficient_usage_or_recharge',
      recent_records: []
    })

    const wrapper = await mountHeader()

    expect(wrapper.find('[data-test="daily-checkin-usage-progress"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="daily-checkin-recharge-progress"]').attributes('style')).toContain('width: 25%')
    expect(wrapper.get('[data-test="daily-checkin-button"]').attributes('disabled')).toBeUndefined()
  })
})

describe('AppHeader user menu', () => {
  beforeEach(() => {
    getCheckinStatus.mockReset()
    getCheckinStatus.mockResolvedValue({
      enabled: false,
      checked_in: false,
      blacklisted: false,
      checkin_date: '2026-06-17',
      reward_amount: null
    })
    authStore.user = {
      id: 12,
      username: 'alice',
      email: 'alice@example.com',
      role: 'admin',
      balance: 10,
      avatar_url: ''
    }
  })

  it('renders a compact narrow account menu', async () => {
    const wrapper = await mountHeader()

    await wrapper.get('[aria-label="User Menu"]').trigger('click')
    await flushPromises()

    const menu = wrapper.get('[role="menu"]')
    expect(menu.classes()).toContain('w-[19rem]')
    expect(menu.classes()).not.toContain('w-[22rem]')

    const avatar = wrapper.get('.profile-menu-avatar')
    expect(avatar.classes()).toContain('h-9')
    expect(avatar.classes()).toContain('w-9')
    expect(avatar.classes()).not.toContain('h-11')

    expect(componentSource).toContain('min-height: 36px')
    expect(componentSource).toContain('border-radius: 10px')
    expect(componentSource).not.toContain('min-height: 44px')
    expect(componentSource).not.toContain('border-radius: 20px;')
  })
})
