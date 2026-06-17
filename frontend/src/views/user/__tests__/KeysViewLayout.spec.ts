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
})
