import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../SubscriptionProgressMini.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('SubscriptionProgressMini mobile layout', () => {
  it('uses a teleported, dismissible centered modal and coordinates with other header panels', () => {
    expect(componentSource).toContain('subscription-progress-backdrop')
    expect(componentSource).toContain('subscription-progress-sheet')
    expect(componentSource).toContain(':disabled="!isMobileTooltip"')
    expect(componentSource).toContain('left-1/2 top-1/2')
    expect(componentSource).toContain('-translate-x-1/2 -translate-y-1/2')
    expect(componentSource).not.toContain('inset-x-2 bottom-2')
    expect(componentSource).toContain('app-header-floating-panel-open')
  })
})
