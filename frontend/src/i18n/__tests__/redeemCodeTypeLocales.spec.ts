import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('redeem code type locale keys', () => {
  it('includes the check-in reward type in admin redeem translations', () => {
    expect(zh.admin.redeem.types.checkin_reward).toBe('签到奖励')
    expect(en.admin.redeem.types.checkin_reward).toBe('Check-in Reward')
  })
})
