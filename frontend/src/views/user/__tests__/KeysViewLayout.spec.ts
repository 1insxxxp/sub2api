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
    expect(keysViewSource).toContain('data-test="custom-groups-entry"')
    expect(keysViewSource).not.toContain('to="/custom-groups"')
    expect(keysViewSource).toContain('@click="showCustomGroupsModal = true"')
    expect(keysViewSource).toContain('data-test="custom-groups-dialog"')
    expect(keysViewSource).toContain('width="full"')
    expect(keysViewSource).toMatch(/class="btn btn-primary"\s+data-test="custom-groups-entry"/)
  })

  it('uses a bottom sheet for the group selector on narrow screens', () => {
    expect(keysViewSource).toContain('isMobileGroupSelector')
    expect(keysViewSource).toContain('window.innerWidth < 768')
    expect(keysViewSource).toContain('data-test="group-selector-backdrop"')
    expect(keysViewSource).toContain('inset-x-2 bottom-2')
    expect(keysViewSource).toContain("isMobileGroupSelector ? undefined")
    expect(keysViewSource).toContain('const dropdownViewportPadding = 8')
    expect(keysViewSource).toContain(':wrap-name="true"')
    expect(keysViewSource).toContain('data-test="group-selector-trigger"')
    expect(keysViewSource).toContain('max-w-full min-w-0 flex-wrap')
  })

  it('closes the group selector when the viewport size changes', () => {
    expect(keysViewSource).toContain("window.addEventListener('resize', closeGroupSelector)")
    expect(keysViewSource).toContain("window.removeEventListener('resize', closeGroupSelector)")
  })

  it('shows the bound custom group instead of treating the key as ungrouped', () => {
    expect(keysViewSource).toContain('v-else-if="row.custom_group"')
    expect(keysViewSource).toContain('{{ row.custom_group.name }}')
  })
})
