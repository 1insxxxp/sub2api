import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../AccountsView.vue'), 'utf8')

describe('AccountsView upstream billing refresh', () => {
  it('keeps single and batch probe results in the current list without a follow-up refresh', () => {
    expect(source).not.toContain('refreshAccountsAfterUpstreamBillingProbe')
  })

  it('patches successful single and batch probe snapshots into their account rows', () => {
    expect(source.match(/patchUpstreamBillingSnapshot\([^)]*snapshot\)/g)?.length).toBeGreaterThanOrEqual(2)
  })
})
