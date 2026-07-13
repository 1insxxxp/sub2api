import type { AffiliateTier } from '@/types'

export type AffiliateTierTheme = 'origin' | 'pulse' | 'orbit' | 'core'
export type AffiliateFeaturedMetric = 'invited' | 'qualified' | 'history' | 'rate'

export interface AffiliateTierPresentation {
  readonly theme: AffiliateTierTheme
  readonly labelKey: string
  readonly objectiveKey: string
  readonly featuredMetric: AffiliateFeaturedMetric
}

const affiliateTierPresentations = Object.freeze({
  standard: Object.freeze({
    theme: 'origin',
    labelKey: 'affiliate.tiers.levels.standard',
    objectiveKey: 'affiliate.tiers.objectives.origin',
    featuredMetric: 'invited'
  }),
  bronze: Object.freeze({
    theme: 'pulse',
    labelKey: 'affiliate.tiers.levels.bronze',
    objectiveKey: 'affiliate.tiers.objectives.pulse',
    featuredMetric: 'qualified'
  }),
  silver: Object.freeze({
    theme: 'orbit',
    labelKey: 'affiliate.tiers.levels.silver',
    objectiveKey: 'affiliate.tiers.objectives.orbit',
    featuredMetric: 'history'
  }),
  gold: Object.freeze({
    theme: 'core',
    labelKey: 'affiliate.tiers.levels.gold',
    objectiveKey: 'affiliate.tiers.objectives.core',
    featuredMetric: 'rate'
  })
}) satisfies Readonly<Record<AffiliateTier, AffiliateTierPresentation>>

export function getAffiliateTierPresentation(
  level: AffiliateTier | string | null | undefined
): AffiliateTierPresentation {
  if (level && Object.prototype.hasOwnProperty.call(affiliateTierPresentations, level)) {
    return affiliateTierPresentations[level as AffiliateTier]
  }

  return affiliateTierPresentations.standard
}
