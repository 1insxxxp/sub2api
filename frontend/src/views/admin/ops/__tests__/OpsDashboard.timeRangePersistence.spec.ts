import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../OpsDashboard.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('OpsDashboard time range persistence', () => {
  it('restores local time range only as the fallback below route query state', () => {
    expect(componentSource).toContain('loadOpsTimeRangeSnapshot(')
    expect(componentSource).toContain("const timeRange = ref<TimeRange>((persistedTimeRange?.timeRange as TimeRange | undefined) ?? '1h')")
    expect(componentSource).toMatch(
      /const nextTimeRange = readQueryString\(QUERY_KEYS\.timeRange\)[\s\S]*?if \(nextTimeRange && allowedTimeRanges\.has\(nextTimeRange as TimeRange\)\)[\s\S]*?timeRange\.value = nextTimeRange as TimeRange/
    )
  })

  it('persists the selected range together with custom endpoints', () => {
    expect(componentSource).toContain('saveOpsTimeRangeSnapshot(')
    expect(componentSource).toContain('customStartTime: nextTimeRange === \'custom\' ? nextStartTime : null')
    expect(componentSource).toContain('customEndTime: nextTimeRange === \'custom\' ? nextEndTime : null')
  })
})
