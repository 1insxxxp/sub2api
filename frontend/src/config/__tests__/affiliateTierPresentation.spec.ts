import { describe, expect, it } from 'vitest'

import { getAffiliateTierPresentation } from '../affiliateTierPresentation'

describe('affiliate tier presentation', () => {
  it.each([
    ['standard', 'origin', 'invited'],
    ['bronze', 'pulse', 'qualified'],
    ['silver', 'orbit', 'history'],
    ['gold', 'core', 'rate']
  ] as const)('maps %s to the approved presentation', (level, theme, featuredMetric) => {
    const presentation = getAffiliateTierPresentation(level)

    expect(presentation).toMatchObject({
      theme,
      featuredMetric,
      labelKey: `affiliate.tiers.levels.${level}`,
      objectiveKey: `affiliate.tiers.objectives.${theme}`
    })
  })

  it.each(['future-tier', '', null, undefined])(
    'falls back to Origin for unknown runtime value %s',
    (level) => {
      expect(getAffiliateTierPresentation(level)).toEqual(
        getAffiliateTierPresentation('standard')
      )
    }
  )
})
