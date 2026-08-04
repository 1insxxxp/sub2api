import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../AppSidebar.vue'), 'utf8')

describe('AppSidebar custom group navigation', () => {
  it('keeps custom groups as an API-key page action instead of a sidebar item', () => {
    expect(source).not.toContain("{ path: '/custom-groups'")
  })
})
