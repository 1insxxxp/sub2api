import { describe, expect, it } from 'vitest'

import { resolveCompletedSetupRedirectPath } from '@/router/setupRedirect'

describe('resolveCompletedSetupRedirectPath', () => {
  it('redirects unauthenticated users to login', () => {
    expect(resolveCompletedSetupRedirectPath(false, false, false)).toBe('/login')
  })

  it('keeps full admins on the full admin dashboard', () => {
    expect(resolveCompletedSetupRedirectPath(true, true, true)).toBe('/admin/dashboard')
  })

  it('sends sub admins to the admin workbench', () => {
    expect(resolveCompletedSetupRedirectPath(true, false, true)).toBe('/admin/workbench')
  })

  it('sends regular users to the user dashboard', () => {
    expect(resolveCompletedSetupRedirectPath(true, false, false)).toBe('/dashboard')
  })
})
