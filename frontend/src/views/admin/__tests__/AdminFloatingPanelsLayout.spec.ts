import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))

const floatingPanelFiles = [
  {
    fileName: 'BackupView.vue',
    expectedPanels: 1
  },
  {
    fileName: 'SubscriptionsView.vue',
    expectedPanels: 1
  },
  {
    fileName: 'RedeemView.vue',
    expectedPanels: 3
  }
] as const

const actionMenuFiles = [
  '../UsersView.vue',
  '../AccountsView.vue',
  '../../../components/admin/account/AccountActionMenu.vue',
  '../../../components/admin/user/UserApiKeysModal.vue'
] as const

const countMatches = (source: string, pattern: string) =>
  (source.match(new RegExp(pattern, 'g')) ?? []).length

describe('admin floating panel layout unification', () => {
  it.each(floatingPanelFiles)('%s uses shared brand floating panel surfaces', ({ fileName, expectedPanels }) => {
    const source = readFileSync(resolve(currentDir, `../${fileName}`), 'utf8')

    expect(countMatches(source, 'brand-overlay')).toBeGreaterThanOrEqual(expectedPanels)
    expect(countMatches(source, 'brand-floating-panel')).toBeGreaterThanOrEqual(expectedPanels)
    expect(countMatches(source, 'brand-floating-header')).toBeGreaterThanOrEqual(expectedPanels)
    expect(source).not.toContain('fixed inset-0 bg-black/50')
    expect(source).not.toContain('rounded-xl bg-white')
  })

  it.each(actionMenuFiles)('%s uses shared admin action menu surfaces', (relativeFile) => {
    const source = readFileSync(resolve(currentDir, relativeFile), 'utf8')

    expect(source).toContain('admin-action-menu')
    expect(source).toContain('admin-action-menu-item')
    expect(source).not.toContain('rounded-xl bg-white shadow-lg ring-1 ring-black/5')
    expect(source).not.toContain('rounded-xl bg-white p-4 dark:border-dark-600 dark:bg-dark-800')
    expect(source).not.toContain('hover:bg-gray-100 dark:hover:bg-dark-700')
  })
})
