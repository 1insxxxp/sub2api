import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../OpsDashboard.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('OpsDashboard auto refresh', () => {
  it('keeps polling disabled while retaining manual dashboard fetches', () => {
    expect(componentSource).not.toContain(
      'autoRefreshEnabled.value = settings.auto_refresh_enabled'
    )
    expect(componentSource).toMatch(
      /async function loadDashboardAdvancedSettings\(\)[\s\S]*autoRefreshEnabled\.value = false[\s\S]*autoRefreshCountdown\.value = 0/
    )
    expect(componentSource).toContain('await fetchData()')
  })
})
