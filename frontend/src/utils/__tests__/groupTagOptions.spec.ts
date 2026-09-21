import { describe, expect, it } from 'vitest'

import { buildReusableGroupTagOptions } from '../groupTagOptions'

describe('buildReusableGroupTagOptions', () => {
  it('deduplicates custom tags, ignores presets and blank values, and keeps a valid color', () => {
    expect(buildReusableGroupTagOptions([
      { tag: ' 专属高速 ', tag_color: '' },
      { tag: '专属高速', tag_color: '#16a34a' },
      { tag: 'chat', tag_color: '#2563eb' },
      { tag: 'image', tag_color: '#10b981' },
      { tag: '  ', tag_color: '#ef4444' },
      { tag: '备用线路', tag_color: '#INVALID' },
    ])).toEqual([
      { tag: '专属高速', color: '#16A34A', count: 2 },
      { tag: '备用线路', color: '', count: 1 },
    ])
  })

  it('sorts by usage count before the display label', () => {
    expect(buildReusableGroupTagOptions([
      { tag: '乙', tag_color: '' },
      { tag: '甲', tag_color: '' },
      { tag: '甲', tag_color: '' },
    ])).toEqual([
      { tag: '甲', color: '', count: 2 },
      { tag: '乙', color: '', count: 1 },
    ])
  })
})
