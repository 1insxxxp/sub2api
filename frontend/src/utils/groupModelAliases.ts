import type { ModelAlias } from '@/types'

export type ModelAliasFormRow = {
  alias: string
  upstream_model: string
  anti_truncation: {
    enabled: boolean
    max_rounds: number | null
    max_output_tokens: number | null
    max_duration_seconds: number | null
  }
}

export const createModelAliasFormRow = (alias: Partial<ModelAlias> = {}): ModelAliasFormRow => ({
  alias: alias.alias ?? '',
  upstream_model: alias.upstream_model ?? '',
  anti_truncation: {
    enabled: alias.anti_truncation?.enabled ?? false,
    max_rounds: alias.anti_truncation?.max_rounds ?? null,
    max_output_tokens: alias.anti_truncation?.max_output_tokens ?? null,
    max_duration_seconds: alias.anti_truncation?.max_duration_seconds ?? null,
  },
})

export const modelAliasesToAPI = (rows: ModelAliasFormRow[]): ModelAlias[] => rows
  .map((row) => {
    const alias = row.alias.trim()
    const upstreamModel = row.upstream_model.trim()
    if (!alias || !upstreamModel) return null
    const antiTruncation = row.anti_truncation.enabled
      ? {
          enabled: true,
          ...(positiveInteger(row.anti_truncation.max_rounds) !== undefined ? { max_rounds: positiveInteger(row.anti_truncation.max_rounds) } : {}),
          ...(positiveInteger(row.anti_truncation.max_output_tokens) !== undefined ? { max_output_tokens: positiveInteger(row.anti_truncation.max_output_tokens) } : {}),
          ...(positiveInteger(row.anti_truncation.max_duration_seconds) !== undefined ? { max_duration_seconds: positiveInteger(row.anti_truncation.max_duration_seconds) } : {}),
        }
      : undefined
    return { alias, upstream_model: upstreamModel, ...(antiTruncation ? { anti_truncation: antiTruncation } : {}) }
  })
  .filter((row): row is ModelAlias => row !== null)

const positiveInteger = (value: number | null): number | undefined =>
  value !== null && Number.isInteger(value) && value > 0 ? value : undefined
