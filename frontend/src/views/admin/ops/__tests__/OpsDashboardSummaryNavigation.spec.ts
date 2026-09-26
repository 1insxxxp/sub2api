import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../OpsDashboard.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('OpsDashboard grouped summary navigation', () => {
  it('keeps the grouped summary open while opening an error detail', () => {
    const summaryHandler = componentSource.split('function openSummaryError')[1]?.split('function openSlaSummaryError')[0] || ''
    const slaHandler = componentSource.split('function openSlaSummaryError')[1]?.split('function onTimeRangeChange')[0] || ''

    expect(summaryHandler).toContain('showErrorModal.value = true')
    expect(summaryHandler).not.toContain('showUpstreamSummary.value = false')
    expect(summaryHandler).not.toContain('showSlaSummary.value = false')
    expect(slaHandler).toContain('showErrorModal.value = true')
    expect(slaHandler).not.toContain('showSlaSummary.value = false')
    expect(slaHandler).not.toContain('showUpstreamSummary.value = false')
  })
})
