import { beforeEach, describe, expect, it, vi } from 'vitest'

const { del } = vi.hoisted(() => ({
  del: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    delete: del
  }
}))

import { deleteOwnAccount } from '@/api/user'

describe('user api account deletion', () => {
  beforeEach(() => {
    del.mockReset()
  })

  it('sends the current password in the DELETE request body', async () => {
    del.mockResolvedValue({ data: { message: 'Account deleted successfully' } })

    const result = await deleteOwnAccount('current-password')

    expect(del).toHaveBeenCalledWith('/user/account', {
      data: { password: 'current-password' }
    })
    expect(result).toEqual({ message: 'Account deleted successfully' })
  })
})
