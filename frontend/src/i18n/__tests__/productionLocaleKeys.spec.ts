import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

const sourceRoot = 'src'
const ignoredDirectories = new Set(['__tests__', 'i18n'])
const literalTranslationPattern = /(?<![\w$])(?:\$t|t)\s*\(\s*(['"])([^'"]+)\1/g
const criticalDynamicKeys = [
  'auth.errors.AUTH_IP_BLOCKED',
  'admin.redeem.types.checkin_reward',
  'redeem.checkinReward',
]

function collectProductionSourceFiles(directory: string): string[] {
  const files: string[] = []

  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name)

    if (entry.isDirectory()) {
      if (!ignoredDirectories.has(entry.name)) {
        files.push(...collectProductionSourceFiles(path))
      }
      continue
    }

    if (/\.(?:ts|vue)$/.test(entry.name) && !/\.(?:spec|test)\.ts$/.test(entry.name)) {
      files.push(path)
    }
  }

  return files
}

function collectLiteralTranslationKeys(): string[] {
  const keys = new Set(criticalDynamicKeys)

  for (const file of collectProductionSourceFiles(sourceRoot)) {
    const source = readFileSync(file, 'utf8')

    for (const match of source.matchAll(literalTranslationPattern)) {
      const key = match[2]
      if (!key.endsWith('.')) {
        keys.add(key)
      }
    }
  }

  return [...keys].sort()
}

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

const productionTranslationKeys = collectLiteralTranslationKeys()

describe.each([
  ['zh', zh],
  ['en', en],
])('production %s locale', (_locale, messages) => {
  it('provides every translation referenced by production code', () => {
    expect(productionTranslationKeys.length).toBeGreaterThan(5_000)

    const missingOrEmptyKeys = productionTranslationKeys.filter((key) => {
      const message = getMessage(messages, key)
      return typeof message !== 'string' || message.trim() === ''
    })

    expect(missingOrEmptyKeys).toEqual([])
  })
})
