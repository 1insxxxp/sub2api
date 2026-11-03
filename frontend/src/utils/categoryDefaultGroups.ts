import type { GroupPlatform, KeyDisplayCategory } from '@/types'
import { getKeyGroupProvider, type KeyGroupProvider } from './keyGroupProviders'

export const CATEGORY_DEFAULT_CATEGORIES = ['anthropic', 'openai', 'gemini', 'grok', 'domestic', 'other'] as const
export type CategoryDefaultCategory = typeof CATEGORY_DEFAULT_CATEGORIES[number]

interface CategoryGroup {
  id: number
  platform: GroupPlatform
  key_display_category?: KeyDisplayCategory
  tag?: string
  is_category_default?: boolean
}

export function sortCategoryDefaultsFirst<T extends { is_category_default?: boolean }>(groups: readonly T[]): T[] {
  return [...groups.filter(group => group.is_category_default), ...groups.filter(group => !group.is_category_default)]
}

export function chooseCategoryDefaultGroup(groups: readonly CategoryGroup[], category: KeyGroupProvider, current: number | null, allowAutomaticSelection: boolean): number | null {
  const choices = groups.filter(group => getKeyGroupProvider(group) === category)
  if (choices.some(group => group.id === current)) return current
  if (!allowAutomaticSelection || category === 'image') return null
  return choices.find(group => group.is_category_default)?.id ?? null
}

/** The container platform does not determine the client's tutorial or native protocol. */
export function getCategoryDefaultGuidePlatform(group: Omit<CategoryGroup, 'id'> | null | undefined): GroupPlatform | null {
  if (!group) return null
  if (!group.is_category_default) return group.platform
  const category = getKeyGroupProvider(group)
  if (category === 'anthropic' || category === 'openai' || category === 'gemini' || category === 'grok') return category
  return 'composite'
}
