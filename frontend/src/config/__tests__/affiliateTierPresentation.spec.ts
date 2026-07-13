import { describe, expect, it } from 'vitest'

import enDashboard from '@/i18n/locales/en/dashboard'
import zhDashboard from '@/i18n/locales/zh/dashboard'
import {
  getAffiliateTierPresentation,
  normalizeAffiliateTier
} from '../affiliateTierPresentation'

const tierCases = [
  {
    level: 'standard',
    theme: 'origin',
    featuredMetric: 'invited',
    zhLabel: '原点级',
    enLabel: 'Origin',
    placeholders: []
  },
  {
    level: 'bronze',
    theme: 'pulse',
    featuredMetric: 'qualified',
    zhLabel: '脉冲级',
    enLabel: 'Pulse',
    placeholders: ['count']
  },
  {
    level: 'silver',
    theme: 'orbit',
    featuredMetric: 'history',
    zhLabel: '星环级',
    enLabel: 'Orbit',
    placeholders: ['count', 'ratio', 'rebate']
  },
  {
    level: 'gold',
    theme: 'core',
    featuredMetric: 'rate',
    zhLabel: '极核级',
    enLabel: 'Core',
    placeholders: ['qualified', 'rate', 'rebate']
  }
] as const

function resolveMessage(locale: unknown, key: string): string | undefined {
  let value = locale

  for (const segment of key.split('.')) {
    if (typeof value !== 'object' || value === null || !(segment in value)) return undefined
    value = (value as Record<string, unknown>)[segment]
  }

  return typeof value === 'string' ? value : undefined
}

function interpolationPlaceholders(message: string): string[] {
  return [...message.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort()
}

describe('affiliate tier presentation', () => {
  it.each([
    ['standard', 'standard'],
    ['bronze', 'bronze'],
    ['silver', 'silver'],
    ['gold', 'gold'],
    ['future-tier', 'standard'],
    ['', 'standard'],
    [null, 'standard'],
    [undefined, 'standard']
  ] as const)('normalizes runtime tier %s to %s', (level, expected) => {
    expect(normalizeAffiliateTier(level)).toBe(expected)
  })

  it.each(tierCases)('maps $level to the approved presentation', ({ level, theme, featuredMetric }) => {
    const presentation = getAffiliateTierPresentation(level)

    expect(presentation).toMatchObject({
      theme,
      featuredMetric,
      labelKey: `affiliate.tiers.levels.${level}`,
      objectiveKey: `affiliate.tiers.objectives.${theme}`
    })
  })

  it.each(tierCases)('returns an immutable $level presentation', ({ level }) => {
    expect(Object.isFrozen(getAffiliateTierPresentation(level))).toBe(true)
  })

  it.each(tierCases)(
    'resolves actual locale contracts for $level',
    ({ level, zhLabel, enLabel, placeholders }) => {
      const presentation = getAffiliateTierPresentation(level)
      const zhLabelMessage = resolveMessage(zhDashboard, presentation.labelKey)
      const enLabelMessage = resolveMessage(enDashboard, presentation.labelKey)
      const zhObjective = resolveMessage(zhDashboard, presentation.objectiveKey)
      const enObjective = resolveMessage(enDashboard, presentation.objectiveKey)

      expect(zhLabelMessage).toBe(zhLabel)
      expect(enLabelMessage).toBe(enLabel)
      expect(zhObjective).toEqual(expect.any(String))
      expect(enObjective).toEqual(expect.any(String))
      const zhPlaceholders = interpolationPlaceholders(zhObjective!)
      const enPlaceholders = interpolationPlaceholders(enObjective!)
      expect(zhPlaceholders).toEqual(enPlaceholders)
      expect(zhPlaceholders).toEqual(placeholders)
    }
  )

  it.each(['future-tier', '', null, undefined])(
    'falls back to Origin for unknown runtime value %s',
    (level) => {
      expect(getAffiliateTierPresentation(level)).toEqual(
        getAffiliateTierPresentation('standard')
      )
    }
  )
})
