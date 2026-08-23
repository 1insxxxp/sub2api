import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('empty response compensation locale copy', () => {
  it('labels the admin entry as a compensation record list instead of a claim queue', () => {
    expect(zh.admin.usage.emptyResponseClaims.tab).toBe('空回补偿列表')
    expect(zh.admin.usage.emptyResponseClaims.claims).toBe('补偿记录数')
    expect(zh.admin.usage.emptyResponseClaims.detailTitle).toBe('空回补偿详情')
    expect(zh.admin.usage.emptyResponseClaims.empty).toContain('补偿记录')
    expect(zh.admin.usage.emptyResponseClaims.loadFailed).toContain('补偿记录')
    expect(zh.admin.usage.emptyResponseClaims.rankings.group).toBe('分组补偿记录排行')

    expect(en.admin.usage.emptyResponseClaims.tab).toBe('Empty response compensation list')
    expect(en.admin.usage.emptyResponseClaims.claims).toBe('Compensation records')
    expect(en.admin.usage.emptyResponseClaims.detailTitle).toBe('Empty response compensation details')
    expect(en.admin.usage.emptyResponseClaims.empty).toContain('compensation records')
    expect(en.admin.usage.emptyResponseClaims.loadFailed).toContain('compensation records')
    expect(en.admin.usage.emptyResponseClaims.rankings.group).toBe('Groups by compensation records')
  })
})
