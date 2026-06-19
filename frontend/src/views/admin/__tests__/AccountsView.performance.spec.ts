import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('AccountsView performance boundaries', () => {
  it('loads account modals lazily instead of bundling them into the initial view chunk', () => {
    const source = readFileSync(
      resolve(__dirname, '../AccountsView.vue'),
      'utf8'
    )

    expect(source).toContain('defineAsyncComponent')
    expect(source).not.toContain("from '@/components/account'")

    for (const component of [
      'CreateAccountModal',
      'EditAccountModal',
      'BulkEditAccountModal',
      'SyncFromCrsModal',
      'TempUnschedStatusModal',
      'ImportDataModal',
      'ReAuthAccountModal',
      'AccountTestModal',
      'AccountStatsModal',
      'ScheduledTestsPanel',
      'ErrorPassthroughRulesModal',
      'TLSFingerprintProfilesModal'
    ]) {
      expect(source).toContain(`const ${component} = defineAsyncComponent`)
    }

    for (const [tag, condition] of [
      ['CreateAccountModal', 'showCreate'],
      ['EditAccountModal', 'showEdit'],
      ['BulkEditAccountModal', 'showBulkEdit'],
      ['SyncFromCrsModal', 'showSync'],
      ['TempUnschedStatusModal', 'showTempUnsched'],
      ['ImportDataModal', 'showImportData'],
      ['ReAuthAccountModal', 'showReAuth'],
      ['AccountTestModal', 'showTest'],
      ['AccountStatsModal', 'showStats'],
      ['ScheduledTestsPanel', 'showSchedulePanel'],
      ['ErrorPassthroughRulesModal', 'showErrorPassthrough'],
      ['TLSFingerprintProfilesModal', 'showTLSFingerprintProfiles']
    ]) {
      expect(source).toMatch(new RegExp(`<${tag}\\s+v-if="${condition}"`))
    }
  })

  it('initializes show-driven lazy dialogs on first mount', () => {
    const dialogFiles = [
      '../../../components/account/CreateAccountModal.vue',
      '../../../components/account/SyncFromCrsModal.vue',
      '../../../components/account/TempUnschedStatusModal.vue',
      '../../../components/admin/account/ImportDataModal.vue',
      '../../../components/admin/account/ReAuthAccountModal.vue',
      '../../../components/admin/account/AccountTestModal.vue',
      '../../../components/admin/account/AccountStatsModal.vue',
      '../../../components/admin/account/ScheduledTestsPanel.vue',
      '../../../components/admin/ErrorPassthroughRulesModal.vue',
      '../../../components/admin/TLSFingerprintProfilesModal.vue'
    ]

    for (const file of dialogFiles) {
      const source = readFileSync(resolve(__dirname, file), 'utf8')
      expect(source, file).toMatch(/props\.show[\s\S]*\{\s*immediate:\s*true\s*\}/)
    }
  })
})
