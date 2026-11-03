import type { ModelAlias } from '@/types'

export type ModelAliasFormRow = ModelAlias

export const createModelAliasFormRow = (alias: Partial<ModelAlias> = {}): ModelAliasFormRow => ({
  alias: alias.alias ?? '',
  upstream_model: alias.upstream_model ?? '',
})

export const modelAliasesToAPI = (rows: ModelAliasFormRow[]): ModelAlias[] => rows
  .map((row) => {
    const alias = row.alias.trim()
    const upstreamModel = row.upstream_model.trim()
    if (!alias || !upstreamModel) return null
    return { alias, upstream_model: upstreamModel }
  })
  .filter((row): row is ModelAlias => row !== null)

export function validateModelAliases(rows: ModelAliasFormRow[]): string | null {
  const seen = new Set<string>()
  for (const row of rows) {
    const alias = row.alias.trim()
    const upstream = row.upstream_model.trim()
    if (!alias || !upstream) return 'modelAliasRequired'
    if ([...alias].length > 100 || [...upstream].length > 200 || /\s/.test(alias) || /\s/.test(upstream)) return 'modelAliasNameInvalid'
    if (alias.toLowerCase() === upstream.toLowerCase()) return 'modelAliasSameTarget'
    if (seen.has(alias.toLowerCase())) return 'modelAliasDuplicate'
    seen.add(alias.toLowerCase())
  }
  if (rows.some((row) => seen.has(row.upstream_model.trim().toLowerCase()))) return 'modelAliasTargetConflict'
  return null
}
