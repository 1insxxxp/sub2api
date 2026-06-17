import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))

const componentFiles = [
  '../AdminComplianceDialog.vue',
  '../TLSFingerprintProfilesModal.vue',
  '../usage/UsageFilters.vue',
  '../user/UserBalanceHistoryModal.vue',
  '../user/UserBalanceModal.vue',
  '../user/UserEditModal.vue',
  '../user/UserAllowedGroupsModal.vue',
  '../account/ScheduledTestsPanel.vue',
  '../account/ImportDataModal.vue',
  '../proxy/ImportDataModal.vue',
  '../account/AccountTestModal.vue',
  '../usage/UsageCleanupDialog.vue',
  '../group/GroupRateMultipliersModal.vue',
  '../group/GroupRPMOverridesModal.vue',
  '../ErrorPassthroughRulesModal.vue',
  '../monitor/MonitorFormDialog.vue',
  '../monitor/MonitorKeyPickerDialog.vue',
  '../monitor/MonitorRunResultDialog.vue',
  '../monitor/MonitorAdvancedRequestConfig.vue',
  '../monitor/MonitorTemplateManagerDialog.vue',
  '../monitor/MonitorTemplateApplyPickerDialog.vue',
  '../monitor/MonitorActionsCell.vue',
  '../channel/ModelTagInput.vue',
  '../announcements/AnnouncementReadStatusDialog.vue',
  '../announcements/AnnouncementTargetingEditor.vue',
  '../user/UserApiKeysModal.vue',
  '../user/GroupReplaceModal.vue',
  '../channel/PricingEntryCard.vue',
  '../channel/IntervalRow.vue',
  '../account/ReAuthAccountModal.vue',
  '../payment/AdminRefundDialog.vue',
  '../payment/TopUsersLeaderboard.vue'
] as const

describe('admin component surface unification', () => {
  it.each(componentFiles)('%s uses shared admin surfaces instead of legacy white cards', (relativeFile) => {
    const source = readFileSync(resolve(currentDir, relativeFile), 'utf8')

    expect(source).toMatch(/admin-surface|admin-toolbar-surface|admin-form-section|admin-warning-zone|brand-floating-card|admin-action-menu|admin-choice-card|admin-list-surface|admin-tag-input|admin-inline-action/)
    expect(source).not.toContain('class="card p-6"')
    expect(source).not.toContain('rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800')
    expect(source).not.toContain('rounded-xl border border-gray-200 bg-white transition-all dark:border-dark-600 dark:bg-dark-800')
    expect(source).not.toContain('rounded-lg border bg-white shadow-lg dark:bg-gray-800')
    expect(source).not.toContain('hover:bg-gray-100 dark:hover:bg-gray-700')
    expect(source).not.toContain('rounded-xl border border-gray-200 p-4 dark:border-dark-700')
    expect(source).not.toContain('rounded-lg border border-dashed border-gray-300 bg-gray-50')
    expect(source).not.toContain('rounded-xl border border-gray-200 bg-gradient-to-r')
    expect(source).not.toContain('rounded-lg border border-gray-200 p-3 dark:border-dark-600')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-white shadow-lg')
    expect(source).not.toContain('tbody class="divide-y divide-gray-200 bg-white')
    expect(source).not.toContain('class="hover:bg-gray-50 dark:hover:bg-dark-700')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-900')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm dark:border-dark-700 dark:bg-dark-900/60')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-white p-2 dark:border-dark-600 dark:bg-dark-800')
    expect(source).not.toContain('rounded-lg border border-gray-200 dark:border-dark-600')
    expect(source).not.toContain('hover:bg-gray-50 dark:hover:bg-dark-800')
    expect(source).not.toContain('hover:bg-gray-100 hover:text-primary-600')
    expect(source).not.toContain('rounded-lg border border-gray-200 dark:border-dark-600')
    expect(source).not.toContain('tbody class="divide-y divide-gray-200 bg-white')
    expect(source).not.toContain('hover:bg-gray-50 dark:hover:bg-dark-700')
    expect(source).not.toContain('rounded-xl bg-gray-50 p-4 dark:bg-dark-700')
    expect(source).not.toContain('border-gray-200 bg-white hover:border-gray-300')
    expect(source).not.toContain('rounded-2xl border border-gray-200 bg-gray-50')
    expect(source).not.toContain('rounded-2xl border border-gray-200 bg-white p-4 shadow-sm')
    expect(source).not.toContain('rounded-xl border border-gray-200 bg-gray-50')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-gray-50 p-3')
    expect(source).not.toContain('flex items-center gap-3 rounded-xl bg-gray-50 p-4')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-gray-50 p-4')
    expect(source).not.toContain('border-gray-200 bg-white dark:border-dark-500 dark:bg-dark-700')
    expect(source).not.toContain('class="card p-4"')
    expect(source).not.toContain('rounded-lg bg-gray-50 p-3')
    expect(source).not.toContain('hover:bg-gray-50 dark:hover:bg-dark-700')
  })
})
