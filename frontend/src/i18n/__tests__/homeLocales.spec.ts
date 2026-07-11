import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

const homeViewSource = readFileSync('src/views/HomeView.vue', 'utf8')
const homeTranslationKeys = [
  ...new Set(
    [...homeViewSource.matchAll(/\bt\(\s*['"`](home\.[^'"`]+)['"`]\s*\)/g)].map(
      (match) => match[1]
    )
  ),
].sort()

function getMessage(messages: LocaleMessages, key: string): unknown {
  let value: unknown = messages

  for (const segment of key.split('.')) {
    if (typeof value !== 'object' || value === null) {
      return undefined
    }
    value = (value as LocaleMessages)[segment]
  }

  return value
}

describe.each([
  ['zh', zh],
  ['en', en],
])('HomeView %s locale', (_locale, messages) => {
  it('provides every translation referenced by the default homepage', () => {
    expect(homeTranslationKeys.length).toBeGreaterThan(0)

    const missingOrEmptyKeys = homeTranslationKeys.filter((key) => {
      const message = getMessage(messages, key)
      return typeof message !== 'string' || message.trim() === ''
    })

    expect(missingOrEmptyKeys).toEqual([])
  })
})
