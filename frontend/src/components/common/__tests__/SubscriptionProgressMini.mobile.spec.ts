import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../SubscriptionProgressMini.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('SubscriptionProgressMini mobile layout', () => {
  it('uses a stable blue-cyan compact trigger without changing its progress content', () => {
    const triggerMarkup = componentSource.match(/<button\s+[\s\S]*?<\/button>/)?.[0]

    expect(triggerMarkup).toContain('data-test="subscription-progress-trigger"')
    expect(triggerMarkup).toContain('subscription-progress-trigger')
    expect(triggerMarkup).toContain('h-9')
    expect(triggerMarkup).toContain('px-2')
    expect(triggerMarkup).toContain('sm:px-3')
    expect(triggerMarkup).toContain('text-blue-600')
    expect(triggerMarkup).toContain('dark:text-cyan-300')
    expect(triggerMarkup).toContain('displaySubscriptions.slice(0, 3)')
    expect(triggerMarkup).toContain('activeSubscriptions.length')
    expect(triggerMarkup).not.toContain('purple')
  })

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
