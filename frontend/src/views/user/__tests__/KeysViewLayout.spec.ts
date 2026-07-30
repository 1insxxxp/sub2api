import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const keysViewSource = readFileSync(resolve(currentDir, '../KeysView.vue'), 'utf8')

describe('KeysView toolbar layout', () => {
  it('keeps filters and primary actions in one toolbar surface', () => {
    expect(keysViewSource).not.toContain('<template #actions>')
    expect(keysViewSource).toContain('data-test="keys-toolbar"')
    expect(keysViewSource).toContain('data-test="keys-toolbar-actions"')
  })

  it('keeps the group selector inside the mobile viewport', () => {
    expect(keysViewSource).toContain('w-[calc(100vw-16px)]')
    expect(keysViewSource).toContain('sm:w-[380px]')
    expect(keysViewSource).toContain('const dropdownViewportPadding = 8')
    expect(keysViewSource).toContain(':wrap-name="true"')
    expect(keysViewSource).toContain('data-test="group-selector-trigger"')
    expect(keysViewSource).toContain('max-w-full min-w-0 flex-wrap')
  })

  it('closes the group selector when the viewport size changes', () => {
    expect(keysViewSource).toContain("window.addEventListener('resize', closeGroupSelector)")
    expect(keysViewSource).toContain("window.removeEventListener('resize', closeGroupSelector)")
  })
})
