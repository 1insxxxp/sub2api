import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../SubscriptionsView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('SubscriptionsView mobile layout', () => {
  it('stacks subscription card header content on mobile to avoid badge/action overlap', () => {
    expect(viewSource).toContain('data-test="subscription-card-header"')
    expect(viewSource).toContain('flex-col gap-3')
    expect(viewSource).toContain('sm:flex-row')
    expect(viewSource).toContain('data-test="subscription-card-title-row"')
    expect(viewSource).toContain('min-w-0 flex-1')
    expect(viewSource).toContain('break-words')
    expect(viewSource).toContain('data-test="subscription-card-actions"')
    expect(viewSource).toContain('w-full justify-between')
    expect(viewSource).toContain('sm:w-auto')
    expect(viewSource).toContain('whitespace-nowrap')
  })

  it('allows expiration details to wrap on narrow mobile screens', () => {
    expect(viewSource).toContain('data-test="subscription-expiration-row"')
    expect(viewSource).toContain('flex-col gap-1')
    expect(viewSource).toContain('sm:flex-row')
    expect(viewSource).toContain('break-words text-right')
  })
})
