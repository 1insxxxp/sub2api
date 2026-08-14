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
    expect(keysViewSource).toMatch(/class="[^"]*keys-mobile-secondary-action[^"]*whitespace-nowrap[^"]*"\s+data-test="custom-groups-entry"/)
  })

  it('uses compact, visually ranked actions on mobile', () => {
    expect(keysViewSource).toContain('grid-cols-[44px_44px_minmax(0,1fr)_minmax(0,1fr)]')
    expect(keysViewSource).toContain('sm:flex sm:w-auto')
    expect(keysViewSource).toContain('data-test="keys-create-entry"')
    expect(keysViewSource).toMatch(/data-test="keys-create-entry"[^>]*class="[^"]*btn-primary[^"]*whitespace-nowrap/)
    expect(keysViewSource).toContain('max-[359px]:hidden')
  })

  it('uses a bottom sheet for column settings on narrow screens', () => {
    expect(keysViewSource).toContain('isMobileColumnSelector')
    expect(keysViewSource).toContain('data-test="column-selector-backdrop"')
    expect(keysViewSource).toContain('data-test="column-selector-sheet"')
    expect(keysViewSource).toContain('inset-x-2 bottom-2')
    expect(keysViewSource).toContain('@click="closeColumnSelector"')
    expect(keysViewSource).toContain('showColumnDropdown && !isMobileColumnSelector')
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

  it('uses a wider desktop group selector with matching viewport clamping', () => {
    expect(keysViewSource).toContain("'w-[480px] slide-in-from-top-2'")
    expect(keysViewSource).toContain('const dropdownEstWidth = Math.min(480,')
    expect(keysViewSource).not.toContain("'w-[380px] slide-in-from-top-2'")
  })

  it('closes the group selector when the viewport size changes', () => {
    expect(keysViewSource).toContain("window.addEventListener('resize', closeGroupSelector)")
    expect(keysViewSource).toContain("window.removeEventListener('resize', closeGroupSelector)")
  })

  it('shows the bound custom group instead of treating the key as ungrouped', () => {
    expect(keysViewSource).toContain('v-else-if="row.custom_group"')
    expect(keysViewSource).toContain('{{ row.custom_group.name }}')
  })

  it('uses the responsive Select component for custom groups in the key dialog', () => {
    expect(keysViewSource).toContain('data-test="custom-group-selector"')
    expect(keysViewSource).toContain(':options="customGroupOptions"')
    expect(keysViewSource).toContain('@update:modelValue="formData.group_id = null"')
    expect(keysViewSource).not.toContain('<select v-model="formData.custom_group_id"')
  })
})
