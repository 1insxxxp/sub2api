import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader available token estimate', () => {
  it('uses the full Token unit instead of the ambiguous TOK abbreviation', () => {
    expect(componentSource).toContain('{{ availableTokensLabel }} Token')
    expect(componentSource).not.toContain('{{ availableTokensLabel }} TOK')
  })
})
