import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

function getMessage(messages: LocaleMessages, key: string): unknown {
  return key.split('.').reduce<unknown>((value, segment) => {
    if (typeof value !== 'object' || value === null) {
      return undefined
    }
    return (value as LocaleMessages)[segment]
  }, messages)
}

describe.each([
  [
    'zh',
    zh,
    {
      giftCredit: '赠送额度（不计活动门槛）',
      giftCreditHint: '该额度及由其支付的用量不计入充值、签到等活动门槛。',
      giftBadge: '赠送额度',
      dailyChartTitle: '每日收益趋势',
      dailyChartCommission: '收益金额',
      dailyChartEmpty: '暂无每日收益数据',
    },
  ],
  [
    'en',
    en,
    {
      giftCredit: 'Gift credit (excluded from activity thresholds)',
      giftCreditHint:
        'This credit and usage funded by it do not count toward recharge, check-in, or other activity thresholds.',
      giftBadge: 'Gift credit',
      dailyChartTitle: 'Daily earnings trend',
      dailyChartCommission: 'Earnings',
      dailyChartEmpty: 'No daily earnings data',
    },
  ],
])('AdminWorkbench %s locale', (_locale, messages, expected) => {
  it('resolves gift-credit copy from the namespace consumed by the view', () => {
    for (const key of ['giftCredit', 'giftCreditHint', 'giftBadge'] as const) {
      const value = expected[key]
      expect(getMessage(messages, `adminWorkbench.balanceTransfer.${key}`)).toBe(value)
    }
  })

  it('resolves daily earnings chart copy from the commission namespace', () => {
    for (const key of [
      'dailyChartTitle',
      'dailyChartCommission',
      'dailyChartEmpty',
    ] as const) {
      expect(getMessage(messages, `adminWorkbench.commission.${key}`)).toBe(expected[key])
    }
  })

  it('explains that blacklist changes apply prospectively without changing customer access', () => {
    const hint = String(getMessage(messages, 'adminWorkbench.commission.exclusionsHint'))
    if (_locale === 'zh') {
      expect(hint).toContain('仅排除加入后的用量，历史收益不变；移除后新用量恢复计入')
      expect(hint).toContain('排除期间的用量不回填')
      expect(hint).toContain('不影响客户登录、请求、计费或权限')
    } else {
      expect(hint).toContain('earlier earnings stay unchanged')
      expect(hint).toContain('New usage counts again after removal')
      expect(hint).toContain('not backfilled')
      expect(hint).toContain('permissions are unaffected')
    }
    for (const key of ['exclusions', 'exclusionSearch', 'exclusionSearchPlaceholder', 'exclusionSearchFailed', 'excludedCustomers', 'emptyExclusions', 'exclusionStartedAt', 'addExclusion', 'removeExclusion', 'alreadyExcluded', 'exclusionsSaved']) {
      expect(getMessage(messages, `adminWorkbench.commission.${key}`)).toEqual(expect.any(String))
    }
    expect(getMessage(messages, 'adminWorkbench.commission.requestLogs')).toBeUndefined()
  })

  it('does not retain a separate balance-spend chart series label', () => {
    expect(getMessage(messages, 'adminWorkbench.commission.dailyChartActualCost')).toBeUndefined()
  })

  it('does not retain duplicate gift-credit copy under the redeem namespace', () => {
    for (const key of ['giftCredit', 'giftCreditHint', 'giftBadge'] as const) {
      expect(getMessage(messages, `redeem.balanceTransfer.${key}`)).toBeUndefined()
    }
  })
})
