import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const redeemViewSource = readFileSync(resolve(currentDir, '../RedeemView.vue'), 'utf8')

describe('RedeemView visual layout', () => {
  it('uses the unified brand surfaces for the redeem experience', () => {
    expect(redeemViewSource).toContain('redeem-shell')
    expect(redeemViewSource).toContain('redeem-balance-card')
    expect(redeemViewSource).toContain('brand-surface')
    expect(redeemViewSource).toContain('redeem-code-panel')
    expect(redeemViewSource).toContain('redeem-info-panel')
    expect(redeemViewSource).toContain('redeem-history-panel')
  })
})
