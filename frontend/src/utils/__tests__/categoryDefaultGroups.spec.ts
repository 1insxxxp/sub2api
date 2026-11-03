import { describe, expect, it } from 'vitest'
import { CATEGORY_DEFAULT_CATEGORIES, chooseCategoryDefaultGroup, getCategoryDefaultGuidePlatform, sortCategoryDefaultsFirst } from '../categoryDefaultGroups'

const ordinary = { id: 11, platform: 'gemini' as const, key_display_category: 'gemini' as const }
const defaultGroup = { id: 90, platform: 'composite' as const, key_display_category: 'gemini' as const, is_category_default: true }

describe('category default groups', () => {
  it('supports exactly six configurable categories', () => {
    expect(CATEGORY_DEFAULT_CATEGORIES).toEqual(['anthropic', 'openai', 'gemini', 'grok', 'domestic', 'other'])
  })

  it('puts default choices first without changing the relative order or mutating inputs', () => {
    const input = [ordinary, defaultGroup, { ...ordinary, id: 12 }]
    expect(sortCategoryDefaultsFirst(input).map(group => group.id)).toEqual([90, 11, 12])
    expect(input[0]).toBe(ordinary)
  })

  it('selects the current category default only when automatic selection is allowed', () => {
    expect(chooseCategoryDefaultGroup([ordinary, defaultGroup], 'gemini', null, true)).toBe(90)
    expect(chooseCategoryDefaultGroup([ordinary, defaultGroup], 'gemini', null, false)).toBeNull()
    expect(chooseCategoryDefaultGroup([ordinary, defaultGroup], 'image', null, true)).toBeNull()
    expect(chooseCategoryDefaultGroup([ordinary, defaultGroup], 'openai', null, true)).toBeNull()
  })

  it('keeps a manual choice on refresh and clears an unavailable choice without switching it', () => {
    expect(chooseCategoryDefaultGroup([ordinary, defaultGroup], 'gemini', 11, true)).toBe(11)
    expect(chooseCategoryDefaultGroup([defaultGroup], 'gemini', 11, false)).toBeNull()
  })

  it('retains category-specific client guidance for composite default entries', () => {
    expect(getCategoryDefaultGuidePlatform(defaultGroup)).toBe('gemini')
    expect(getCategoryDefaultGuidePlatform({ ...defaultGroup, key_display_category: 'openai' })).toBe('openai')
    expect(getCategoryDefaultGuidePlatform({ ...defaultGroup, key_display_category: 'domestic' })).toBe('composite')
    expect(getCategoryDefaultGuidePlatform(ordinary)).toBe('gemini')
    expect(getCategoryDefaultGuidePlatform(null)).toBeNull()
  })
})
