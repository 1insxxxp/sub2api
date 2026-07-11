import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

function collectLeafMessages(
  messages: LocaleMessages,
  prefix = '',
  leaves = new Map<string, unknown>(),
): Map<string, unknown> {
  for (const [key, value] of Object.entries(messages)) {
    const path = prefix ? `${prefix}.${key}` : key

    if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
      collectLeafMessages(value as LocaleMessages, path, leaves)
    } else {
      leaves.set(path, value)
    }
  }

  return leaves
}

const zhMessages = collectLeafMessages(zh)
const enMessages = collectLeafMessages(en)

describe('locale parity', () => {
  it('keeps the complete Chinese and English message trees in sync', () => {
    const zhOnly = [...zhMessages.keys()].filter((key) => !enMessages.has(key))
    const enOnly = [...enMessages.keys()].filter((key) => !zhMessages.has(key))

    expect({ zhOnly, enOnly }).toEqual({ zhOnly: [], enOnly: [] })
  })

  it.each([
    ['zh', zhMessages],
    ['en', enMessages],
  ])('contains only non-empty leaf messages in %s', (_locale, messages) => {
    const invalidMessages = [...messages].filter(
      ([, value]) => typeof value !== 'string' || value.trim() === '',
    )

    expect(invalidMessages).toEqual([])
  })
})
