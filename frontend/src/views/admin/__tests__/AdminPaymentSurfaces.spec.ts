import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))

const paymentSurfaceFiles = [
  '../SubscriptionsView.vue',
  '../orders/PlanEditDialog.vue',
  '../orders/AdminOrdersView.vue',
  '../orders/AdminPaymentDashboardView.vue',
  '../affiliates/AdminAffiliateRecordsTable.vue',
] as const

describe('admin payment and subscription surfaces', () => {
  it.each(paymentSurfaceFiles)('%s keeps dense payment forms on shared admin surfaces', (relativeFile) => {
    const source = readFileSync(resolve(currentDir, relativeFile), 'utf8')

    expect(source).toMatch(/admin-form-section|admin-surface|admin-list-row|admin-choice-card/)
    expect(source).not.toContain('rounded-lg bg-gray-50 p-4')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-gray-50 p-3')
    expect(source).not.toContain('hover:bg-gray-100 dark:text-gray-400')
    expect(source).not.toContain('rounded-lg border border-gray-100 bg-gray-50')
    expect(source).not.toContain('hover:bg-gray-50 dark:hover:bg-dark-700')
  })
})
