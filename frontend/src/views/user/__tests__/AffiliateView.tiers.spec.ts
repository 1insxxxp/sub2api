import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'
import enDashboard from '@/i18n/locales/en/dashboard'
import zhDashboard from '@/i18n/locales/zh/dashboard'
import type { UserAffiliateDetail } from '@/types'

const { getAffiliateDetail } = vi.hoisted(() => ({
  getAffiliateDetail: vi.fn()
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn()
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser: vi.fn() })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        const messages: Record<string, string> = {
          'affiliate.tiers.currentLevel': 'Current automatic level',
          'affiliate.tiers.effectiveRate': 'Effective rebate',
          'affiliate.tiers.qualifiedCount': 'Qualified invitees',
          'affiliate.tiers.customRate': 'Custom rate',
          'affiliate.tiers.nextProgress': '{current} / {target} qualified invitees',
          'affiliate.tiers.remaining': '{count} more to {level}',
          'affiliate.tiers.highestLevel': 'Highest level reached',
          'affiliate.tiers.rulesTitle': 'Promotion levels',
          'affiliate.tiers.rulesDescription': 'Qualifies after {amount}.',
          'affiliate.tiers.levels.standard': 'Origin',
          'affiliate.tiers.levels.bronze': 'Pulse',
          'affiliate.tiers.levels.silver': 'Orbit',
          'affiliate.tiers.levels.gold': 'Core',
          'affiliate.tiers.requirement': '{count} qualified invitees',
          'affiliate.tiers.identity.stageObjective': 'Current objective',
          'affiliate.tiers.identity.featuredMetric': 'Featured metric for current tier',
          'affiliate.tiers.objectives.origin': 'Complete your first qualified invite to activate your promotion path',
          'affiliate.tiers.objectives.pulse': '{count} more qualified invitees to reach Orbit',
          'affiliate.tiers.objectives.orbit': '{count} more qualified invitees to reach Core; current qualified ratio {ratio}, cumulative rebate {rebate}',
          'affiliate.tiers.objectives.core': 'Highest tier reached: {qualified} qualified invitees, {rebate} cumulative rebate, and a {rate} current rebate rate',
          'affiliate.stats.invitedUsers': 'Invited users',
          'affiliate.invitees.columns.paymentProgress': 'Cumulative paid',
          'affiliate.invitees.qualified': 'Qualified',
          'affiliate.invitees.inProgress': 'In progress'
        }
        return (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, name: string) => String(params?.[name] ?? ''))
      }
    })
  }
})

const tiers: UserAffiliateDetail['tiers'] = [
  { level: 'standard', min_qualified_invitees: 0, rate_percent: 8 },
  { level: 'bronze', min_qualified_invitees: 3, rate_percent: 10 },
  { level: 'silver', min_qualified_invitees: 10, rate_percent: 12 },
  { level: 'gold', min_qualified_invitees: 30, rate_percent: 15 }
]

function makeDetail(overrides: Partial<UserAffiliateDetail> = {}): UserAffiliateDetail {
  return {
    user_id: 1,
    aff_code: 'AFF123',
    aff_count: 12,
    aff_quota: 12,
    aff_frozen_quota: 0,
    aff_history_quota: 20,
    automatic_level: 'silver',
    automatic_rebate_rate_percent: 12,
    effective_rebate_rate_percent: 12,
    has_custom_rebate_rate: false,
    custom_rebate_rate_percent: null,
    qualified_invitee_count: 12,
    qualification_amount: 50,
    next_level_invitee_threshold: 30,
    remaining_qualified_invitees: 18,
    tiers,
    invitees: [
      {
        user_id: 2,
        email: 'invitee-with-a-long-address@example.com',
        username: 'long-invitee-name',
        total_rebate: 2.8,
        qualifying_payment_amount: 35,
        qualified: false,
        qualified_at: null,
        created_at: '2026-07-12T00:00:00Z'
      },
      {
        user_id: 3,
        email: 'qualified@example.com',
        username: 'qualified-user',
        total_rebate: 4,
        qualifying_payment_amount: 50,
        qualified: true,
        qualified_at: '2026-07-12T00:00:00Z',
        created_at: '2026-07-11T00:00:00Z'
      }
    ],
    ...overrides
  }
}

async function mountView(detail: UserAffiliateDetail) {
  getAffiliateDetail.mockResolvedValueOnce(detail)
  const wrapper = mount(AffiliateView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true
      }
    }
  })
  await flushPromises()
  return wrapper
}

describe('AffiliateView promotion tiers', () => {
  beforeEach(() => {
    getAffiliateDetail.mockReset()
  })

  it('provides localized accessible text for featured metrics', () => {
    expect(enDashboard.affiliate.tiers.identity.featuredMetric).toBe('Featured metric for current tier')
    expect(zhDashboard.affiliate.tiers.identity.featuredMetric).toBe('当前等级重点指标')
  })

  it('renders one integrated identity with the automatic level, progress, objectives, and all rules', async () => {
    const wrapper = await mountView(makeDetail())

    const summaries = wrapper.findAll('[data-testid="tier-summary"]')
    expect(summaries).toHaveLength(1)
    const summary = summaries[0]
    expect(summary.text()).toContain('Orbit')
    expect(summary.text()).toContain('12%')
    expect(summary.text()).toContain('12')
    expect(summary.text()).toContain('12 / 30 qualified invitees')
    expect(summary.text()).toContain('18 more qualified invitees to reach Core')
    expect(summary.text()).toContain('current qualified ratio 100%')
    expect(summary.text()).toContain('cumulative rebate $20.00')

    const rules = wrapper.findAll('[data-testid="tier-rule"]')
    expect(rules).toHaveLength(4)
    expect(rules.map((rule) => rule.text())).toEqual(expect.arrayContaining([
      expect.stringContaining('Origin'),
      expect.stringContaining('Pulse'),
      expect.stringContaining('Orbit'),
      expect.stringContaining('Core')
    ]))

    const currentBadge = wrapper.get('[data-testid="current-tier-badge"]')
    expect(currentBadge.attributes('alt')).toBe('')

    const ruleBadges = wrapper.findAll('[data-testid="tier-rule-badge"]')
    expect(ruleBadges).toHaveLength(4)
    expect(ruleBadges.every((badge) => badge.attributes('alt') === '')).toBe(true)
    expect(wrapper.get('[data-testid="tier-rule"][data-current="true"]').text()).toContain('Orbit')
    expect(wrapper.html()).not.toContain('tier-badge-gold')
    expect(wrapper.html()).not.toContain('tier-badge-pulse')
  })

  it.each([
    {
      level: 'standard',
      theme: 'origin',
      qualified: 1,
      nextThreshold: 3,
      remaining: 2,
      featuredStat: 'acquisition',
      acquisitionMetric: 'invited',
      primaryLabel: 'Invited users',
      primaryValue: '40',
      secondaryLabel: 'Qualified invitees',
      secondaryValue: '1',
      objective: 'Complete your first qualified invite',
      progressTarget: '1 / 3 qualified invitees'
    },
    {
      level: 'bronze',
      theme: 'pulse',
      qualified: 7,
      nextThreshold: 10,
      remaining: 3,
      featuredStat: 'acquisition',
      acquisitionMetric: 'qualified',
      primaryLabel: 'Qualified invitees',
      primaryValue: '7',
      secondaryLabel: 'Invited users',
      secondaryValue: '40',
      objective: '3 more qualified invitees to reach Orbit',
      progressTarget: '7 / 10 qualified invitees'
    },
    {
      level: 'silver',
      theme: 'orbit',
      qualified: 12,
      nextThreshold: 30,
      remaining: 18,
      featuredStat: 'history',
      acquisitionMetric: 'invited',
      primaryLabel: 'Invited users',
      primaryValue: '40',
      secondaryLabel: 'Qualified invitees',
      secondaryValue: '12',
      objective: 'current qualified ratio 30%',
      progressTarget: '12 / 30 qualified invitees'
    },
    {
      level: 'gold',
      theme: 'core',
      qualified: 32,
      nextThreshold: null,
      remaining: 0,
      featuredStat: 'rate',
      acquisitionMetric: 'invited',
      primaryLabel: 'Invited users',
      primaryValue: '40',
      secondaryLabel: 'Qualified invitees',
      secondaryValue: '32',
      objective: 'Highest tier reached',
      progressTarget: null
    }
  ] as const)('renders coherent $level tier metrics and progress', async ({
    level,
    theme,
    qualified,
    nextThreshold,
    remaining,
    featuredStat,
    acquisitionMetric,
    primaryLabel,
    primaryValue,
    secondaryLabel,
    secondaryValue,
    objective,
    progressTarget
  }) => {
    const wrapper = await mountView(makeDetail({
      automatic_level: level,
      aff_count: 40,
      qualified_invitee_count: qualified,
      next_level_invitee_threshold: nextThreshold,
      remaining_qualified_invitees: remaining
    }))

    expect(wrapper.get('[data-testid="tier-summary"]').attributes('data-tier-theme')).toBe(theme)
    expect(wrapper.get('[data-testid="tier-summary"]').text()).toContain(objective)
    const stats = wrapper.findAll('[data-stat]')
    expect(stats.map((stat) => stat.attributes('data-stat'))).toEqual([
      'rate',
      'acquisition',
      'available',
      'history'
    ])
    const acquisition = wrapper.get('[data-stat="acquisition"]')
    expect(acquisition.attributes('data-metric')).toBe(acquisitionMetric)
    expect(acquisition.get('[data-acquisition="primary"]').text()).toContain(primaryLabel)
    expect(acquisition.get('[data-acquisition="primary"]').text()).toContain(primaryValue)
    expect(acquisition.get('[data-acquisition="secondary"]').text()).toContain(secondaryLabel)
    expect(acquisition.get('[data-acquisition="secondary"]').text()).toContain(secondaryValue)

    const featured = wrapper.get(`[data-stat="${featuredStat}"]`)
    expect(featured.attributes('data-featured')).toBe('true')
    expect(featured.get('.sr-only').text()).toBe('Featured metric for current tier')
    expect(stats.filter((stat) => stat.attributes('data-featured') === 'true')).toHaveLength(1)
    expect(stats.filter((stat) => stat.find('.sr-only').exists())).toHaveLength(1)

    const progressbar = wrapper.find('[role="progressbar"]')
    if (progressTarget) {
      expect(progressbar.exists()).toBe(true)
      expect(progressbar.attributes('aria-label')).toBe(progressTarget)
    } else {
      expect(progressbar.exists()).toBe(false)
    }
  })

  it('labels a custom rate while preserving automatic-level progress', async () => {
    const wrapper = await mountView(makeDetail({
      automatic_level: 'bronze',
      automatic_rebate_rate_percent: 10,
      effective_rebate_rate_percent: 13.5,
      has_custom_rebate_rate: true,
      custom_rebate_rate_percent: 13.5,
      qualified_invitee_count: 7,
      next_level_invitee_threshold: 10,
      remaining_qualified_invitees: 3
    }))

    const summary = wrapper.get('[data-testid="tier-summary"]')
    expect(summary.text()).toContain('Pulse')
    expect(summary.text()).toContain('13.5%')
    expect(summary.text()).toContain('Custom rate')
    expect(summary.text()).toContain('7 / 10 qualified invitees')
    expect(summary.text()).toContain('3 more qualified invitees to reach Orbit')
  })

  it('shows Core as the completed highest level', async () => {
    const wrapper = await mountView(makeDetail({
      automatic_level: 'gold',
      automatic_rebate_rate_percent: 15,
      effective_rebate_rate_percent: 15,
      qualified_invitee_count: 32,
      next_level_invitee_threshold: null,
      remaining_qualified_invitees: 0
    }))

    expect(wrapper.get('[data-testid="tier-summary"]').text()).toContain('Highest level reached')
    expect(wrapper.get('[data-testid="tier-summary"]').attributes('data-tier-theme')).toBe('core')
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="tier-rule"][data-current="true"]').text()).toContain('Core')
  })

  it('declares separate desktop and wrapping mobile invitee layouts without fixed minimum width', async () => {
    const wrapper = await mountView(makeDetail())

    expect(wrapper.get('[data-testid="invitees-desktop"]').classes()).toContain('hidden')
    const mobile = wrapper.get('[data-testid="invitees-mobile"]')
    expect(mobile.classes()).toContain('md:hidden')
    expect(mobile.text()).toContain('$35.00 / $50.00')
    expect(mobile.text()).toContain('In progress')
    expect(mobile.text()).toContain('$50.00 / $50.00')
    expect(mobile.text()).toContain('Qualified')
    expect(wrapper.text()).toContain('AFF123')
    expect(wrapper.text()).toContain('/register?aff=AFF123')
    expect(wrapper.html()).not.toMatch(/min-w-\[/)
    expect(mobile.findAll('.break-all').length).toBeGreaterThan(0)
  })

  // Vitest currently runs this file in jsdom, where layout metrics are always zero.
  // Keep these requirements pending until Task 8 provides the project's browser runner.
  it.todo('at a 320px browser viewport keeps the page and promotion content scrollWidth within clientWidth')
  it.todo('at a 390px browser viewport keeps the page and promotion content scrollWidth within clientWidth')
})
