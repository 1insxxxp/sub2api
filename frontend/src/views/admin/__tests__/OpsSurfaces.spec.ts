import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const opsComponentsDir = resolve(currentDir, '../ops/components')

const opsSurfaceFiles = [
  'OpsConcurrencyCard.vue',
  'OpsEmailNotificationCard.vue',
  'OpsSettingsDialog.vue',
  'OpsRuntimeSettingsCard.vue',
  'OpsErrorTrendChart.vue',
  'OpsThroughputTrendChart.vue',
  'OpsErrorDistributionChart.vue',
  'OpsAlertEventsCard.vue',
  'OpsSystemLogTable.vue',
  'OpsErrorDetailModal.vue',
  'OpsDashboardHeader.vue',
  'OpsDashboardSkeleton.vue',
  'OpsAlertRulesCard.vue',
  'OpsRequestDetailsModal.vue',
  'OpsLatencyChart.vue',
  'OpsSwitchRateTrendChart.vue',
  'OpsErrorDetailsModal.vue'
] as const

const legacySurfaceFragments = [
  'rounded-2xl bg-gray-50 p-4',
  'rounded-xl bg-gray-50 p-4',
  'rounded-xl bg-gray-50 p-6',
  'rounded-lg bg-gray-50 p-2.5',
  'rounded-lg bg-gray-50 p-3',
  'rounded-xl border border-gray-200 bg-white p-4',
  'rounded-lg border border-gray-200 bg-gray-50 p-4',
  'rounded-lg border border-gray-200 bg-gray-50 p-3',
  'rounded-lg border border-gray-200 bg-white px-2 py-1',
  'rounded-full border border-gray-200 bg-white px-3 py-1',
  'rounded-lg bg-gray-100 px-3 py-1.5',
  'bg-gray-100 text-gray-500 hover:bg-gray-200',
  'hover:bg-gray-50 dark:hover:bg-dark-700',
  'hover:bg-gray-50 dark:hover:bg-dark-700/50',
  'rounded-2xl border border-gray-200 bg-white p-4 shadow-sm'
] as const

describe('ops admin surface unification', () => {
  it.each(opsSurfaceFiles)('%s uses shared admin surfaces for dense ops UI', (fileName) => {
    const source = readFileSync(resolve(opsComponentsDir, fileName), 'utf8')

    expect(source).toMatch(/admin-surface|admin-form-section|admin-toolbar-surface|admin-list-surface|admin-list-row|admin-choice-card|admin-inline-action|admin-empty-state/)

    for (const fragment of legacySurfaceFragments) {
      expect(source).not.toContain(fragment)
    }
  })
})
