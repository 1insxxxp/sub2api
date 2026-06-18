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
    expect(redeemViewSource).toContain('redeem-balance-content')
    expect(redeemViewSource).toContain('redeem-balance-main')
    expect(redeemViewSource).toContain('redeem-concurrency-metric')
    expect(redeemViewSource).toContain('redeem-concurrency-icon')
    expect(redeemViewSource).toContain('redeem-concurrency-value')
    expect(redeemViewSource).not.toContain('redeem-balance-glass')
    expect(redeemViewSource).not.toContain('redeem-concurrency-card')
    expect(redeemViewSource).toContain('brand-surface')
    expect(redeemViewSource).toContain('redeem-code-panel')
    expect(redeemViewSource).toContain('redeem-info-panel')
    expect(redeemViewSource).toContain('redeem-history-panel')
  })

  it('keeps recent activity interaction states on the brand palette', () => {
    expect(redeemViewSource).toContain('class="redeem-history-row"')
    expect(redeemViewSource).toContain('class="redeem-history-card"')
    expect(redeemViewSource).not.toContain('brand-floating-card redeem-history-row')
    expect(redeemViewSource).toContain('.redeem-history-row:hover .redeem-history-card')
    expect(redeemViewSource).toContain('.redeem-history-row:focus-within .redeem-history-card')
    expect(redeemViewSource).toContain('.redeem-history-card:hover')
    expect(redeemViewSource).toContain('border: 1px solid rgba(191, 219, 254, 0.88)')
    expect(redeemViewSource).toContain('border-color: rgba(var(--brand-rgb), 0.52)')
    expect(redeemViewSource).toContain('inset 0 0 0 1px rgba(var(--brand-rgb), 0.32)')
    expect(redeemViewSource).toContain('rgba(var(--brand-cyan-rgb), 0.16)')
    expect(redeemViewSource).toContain('border: 0 !important')
    expect(redeemViewSource).toContain('outline: 0 !important')
    expect(redeemViewSource).toContain('box-shadow: none !important')
    expect(redeemViewSource).toContain('.redeem-history-card,\n  .redeem-input')
    expect(redeemViewSource).toContain('.redeem-history-row:hover .redeem-history-card')
    expect(redeemViewSource).not.toContain('border-color: transparent !important')
    expect(redeemViewSource).not.toContain('border-color: rgb(17, 24, 39)')
    expect(redeemViewSource).not.toContain('border-color: black')
    expect(redeemViewSource).not.toContain('outline: 1px solid')
    expect(redeemViewSource).not.toContain('outline: auto')
  })
})
