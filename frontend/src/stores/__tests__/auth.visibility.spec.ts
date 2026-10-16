import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

const mockLogin = vi.fn()
const mockLogout = vi.fn()
const mockGetCurrentUser = vi.fn()
const mockRefreshToken = vi.fn()

vi.mock('@/api', () => ({
  authAPI: {
    login: (...args: any[]) => mockLogin(...args),
    logout: (...args: any[]) => mockLogout(...args),
    getCurrentUser: (...args: any[]) => mockGetCurrentUser(...args),
    refreshToken: (...args: any[]) => mockRefreshToken(...args)
  },
  isTotp2FARequired: (response: any) => response?.requires_2fa === true
}))

const fakeUser = {
  id: 1,
  username: 'testuser',
  email: 'test@example.com',
  role: 'user' as const,
  balance: 100,
  concurrency: 5,
  status: 'active' as const,
  allowed_groups: null,
  created_at: '2024-01-01',
  updated_at: '2024-01-01'
}

const fakeAuthResponse = {
  access_token: 'test-token-123',
  refresh_token: 'refresh-token-456',
  expires_in: 3600,
  token_type: 'Bearer',
  user: fakeUser
}

function setDocumentHidden(hidden: boolean): void {
  Object.defineProperty(document, 'hidden', {
    configurable: true,
    value: hidden
  })
}

describe('useAuthStore visibility-aware polling', () => {
  const originalHiddenDescriptor = Object.getOwnPropertyDescriptor(document, 'hidden')

  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    setDocumentHidden(false)
    vi.useFakeTimers()
    vi.clearAllMocks()
    mockLogin.mockResolvedValue(fakeAuthResponse)
    mockLogout.mockResolvedValue(undefined)
    mockGetCurrentUser.mockResolvedValue({ data: fakeUser })
  })

  afterEach(() => {
    vi.useRealTimers()
    if (originalHiddenDescriptor) {
      Object.defineProperty(document, 'hidden', originalHiddenDescriptor)
    }
  })

  it('skips the user refresh interval while the document is hidden', async () => {
    const store = useAuthStore()
    await store.login({ email: fakeUser.email, password: 'secret' })
    setDocumentHidden(true)

    vi.advanceTimersByTime(60_000)

    expect(mockGetCurrentUser).not.toHaveBeenCalled()
    await store.logout()
  })

  it('refreshes user data once when returning to a visible document', async () => {
    const store = useAuthStore()
    await store.login({ email: fakeUser.email, password: 'secret' })
    setDocumentHidden(true)
    vi.advanceTimersByTime(60_000)

    setDocumentHidden(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await Promise.resolve()

    expect(mockGetCurrentUser).toHaveBeenCalledTimes(1)
    await store.logout()
  })

  it('keeps refreshing on the existing interval while visible', async () => {
    const store = useAuthStore()
    await store.login({ email: fakeUser.email, password: 'secret' })

    vi.advanceTimersByTime(60_000)
    await Promise.resolve()

    expect(mockGetCurrentUser).toHaveBeenCalledTimes(1)
    await store.logout()
  })
})
