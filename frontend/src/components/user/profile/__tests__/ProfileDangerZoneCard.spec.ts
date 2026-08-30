import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileDangerZoneCard from '@/components/user/profile/ProfileDangerZoneCard.vue'

const {
  deleteOwnAccountMock,
  logoutMock,
  pushMock,
  showErrorMock,
  showSuccessMock
} = vi.hoisted(() => ({
  deleteOwnAccountMock: vi.fn(),
  logoutMock: vi.fn(),
  pushMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn()
}))

vi.mock('@/api', () => ({
  userAPI: {
    deleteOwnAccount: deleteOwnAccountMock
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    logout: logoutMock
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('ProfileDangerZoneCard', () => {
  beforeEach(() => {
    deleteOwnAccountMock.mockReset()
    logoutMock.mockReset()
    pushMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('does not submit without a password', async () => {
    const wrapper = mount(ProfileDangerZoneCard, {
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('[data-testid="account-delete-open"]').trigger('click')
    await wrapper.get('[data-testid="account-delete-form"]').trigger('submit.prevent')

    expect(deleteOwnAccountMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('profile.accountDeletion.passwordRequired')
  })

  it('deletes the account, logs out locally, and redirects to login', async () => {
    deleteOwnAccountMock.mockResolvedValue({ message: 'ok' })
    logoutMock.mockResolvedValue(undefined)
    const wrapper = mount(ProfileDangerZoneCard, {
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('[data-testid="account-delete-open"]').trigger('click')
    await wrapper.get('[data-testid="account-delete-password"]').setValue('current-password')
    await wrapper.get('[data-testid="account-delete-form"]').trigger('submit.prevent')
    await flushPromises()

    expect(deleteOwnAccountMock).toHaveBeenCalledWith('current-password')
    expect(logoutMock).toHaveBeenCalled()
    expect(pushMock).toHaveBeenCalledWith('/login')
    expect(showSuccessMock).toHaveBeenCalledWith('profile.accountDeletion.success')
  })

  it('shows backend error messages on failure', async () => {
    deleteOwnAccountMock.mockRejectedValue({ message: 'current password is incorrect' })
    const wrapper = mount(ProfileDangerZoneCard, {
      global: {
        stubs: { Icon: true }
      }
    })

    await wrapper.get('[data-testid="account-delete-open"]').trigger('click')
    await wrapper.get('[data-testid="account-delete-password"]').setValue('bad-password')
    await wrapper.get('[data-testid="account-delete-form"]').trigger('submit.prevent')
    await flushPromises()

    expect(logoutMock).not.toHaveBeenCalled()
    expect(pushMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('current password is incorrect')
  })
})
