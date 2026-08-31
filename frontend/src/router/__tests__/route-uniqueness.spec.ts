import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'),
  'utf8',
)

function duplicateValues(values: string[]): string[] {
  return values.filter((value, index) => values.indexOf(value) !== index)
}

describe('route declarations', () => {
  it('keeps top-level route paths and names unique', () => {
    const paths = [...routerSource.matchAll(/^ {4}path: '([^']+)'/gm)].map((match) => match[1])
    const names = [...routerSource.matchAll(/^ {4}name: '([^']+)'/gm)].map((match) => match[1])

    expect(duplicateValues(paths)).toEqual([])
    expect(duplicateValues(names)).toEqual([])
  })
})
