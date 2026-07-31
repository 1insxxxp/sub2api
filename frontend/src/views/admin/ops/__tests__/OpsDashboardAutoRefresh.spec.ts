import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../OpsDashboard.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('OpsDashboard auto refresh', () => {
  it('follows the saved polling setting while retaining manual dashboard fetches', () => {
    expect(componentSource).toContain(
      'autoRefreshEnabled.value = settings.auto_refresh_enabled'
    )
    expect(componentSource).toMatch(
      /const settings = await opsAPI\.getAdvancedSettings\(\)[\s\S]*autoRefreshEnabled\.value = settings\.auto_refresh_enabled[\s\S]*autoRefreshIntervalMs\.value = settings\.auto_refresh_interval_seconds \* 1000/
    )
    expect(componentSource).toContain('await fetchData()')
    expect(componentSource).toContain("document.addEventListener('visibilitychange', handleVisibilityChange)")
    expect(componentSource).toContain("document.removeEventListener('visibilitychange', handleVisibilityChange)")
    expect(componentSource).toContain('if (document.hidden)')
  })
})
