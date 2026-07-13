import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AffiliateTierIdentity from '../AffiliateTierIdentity.vue'
import type { AffiliateTier, AffiliateTierDefinition, UserAffiliateDetail } from '@/types'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        const messages: Record<string, string> = {
          'affiliate.tiers.currentLevel': 'Current automatic level',
          'affiliate.tiers.effectiveRate': 'Effective rebate rate',
          'affiliate.tiers.qualifiedCount': 'Qualified invitees',
          'affiliate.tiers.customRate': 'Custom rebate rate',
          'affiliate.tiers.nextProgress': '{current} / {target} qualified invitees',
          'affiliate.tiers.highestLevel': 'Highest level reached',
          'affiliate.tiers.rulesTitle': 'Promotion level rules',
          'affiliate.tiers.rulesDescription': 'Qualifies after {amount}.',
          'affiliate.tiers.requirement': '{count} qualified invitees',
          'affiliate.tiers.levels.standard': 'Origin',
          'affiliate.tiers.levels.bronze': 'Pulse',
          'affiliate.tiers.levels.silver': 'Orbit',
          'affiliate.tiers.levels.gold': 'Core',
          'affiliate.tiers.identity.stageObjective': 'Current objective',
          'affiliate.tiers.objectives.origin': 'Complete your first qualified invite to activate your promotion path',
          'affiliate.tiers.objectives.pulse': '{count} more qualified invitees to reach Orbit',
          'affiliate.tiers.objectives.orbit': '{count} more qualified invitees to reach Core; current qualified ratio {ratio}, cumulative rebate {rebate}',
          'affiliate.tiers.objectives.core': 'Highest tier reached: {qualified} qualified invitees, {rebate} cumulative rebate, and a {rate} current rebate rate'
        }

        return (messages[key] ?? key).replace(
          /\{(\w+)\}/g,
          (_, name: string) => String(params?.[name] ?? '')
        )
      }
    })
  }
})

const tiers: AffiliateTierDefinition[] = [
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
    aff_history_quota: 234.5,
    automatic_level: 'standard',
    automatic_rebate_rate_percent: 8,
    effective_rebate_rate_percent: 8,
    has_custom_rebate_rate: false,
    custom_rebate_rate_percent: null,
    qualified_invitee_count: 1,
    qualification_amount: 50,
    next_level_invitee_threshold: 3,
    remaining_qualified_invitees: 2,
    tiers,
    invitees: [],
    ...overrides
  }
}

function nextTier(level: AffiliateTier): AffiliateTierDefinition | null {
  const index = tiers.findIndex((tier) => tier.level === level)
  return tiers[index + 1] ?? null
}

function mountIdentity(
  detail: UserAffiliateDetail,
  props: Partial<{ nextTier: AffiliateTierDefinition | null; progress: number; formattedRate: string }> = {}
) {
  return mount(AffiliateTierIdentity, {
    props: {
      detail,
      nextTier: nextTier(detail.automatic_level),
      progress: 25,
      formattedRate: String(detail.effective_rebate_rate_percent),
      ...props
    }
  })
}

describe('AffiliateTierIdentity', () => {
  it.each([
    ['standard', 'origin', 'Origin'],
    ['bronze', 'pulse', 'Pulse'],
    ['silver', 'orbit', 'Orbit'],
    ['gold', 'core', 'Core']
  ] as const)('renders %s as the localized %s identity', (level, theme, label) => {
    const wrapper = mountIdentity(makeDetail({ automatic_level: level }), {
      nextTier: nextTier(level)
    })

    const summary = wrapper.get('[data-testid="tier-summary"]')
    expect(summary.attributes('data-tier-theme')).toBe(theme)
    expect(summary.text()).toContain(label)
    expect(wrapper.get('[data-testid="current-tier-badge"]').attributes('alt')).toBe('')
  })

  it('shows the Origin objective and keeps a zero-invite ratio finite', () => {
    const wrapper = mountIdentity(makeDetail({
      aff_count: 0,
      qualified_invitee_count: 0,
      remaining_qualified_invitees: 3
    }))

    expect(wrapper.text()).toContain('Complete your first qualified invite to activate your promotion path')
    expect(wrapper.text()).not.toMatch(/NaN|Infinity/)
  })

  it('shows the Pulse remaining objective', () => {
    const wrapper = mountIdentity(makeDetail({
      automatic_level: 'bronze',
      qualified_invitee_count: 6,
      remaining_qualified_invitees: 4
    }), { nextTier: tiers[2] })

    expect(wrapper.text()).toContain('4 more qualified invitees to reach Orbit')
  })

  it.each([
    [-4.8, 0],
    [Number.POSITIVE_INFINITY, 0],
    [4.8, 4]
  ])('sanitizes remaining count %s to the non-negative integer %s', (remaining, expected) => {
    const wrapper = mountIdentity(makeDetail({
      automatic_level: 'bronze',
      remaining_qualified_invitees: remaining
    }), { nextTier: tiers[2] })

    expect(wrapper.text()).toContain(`${expected} more qualified invitees to reach Orbit`)
    expect(wrapper.text()).not.toMatch(/NaN|Infinity/)
  })

  it('shows the Orbit remaining objective, qualified ratio, and cumulative rebate', () => {
    const wrapper = mountIdentity(makeDetail({
      automatic_level: 'silver',
      aff_count: 16,
      qualified_invitee_count: 12,
      remaining_qualified_invitees: 18,
      aff_history_quota: 234.5
    }), { nextTier: tiers[3], formattedRate: '12' })

    expect(wrapper.text()).toContain('18 more qualified invitees to reach Core')
    expect(wrapper.text()).toContain('current qualified ratio 75%')
    expect(wrapper.text()).toContain('cumulative rebate $234.50')
  })

  it.each([
    [Number.NaN, 12, '0%'],
    [Number.POSITIVE_INFINITY, 12, '0%'],
    [10, -3, '0%'],
    [10, Number.POSITIVE_INFINITY, '0%'],
    [10, 25, '100%']
  ])('keeps the Orbit ratio finite and clamped for %s invited and %s qualified', (invited, qualified, expected) => {
    const wrapper = mountIdentity(makeDetail({
      automatic_level: 'silver',
      aff_count: invited,
      qualified_invitee_count: qualified,
      remaining_qualified_invitees: 18
    }), { nextTier: tiers[3] })

    expect(wrapper.text()).toContain(`current qualified ratio ${expected}`)
    expect(wrapper.text()).not.toMatch(/NaN|Infinity/)
  })

  it('shows the Core highest-tier objective and omits remaining-to-next content', () => {
    const wrapper = mountIdentity(makeDetail({
      automatic_level: 'gold',
      qualified_invitee_count: 32,
      remaining_qualified_invitees: 0,
      effective_rebate_rate_percent: 15
    }), { nextTier: tiers[1], progress: 100, formattedRate: '15' })

    expect(wrapper.text()).toContain('Highest level reached')
    expect(wrapper.text()).toContain('Highest tier reached: 32 qualified invitees, $234.50 cumulative rebate, and a 15% current rebate rate')
    expect(wrapper.text()).not.toContain('more qualified invitees to reach')
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
  })

  it('sanitizes objective counts, historical rebate, and the effective rate', () => {
    const wrapper = mountIdentity(makeDetail({
      automatic_level: 'gold',
      qualified_invitee_count: Number.POSITIVE_INFINITY,
      remaining_qualified_invitees: -9.8,
      aff_history_quota: Number.NEGATIVE_INFINITY
    }), { nextTier: null, formattedRate: '125.500' })

    expect(wrapper.text()).toContain('Highest tier reached: 0 qualified invitees, $0.00 cumulative rebate, and a 100% current rebate rate')
    expect(wrapper.text()).not.toMatch(/NaN|Infinity|-9/)
  })

  it.each([
    ['not-a-rate', '0%'],
    ['Infinity', '0%'],
    ['-12.5', '0%'],
    ['012.500', '12.5%'],
    ['150', '100%']
  ])('renders formatted rate %s safely as %s', (formattedRate, expected) => {
    const wrapper = mountIdentity(makeDetail(), { formattedRate })

    expect(wrapper.text()).toContain(expected)
    expect(wrapper.text()).not.toMatch(/NaN|Infinity/)
  })

  it.each([
    [Number.NaN, '0'],
    [Number.POSITIVE_INFINITY, '0'],
    [-25, '0'],
    [125, '100']
  ])('sanitizes progress %s to %s percent', (progress, expected) => {
    const wrapper = mountIdentity(makeDetail(), { progress })
    const progressbar = wrapper.get('[role="progressbar"]')

    expect(progressbar.attributes('aria-valuenow')).toBe(expected)
    expect(progressbar.get('.tier-identity__progress').attributes('style')).toContain(`width: ${expected}%`)
  })

  it('gives next-tier progress a localized accessible name', () => {
    const wrapper = mountIdentity(makeDetail({
      qualified_invitee_count: 1
    }), { nextTier: tiers[1], progress: 33 })

    expect(wrapper.get('[role="progressbar"]').attributes('aria-label')).toBe(
      '1 / 3 qualified invitees'
    )
  })

  it('marks a custom effective rate', () => {
    const wrapper = mountIdentity(makeDetail({
      has_custom_rebate_rate: true,
      effective_rebate_rate_percent: 13.5
    }), { formattedRate: '13.5' })

    expect(wrapper.text()).toContain('13.5%')
    expect(wrapper.text()).toContain('Custom rebate rate')
  })

  it('renders four compact rules and marks the current one', () => {
    const wrapper = mountIdentity(makeDetail({ automatic_level: 'silver' }), {
      nextTier: tiers[3]
    })

    const rules = wrapper.findAll('[data-testid="tier-rule"]')
    expect(rules).toHaveLength(4)
    const ruleBadges = wrapper.findAll('[data-testid="tier-rule-badge"]')
    expect(ruleBadges).toHaveLength(4)
    expect(ruleBadges.every((badge) => badge.attributes('alt') === '')).toBe(true)
    expect(rules.map((rule) => rule.attributes('data-current'))).toEqual([
      'false',
      'false',
      'true',
      'false'
    ])
    expect(rules.map((rule) => rule.text())).toEqual(expect.arrayContaining([
      expect.stringContaining('Origin'),
      expect.stringContaining('Pulse'),
      expect.stringContaining('Orbit'),
      expect.stringContaining('Core')
    ]))
  })

  it.each([
    Number.NaN,
    Number.POSITIVE_INFINITY,
    -50
  ])('sanitizes qualification amount %s before formatting it as currency', (qualificationAmount) => {
    const wrapper = mountIdentity(makeDetail({
      qualification_amount: qualificationAmount
    }))

    expect(wrapper.text()).toContain('Qualifies after $0.00.')
    expect(wrapper.text()).not.toMatch(/NaN|Infinity|-\$50/)
  })

  it('sanitizes tier rule invitee thresholds as non-negative integers', () => {
    const unsafeTiers: AffiliateTierDefinition[] = [
      { level: 'standard', min_qualified_invitees: Number.NaN, rate_percent: 8 },
      { level: 'bronze', min_qualified_invitees: Number.POSITIVE_INFINITY, rate_percent: 10 },
      { level: 'silver', min_qualified_invitees: -10, rate_percent: 12 },
      { level: 'gold', min_qualified_invitees: 4.8, rate_percent: 15 }
    ]
    const wrapper = mountIdentity(makeDetail({ tiers: unsafeTiers }))

    expect(wrapper.findAll('[data-testid="tier-rule"]').map((rule) => rule.text())).toEqual([
      expect.stringContaining('0 qualified invitees'),
      expect.stringContaining('0 qualified invitees'),
      expect.stringContaining('0 qualified invitees'),
      expect.stringContaining('4 qualified invitees')
    ])
  })

  it('sanitizes tier rule rates without hiding localized tier labels', () => {
    const unsafeTiers: AffiliateTierDefinition[] = [
      { level: 'standard', min_qualified_invitees: 0, rate_percent: Number.NaN },
      { level: 'bronze', min_qualified_invitees: 3, rate_percent: -5 },
      { level: 'silver', min_qualified_invitees: 10, rate_percent: 12.5 },
      { level: 'gold', min_qualified_invitees: 30, rate_percent: 250 }
    ]
    const wrapper = mountIdentity(makeDetail({ tiers: unsafeTiers }))

    expect(wrapper.findAll('[data-testid="tier-rule"]').map((rule) => rule.text())).toEqual([
      expect.stringContaining('Origin0%'),
      expect.stringContaining('Pulse0%'),
      expect.stringContaining('Orbit12.5%'),
      expect.stringContaining('Core100%')
    ])
  })

  it('reacts to tier, label, and effective rate prop updates', async () => {
    const wrapper = mountIdentity(makeDetail(), { formattedRate: '8' })

    await wrapper.setProps({
      detail: makeDetail({
        automatic_level: 'bronze',
        effective_rebate_rate_percent: 13.5,
        qualified_invitee_count: 4,
        remaining_qualified_invitees: 6
      }),
      nextTier: tiers[2],
      formattedRate: '013.500'
    })

    expect(wrapper.get('[data-testid="tier-summary"]').attributes('data-tier-theme')).toBe('pulse')
    expect(wrapper.text()).toContain('Pulse')
    expect(wrapper.text()).toContain('13.5%')
    expect(wrapper.text()).not.toContain('013.500%')
  })

  it('falls back to the Origin theme and standard badge for an unknown runtime tier', () => {
    const origin = mountIdentity(makeDetail())
    const unknownDetail = makeDetail()
    unknownDetail.automatic_level = 'future-tier' as AffiliateTier

    const wrapper = mountIdentity(unknownDetail, { nextTier: tiers[1] })

    expect(wrapper.get('[data-testid="tier-summary"]').attributes('data-tier-theme')).toBe('origin')
    expect(wrapper.text()).toContain('Origin')
    expect(wrapper.get('[data-testid="current-tier-badge"]').attributes('src')).toBe(
      origin.get('[data-testid="current-tier-badge"]').attributes('src')
    )
    expect(wrapper.get('[data-testid="tier-rule"][data-current="true"]').text()).toContain('Origin')
  })
})
