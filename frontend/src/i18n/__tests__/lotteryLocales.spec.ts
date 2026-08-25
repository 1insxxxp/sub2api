import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('lottery locale registration', () => {
  it.each([
    ['zh', zh, '幸运抽奖'],
    ['en', en, 'Lottery'],
  ])('registers the %s lottery messages at the public lottery path', (_locale, messages, title) => {
    expect(messages.lottery.title).toBe(title)
    expect(messages.lottery.admin.title).toBeTruthy()
  })
})
