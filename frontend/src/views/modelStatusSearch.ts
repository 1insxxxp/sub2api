import type { ModelStatusGroup } from '@/api/modelStatus'

export function normalizeModelStatusSearch(value: string): string {
  return value.trim().toLocaleLowerCase()
}

export function filterModelStatusGroups(groups: ModelStatusGroup[], query: string): ModelStatusGroup[] {
  const normalized = normalizeModelStatusSearch(query)
  if (!normalized) return groups
  return groups
    .map(group => ({
      ...group,
      models: group.models.filter(model => `${model.name} ${model.platform}`.toLocaleLowerCase().includes(normalized)),
    }))
    .filter(group => group.models.length > 0)
}
