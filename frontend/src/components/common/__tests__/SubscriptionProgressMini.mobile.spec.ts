import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../SubscriptionProgressMini.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('SubscriptionProgressMini mobile layout', () => {
  it('preserves the original purple borderless desktop pill', () => {
    const triggerMarkup = componentSource.match(/<button\s+[\s\S]*?<\/button>/)?.[0]

    expect(triggerMarkup).toContain('data-test="subscription-progress-trigger"')
    expect(triggerMarkup).toContain('class="subscription-progress-trigger flex cursor-pointer items-center gap-2 rounded-xl bg-purple-50 px-3 py-1.5 transition-colors hover:bg-purple-100 dark:bg-purple-900/20 dark:hover:bg-purple-900/30"')
    expect(triggerMarkup).toContain('class="subscription-progress-trigger-icon text-purple-600 dark:text-purple-400"')
    expect(triggerMarkup).toContain('class="subscription-progress-trigger-count text-xs font-medium text-purple-700 dark:text-purple-300"')
    expect(triggerMarkup).not.toContain('border-blue')
    expect(triggerMarkup).not.toContain('shadow-blue')
    expect(triggerMarkup).toContain('displaySubscriptions.slice(0, 3)')
    expect(triggerMarkup).toContain('activeSubscriptions.length')
  })

  it('supplies the 36px blue-cyan trigger theme only from mobile CSS', () => {
    const mobileHeaderCss = styleSource.match(/@media \(max-width: 639px\) \{([\s\S]*?)\n {2}\}\n\n {2}\.app-header-balance-pill/)?.[1]
    const triggerRule = mobileHeaderCss?.match(/\.subscription-progress-trigger\s*\{([^}]*)\}/)?.[1]
    const iconRule = mobileHeaderCss?.match(/\.subscription-progress-trigger-icon\s*\{([^}]*)\}/)?.[1]
    const countRule = mobileHeaderCss?.match(/\.subscription-progress-trigger-count\s*\{([^}]*)\}/)?.[1]

    expect(triggerRule).toContain('width: var(--app-header-mobile-subscription-width)')
    expect(triggerRule).toContain('height: var(--app-header-mobile-control-size)')
    expect(triggerRule).toContain('var(--brand-rgb)')
    expect(triggerRule).toContain('var(--brand-cyan-rgb)')
    expect(iconRule).toContain('var(--brand-600)')
    expect(countRule).toContain('var(--brand-700)')
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
