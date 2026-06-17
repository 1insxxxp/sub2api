import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))

const pageFiles = [
  'AccountsView.vue',
  'SubscriptionsView.vue',
  'ProxiesView.vue',
  'RedeemView.vue',
  'AnnouncementsView.vue',
  'PromoCodesView.vue'
] as const

const highTrafficListFiles = [
  'UsersView.vue',
  'UsageView.vue',
  'SubscriptionsView.vue',
  'ChannelsView.vue',
  'AccountsView.vue',
  'ProxiesView.vue',
  'GroupsView.vue',
  'AnnouncementsView.vue',
  'PromoCodesView.vue'
] as const

const adminDetailComponentFiles = [
  '../../../components/admin/account/AccountStatsModal.vue',
  '../../../components/admin/account/ScheduledTestsPanel.vue',
  '../../../components/admin/monitor/MonitorTemplateApplyPickerDialog.vue',
  '../../../components/admin/ErrorPassthroughRulesModal.vue'
] as const

const sharedAdminComponentFiles = [
  '../../../components/account/CreateAccountModal.vue',
  '../../../components/account/EditAccountModal.vue',
  '../../../components/account/AccountTestModal.vue',
  '../../../components/account/AccountStatsModal.vue',
  '../../../components/account/AccountUsageCell.vue',
  '../../../components/account/UsageProgressBar.vue',
  '../../../components/account/ModelWhitelistSelector.vue',
  '../../../components/charts/ModelDistributionChart.vue'
] as const

describe('admin legacy page layout unification', () => {
  it.each(pageFiles)('%s uses the shared admin hero and toolbar structure', (fileName) => {
    const source = readFileSync(resolve(currentDir, `../${fileName}`), 'utf8')

    expect(source).toContain('data-test="admin-page-hero"')
    expect(source).toContain('admin-page-hero')
    expect(source).toContain('admin-page-meta-chip')
    expect(source).toContain('admin-toolbar')
    expect(source).toContain('admin-toolbar-group')
  })

  it.each(highTrafficListFiles)('%s keeps dropdowns and row actions on shared admin surfaces', (fileName) => {
    const source = readFileSync(resolve(currentDir, `../${fileName}`), 'utf8')

    expect(source).not.toMatch(/rounded-lg border border-gray-200 bg-white/)
    expect(source).not.toMatch(/hover:bg-gray-100/)
    expect(source).not.toMatch(/flex flex-col items-center gap-0\.5 rounded-lg p-1\.5/)
    expect(source).not.toMatch(/@apply\s+admin-[\w-]+/)
  })

  it.each([
    'GroupsView.vue',
    'ProxiesView.vue',
    'RedeemView.vue',
    'RiskControlView.vue',
    'SettingsView.vue',
    'BackupView.vue',
    'SubscriptionsView.vue'
  ] as const)('%s keeps dense dialog panels on shared admin surfaces', (fileName) => {
    const source = readFileSync(resolve(currentDir, `../${fileName}`), 'utf8')

    expect(source).toMatch(/admin-form-section|admin-list-surface|admin-choice-card/)
    expect(source).not.toContain('overflow-hidden rounded-lg border border-gray-200 bg-gray-50/50')
    expect(source).not.toContain('rounded-lg bg-gray-50 p-3')
    expect(source).not.toContain('relative overflow-hidden rounded-xl border border-gray-200 bg-white')
    expect(source).not.toContain('group relative rounded-xl border border-gray-200 bg-white p-4')
    expect(source).not.toContain('rounded-lg bg-gray-50 p-4')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-gray-50 p-4')
    expect(source).not.toContain('border-gray-200 text-gray-700 hover:bg-gray-50')
    expect(source).not.toContain('w-full resize-none rounded-lg border border-gray-200 bg-gray-50')
    expect(source).not.toContain('inline-flex rounded-lg bg-gray-100 p-1 dark:bg-dark-700')
    expect(source).not.toContain('rounded-xl border border-gray-100 bg-white p-4 shadow-sm')
    expect(source).not.toContain('overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600')
    expect(source).not.toContain('rounded-lg border border-gray-300 bg-white')
    expect(source).not.toContain('rounded bg-gray-100 px-2 py-0.5')
    expect(source).not.toContain('rounded bg-gray-100 px-2 py-1')
    expect(source).not.toContain('bg-gray-50 focus:bg-white')
    expect(source).not.toContain('rounded-xl border-2 border-dashed border-gray-300 bg-white')
    expect(source).not.toContain('grid grid-cols-2 gap-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-700')
  })

  it.each(adminDetailComponentFiles)('%s keeps small admin controls on shared button/pill styles', (relativeFile) => {
    const source = readFileSync(resolve(currentDir, relativeFile), 'utf8')

    expect(source).not.toContain('rounded-lg bg-gray-100 px-4 py-2')
    expect(source).not.toContain('rounded-lg bg-gray-100 px-3 py-1.5')
    expect(source).not.toContain('rounded bg-gray-100 px-1.5 py-0.5')
    expect(source).not.toContain('rounded bg-gray-100 text-xs')
  })

  it.each(sharedAdminComponentFiles)('%s avoids legacy gray controls when reused in admin flows', (relativeFile) => {
    const source = readFileSync(resolve(currentDir, relativeFile), 'utf8')

    expect(source).not.toContain('rounded-lg bg-gray-100 px-4 py-2')
    expect(source).not.toContain('rounded-lg bg-gray-100 px-3 py-1.5')
    expect(source).not.toContain('inline-flex rounded-lg bg-gray-100 p-1')
    expect(source).not.toContain('flex rounded-lg bg-gray-100 p-1')
    expect(source).not.toContain('rounded bg-gray-100 px-1.5 py-0.5')
    expect(source).not.toContain('rounded bg-gray-100 px-2 py-1')
  })
})
