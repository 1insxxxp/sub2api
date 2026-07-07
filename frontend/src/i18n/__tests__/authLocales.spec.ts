import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('auth locale error copy', () => {
  it('shows a clear message when auth IP access is blocked', () => {
    expect(zh.auth.errors.AUTH_IP_BLOCKED).toBe('当前 IP 被限制登录，请联系管理员处理。')
    expect(en.auth.errors.AUTH_IP_BLOCKED).toBe('Login from your current IP is blocked. Please contact the administrator.')
  })
})
