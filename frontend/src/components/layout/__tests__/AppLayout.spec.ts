import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppLayout workspace sizing', () => {
  it('keeps module content fluid inside the available workspace', () => {
    expect(componentSource).toContain('class="app-shell-content"')
    expect(componentSource).toContain('min-width: 0;')
    expect(componentSource).toContain('max-width: none;')
    expect(componentSource).toContain('--app-content-padding-y: clamp(')
    expect(componentSource).toContain('--app-content-padding-x: clamp(')
    expect(componentSource).toContain('padding: var(--app-content-padding-y) var(--app-content-padding-x);')
    expect(componentSource).not.toContain('max-w-[1600px]')
    expect(componentSource).not.toContain('mx-auto w-full max-w')
  })
})
