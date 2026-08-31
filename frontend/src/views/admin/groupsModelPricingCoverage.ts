import type {
  GroupPricingCoverageModel,
  GroupPricingCoverageResponse
} from '@/api/admin/groups'
import type { PricingFormEntry } from '@/components/admin/channel/types'
import type { ModelsListState } from './groupsModelsList'

export const normalizeCoverageModelName = (value: string) => value.trim().toLowerCase()

export const normalizeCoverageModels = (models: string[]) => {
  const seen = new Set<string>()
  const normalized: string[] = []
  for (const raw of models) {
    const model = raw.trim()
    const key = normalizeCoverageModelName(model)
    if (!key || seen.has(key)) continue
    seen.add(key)
    normalized.push(model)
  }
  return normalized
}

export const advertisedModelsForCoverage = (state: ModelsListState) => {
  if (!state.enabled) return []
  return normalizeCoverageModels(
    state.items.length > 0
      ? state.items.filter((item) => item.selected).map((item) => item.id)
      : state.savedModels
  )
}

export const requiredPricingModels = (coverage: GroupPricingCoverageResponse | null) =>
  normalizeCoverageModels(
    (coverage?.models ?? [])
      .filter((entry) => entry.status === 'missing' || entry.status === 'invalid')
      .map((entry) => entry.model)
  )

export const isCoverageResolved = (
  models: string[],
  coverage: GroupPricingCoverageResponse | null
) => {
  const expected = new Set(normalizeCoverageModels(models).map(normalizeCoverageModelName))
  if (expected.size === 0) return true
  const result = new Map(
    (coverage?.models ?? []).map((entry) => [normalizeCoverageModelName(entry.model), entry.status])
  )
  return [...expected].every((model) => result.get(model) === 'priced')
}

export const appendPendingPricingEntries = (
  entries: PricingFormEntry[],
  coverage: GroupPricingCoverageResponse | null,
  createEntry: (model: string) => PricingFormEntry
) => {
  const existing = new Set(
    entries.flatMap((entry) => entry.models).map(normalizeCoverageModelName)
  )
  const pending = requiredPricingModels(coverage)
    .filter((model) => !existing.has(normalizeCoverageModelName(model)))
    .map(createEntry)
  return pending.length > 0 ? [...entries, ...pending] : entries
}

export const coverageStatusByModel = (
  coverage: GroupPricingCoverageResponse | null
) => new Map<string, GroupPricingCoverageModel>(
  (coverage?.models ?? []).map((entry) => [normalizeCoverageModelName(entry.model), entry])
)
