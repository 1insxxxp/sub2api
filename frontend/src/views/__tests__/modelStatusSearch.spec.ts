import { describe, expect, it } from 'vitest'
import { filterModelStatusGroups, normalizeModelStatusSearch } from '../modelStatusSearch'

const groups = [
  { id: 1, name: 'Gemini', platform: 'gemini', models: [
    { name: 'Gemini 3.1 Pro', platform: 'gemini', status: 'healthy' },
    { name: 'Claude Opus 5', platform: 'anthropic', status: 'healthy' },
  ] },
]

describe('model status search', () => {
  it('normalizes whitespace and case', () => {
    expect(normalizeModelStatusSearch('  Gemini 3.1 PRO ')).toBe('gemini 3.1 pro')
  })

  it('filters models by fuzzy name and removes empty groups', () => {
    expect(filterModelStatusGroups(groups as never, ' opus ')).toEqual([
      { ...groups[0], models: [groups[0].models[1]] },
    ])
  })

  it('returns all groups for an empty query', () => {
    expect(filterModelStatusGroups(groups as never, '')).toEqual(groups)
  })
})
