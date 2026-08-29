import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../AccountsView.vue'), 'utf8')

describe('AccountsView upstream billing refresh', () => {
  it('refreshes compact upstream billing data after probe snapshots are patched', () => {
    expect(source).toContain('const refreshUpstreamBillingRates = async (force = false) =>')
    expect(source).toContain('await refreshUpstreamBillingSortedList(true)')
    expect(source).toMatch(
      /const refreshAccountsAfterUpstreamBillingProbe = async \(\) => \{[\s\S]*?await refreshUpstreamBillingSortedList\(true\)/,
    )
  })

  it('shares compact synchronization between single and batch probes', () => {
    expect(source.match(/refreshAccountsAfterUpstreamBillingProbe\(\)/g)).toHaveLength(2)
  })
})
