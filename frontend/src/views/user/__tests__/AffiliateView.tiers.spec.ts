import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'
import enDashboard from '@/i18n/locales/en/dashboard'
import zhDashboard from '@/i18n/locales/zh/dashboard'
import type { UserAffiliateDetail } from '@/types'

const { getAffiliateDetail, claimAffiliateReward } = vi.hoisted(() => ({
  getAffiliateDetail: vi.fn(),
  claimAffiliateReward: vi.fn()
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
    claimAffiliateReward
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
          'affiliate.campaign.eyebrow': 'Growth console',
          'affiliate.campaign.title': 'Invite friends, unlock higher rebates',
          'affiliate.campaign.subtitle': 'Share your link, track qualified purchases, and transfer available rebate quota into balance.',
          'affiliate.campaign.nextTier': '{count} more qualified invitees to unlock {level}',
          'affiliate.campaign.maxTier': 'Top tier unlocked',
          'affiliate.campaign.stepsTitle': '3-step reward path',
          'affiliate.campaign.stepRegisterTitle': 'Friend registers',
          'affiliate.campaign.stepRegisterDescription': 'New users bind to you from the invite link or affiliate code.',
          'affiliate.campaign.stepRechargeTitle': 'Friend pays {amount}',
          'affiliate.campaign.stepRechargeDescription': 'Cumulative paid purchases turn them into a qualified invitee.',
          'affiliate.campaign.stepRewardTitle': 'You earn rebates',
          'affiliate.campaign.stepRewardDescription': 'Recharge rebates follow your current rate and can be moved into balance.',
          'affiliate.campaign.toolsTitle': 'Invite toolkit',
          'affiliate.campaign.toolsDescription': 'Copy the materials users actually need when sharing in groups.',
          'affiliate.campaign.copyPitch': 'Copy pitch',
          'affiliate.campaign.pitchCopied': 'Pitch copied',
          'affiliate.campaign.pitchTemplate': 'Sign up for trial credits and access GPT, Claude, Gemini, and other models: {link}',
          'affiliate.campaign.progressTitle': 'Progress center',
          'affiliate.campaign.progressDescription': 'Qualified invitees drive permanent tier upgrades.',
          'affiliate.campaign.qualifiedRatio': 'Qualified ratio',
          'affiliate.campaign.mobileCta': 'Copy invite link',
          'affiliate.stats.invitedUsers': 'Invited users',
          'affiliate.stats.rebateRate': 'My rebate rate',
          'affiliate.stats.availableQuota': 'Available quota',
          'affiliate.stats.totalQuota': 'Historical quota',
          'affiliate.rewards.title': 'Milestone rewards',
          'affiliate.rewards.description': 'Claim redeem codes after qualified-invite milestones.',
          'affiliate.rewards.requirement': '{count} qualified invitees',
          'affiliate.rewards.balanceBenefit': '{amount} balance reward',
          'affiliate.rewards.subscriptionBenefit': '{group} subscription reward for {days} days',
          'affiliate.rewards.groupFallback': 'Group #{id}',
          'affiliate.rewards.subscriptionGeneric': 'subscription',
          'affiliate.rewards.remaining': '{count} more qualified invitees to claim',
          'affiliate.rewards.claim': 'Claim redeem code',
          'affiliate.rewards.claiming': 'Claiming...',
          'affiliate.rewards.claimSuccess': 'Redeem code claimed: {code}',
          'affiliate.rewards.claimFailed': 'Failed to claim milestone reward',
          'affiliate.rewards.copyCode': 'Copy code',
          'affiliate.rewards.codeCopied': 'Redeem code copied',
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
    rewards: [
      {
        id: 1,
        name: 'Starter bonus',
        description: 'First milestone',
        enabled: true,
        required_qualified_invitees: 3,
        reward_type: 'balance',
        balance_value: 10,
        group_id: null,
        validity_days: 0,
        redeem_expires_in_days: 0,
        sort_order: 1,
        qualified_invitee_count: 12,
        remaining_invitees: 0,
        claimable: false,
        claimed: true,
        claimed_at: '2026-07-12T00:00:00Z',
        redeem_code_id: 88,
        code: 'CLAIMED-CODE'
      },
      {
        id: 2,
        name: 'Orbit trial',
        description: '',
        enabled: true,
        required_qualified_invitees: 10,
        reward_type: 'subscription',
        balance_value: 0,
        group_id: 9,
        group_name: 'GPT Full Models',
        validity_days: 30,
        redeem_expires_in_days: 14,
        sort_order: 2,
        qualified_invitee_count: 12,
        remaining_invitees: 0,
        claimable: true,
        claimed: false
      },
      {
        id: 3,
        name: 'Core bonus',
        description: '',
        enabled: true,
        required_qualified_invitees: 30,
        reward_type: 'balance',
        balance_value: 100,
        group_id: null,
        validity_days: 0,
        redeem_expires_in_days: 0,
        sort_order: 3,
        qualified_invitee_count: 12,
        remaining_invitees: 18,
        claimable: false,
        claimed: false
      }
    ],
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
    claimAffiliateReward.mockReset()
  })

  it('provides localized accessible text for featured metrics', () => {
    expect(enDashboard.affiliate.tiers.identity.featuredMetric).toBe('Featured metric for current tier')
    expect(zhDashboard.affiliate.tiers.identity.featuredMetric).toBe('当前等级重点指标')
  })

  it('keeps the Chinese promotion pitch focused on trial credits and full-model access', () => {
    expect(zhDashboard.affiliate.campaign.pitchTemplate).toContain('注册即送体验额度')
    expect(zhDashboard.affiliate.campaign.pitchTemplate).toContain('GPT、Claude、Gemini 全模型')
    expect(zhDashboard.affiliate.campaign.pitchTemplate).toContain('每天还可以签到领免费额度')
    expect(zhDashboard.affiliate.campaign.pitchTemplate).not.toContain('充值满')
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

  it('renders the promotion console with reward steps, invite tools, progress, and a mobile CTA', async () => {
    const wrapper = await mountView(makeDetail({
      automatic_level: 'bronze',
      qualified_invitee_count: 7,
      next_level_invitee_threshold: 10,
      remaining_qualified_invitees: 3
    }))

    const console = wrapper.get('[data-testid="affiliate-promotion-console"]')
    expect(console.text()).toContain('Growth console')
    expect(console.text()).toContain('Invite friends, unlock higher rebates')
    expect(console.text()).toContain('3 more qualified invitees to unlock Orbit')

    const steps = wrapper.findAll('[data-testid="affiliate-reward-step"]')
    expect(steps).toHaveLength(3)
    expect(steps.map((step) => step.text())).toEqual([
      expect.stringContaining('Friend registers'),
      expect.stringContaining('Friend pays $50.00'),
      expect.stringContaining('You earn rebates')
    ])

    const tools = wrapper.get('[data-testid="affiliate-invite-tools"]')
    expect(tools.text()).toContain('AFF123')
    expect(tools.text()).toContain('/register?aff=AFF123')
    expect(tools.text()).toContain('Copy pitch')
    expect(tools.text()).toContain('GPT, Claude, Gemini')

    const progress = wrapper.get('[data-testid="affiliate-progress-center"]')
    expect(progress.text()).toContain('Progress center')
    expect(progress.text()).toContain('Qualified ratio')
    expect(progress.text()).toContain('58%')

    const mobileCta = wrapper.get('[data-testid="affiliate-mobile-cta"]')
    expect(mobileCta.classes()).toEqual(expect.arrayContaining(['md:hidden', 'sticky']))
    expect(mobileCta.text()).toContain('Copy invite link')
  })

  it('renders milestone reward tasks with claimed, claimable, and locked states', async () => {
    const wrapper = await mountView(makeDetail())

    const panel = wrapper.get('[data-testid="affiliate-milestone-rewards"]')
    expect(panel.text()).toContain('Milestone rewards')

    const tasks = wrapper.findAll('[data-testid="affiliate-reward-task"]')
    expect(tasks).toHaveLength(3)
    expect(tasks[0].attributes('data-state')).toBe('claimed')
    expect(tasks[0].text()).toContain('CLAIMED-CODE')
    expect(tasks[0].text()).toContain('$10.00 balance reward')

    expect(tasks[1].attributes('data-state')).toBe('claimable')
    expect(tasks[1].text()).toContain('GPT Full Models subscription reward for 30 days')
    expect(tasks[1].text()).toContain('Claim redeem code')

    expect(tasks[2].attributes('data-state')).toBe('locked')
    expect(tasks[2].text()).toContain('18 more qualified invitees to claim')
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

  it.each([
    { affCount: Number.NaN, qualifiedCount: Number.POSITIVE_INFINITY },
    { affCount: -12, qualifiedCount: -7 }
  ])('sanitizes invalid acquisition counts to zero', async ({ affCount, qualifiedCount }) => {
    const wrapper = await mountView(makeDetail({
      automatic_level: 'bronze',
      aff_count: affCount,
      qualified_invitee_count: qualifiedCount
    }))

    const acquisition = wrapper.get('[data-stat="acquisition"]')
    expect(acquisition.findAll('[data-acquisition="primary"] p')[1].text()).toBe('0')
    expect(acquisition.get('[data-acquisition="secondary"] strong').text()).toBe('0')
    expect(acquisition.text()).not.toMatch(/NaN|Infinity|-[0-9]/)
  })

  it('rounds positive decimal acquisition counts down consistently', async () => {
    const wrapper = await mountView(makeDetail({
      automatic_level: 'bronze',
      aff_count: 12.9,
      qualified_invitee_count: 7.8
    }))

    const acquisition = wrapper.get('[data-stat="acquisition"]')
    expect(acquisition.findAll('[data-acquisition="primary"] p')[1].text()).toBe('7')
    expect(acquisition.get('[data-acquisition="secondary"] strong').text()).toBe('12')
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
