import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../AccountsView.vue'), 'utf8')

describe('AccountsView upstream billing refresh', () => {
  it('silently synchronizes the current account page after probe snapshots are patched', () => {
    expect(source).toContain('const load = async (options: { silent?: boolean } = {}) =>')
    expect(source).toContain('await baseLoad(options)')
    expect(source).toMatch(
      /const refreshAccountsAfterUpstreamBillingProbe = async \(\) => \{[\s\S]*?await load\(\{ silent: true \}\)/,
    )
  })

  it('shares silent synchronization between single and batch probes', () => {
    expect(source.match(/refreshAccountsAfterUpstreamBillingProbe\(\)/g)).toHaveLength(2)
  })
})
