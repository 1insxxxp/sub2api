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

  it('does not retain a separate balance-spend chart series label', () => {
    expect(getMessage(messages, 'adminWorkbench.commission.dailyChartActualCost')).toBeUndefined()
  })

  it('does not retain duplicate gift-credit copy under the redeem namespace', () => {
    for (const key of ['giftCredit', 'giftCreditHint', 'giftBadge'] as const) {
      expect(getMessage(messages, `redeem.balanceTransfer.${key}`)).toBeUndefined()
    }
  })
})
