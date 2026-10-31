import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { useUserSupportAccess } from '../useUserSupportAccess'

const { getAccess } = vi.hoisted(() => ({ getAccess: vi.fn() }))
vi.mock('@/api/admin/userSupport', () => ({ getAccess }))

describe('user support access isolation', () => {
  beforeEach(() => { vi.clearAllMocks(); localStorage.clear() })

  it('fails closed for logged-out users without requesting admin APIs', async () => {
    const auth = reactive({ user: { id: 4, role: 'user' }, isAdmin: false, isSubAdmin: false, isAuthenticated: false })
    const access = useUserSupportAccess(auth)
    expect(await access.refresh()).toBe(false)
    expect(access.canAccess.value).toBe(false)
    expect(getAccess).not.toHaveBeenCalled()
  })

  it('allows an ordinary user only after explicit support authorization', async () => {
    const auth = reactive({ user: { id: 104, role: 'user' }, isAdmin: false, isSubAdmin: false })
    const access = useUserSupportAccess(auth)
    getAccess.mockResolvedValue({ enabled: true })
    expect(access.canAccess.value).toBe(false)
    expect(await access.refresh()).toBe(true)
    expect(access.canAccess.value).toBe(true)
    expect(auth.isAdmin).toBe(false)
    expect(auth.isSubAdmin).toBe(false)
  })

  it('requires explicit sub-admin permission and refreshes revocations', async () => {
    const auth = reactive({ user: { id: 5, role: 'sub_admin' }, isAdmin: false, isSubAdmin: true })
    const access = useUserSupportAccess(auth)
    getAccess.mockResolvedValueOnce({ enabled: true }).mockResolvedValueOnce({ enabled: false })
    expect(await access.refresh()).toBe(true)
    expect(access.canAccess.value).toBe(true)
    expect(await access.refresh()).toBe(false)
    expect(access.canAccess.value).toBe(false)
  })

  it('cannot transfer an in-flight permission to another login', async () => {
    const auth = reactive({ user: { id: 6, role: 'sub_admin' }, isAdmin: false, isSubAdmin: true, token: 'first-session' })
    let complete!: (result: { enabled: boolean }) => void
    getAccess.mockReturnValueOnce(new Promise(resolve => { complete = resolve }))
    const access = useUserSupportAccess(auth)
    const loading = access.refresh()
    auth.user.id = 7
    auth.token = 'next-session'
    complete({ enabled: true })
    expect(await loading).toBe(false)
    expect(access.canAccess.value).toBe(false)
  })

  it('clears a revoked current permission immediately', async () => {
    const auth = reactive({ user: { id: 8, role: 'sub_admin' }, isAdmin: false, isSubAdmin: true })
    const access = useUserSupportAccess(auth)
    getAccess.mockResolvedValue({ enabled: true })
    await access.refresh()
    access.revoke()
    expect(access.canAccess.value).toBe(false)
  })
})
