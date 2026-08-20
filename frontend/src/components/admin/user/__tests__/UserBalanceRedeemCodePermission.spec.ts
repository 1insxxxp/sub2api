import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UserCreateModal from '../UserCreateModal.vue'
import UserEditModal from '../UserEditModal.vue'

const { createUser, updateUser, updateUserAttributeValues, showSuccess } = vi.hoisted(() => ({
  createUser: vi.fn(),
  updateUser: vi.fn(),
  updateUserAttributeValues: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      create: createUser,
      update: updateUser
    },
    userAttributes: {
      updateUserAttributeValues
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess
  })
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({
    run: (operation: () => Promise<unknown>) => operation()
  }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => ''
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const dialogStub = {
  props: ['show', 'title', 'width'],
  emits: ['close'],
  template: '<div v-if="show"><slot /><footer><slot name="footer" /></footer></div>'
}

const baseUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 7,
  username: 'transfer',
  email: 'transfer@example.com',
  role: 'user',
  balance: 25,
  concurrency: 2,
  status: 'active',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  balance_redeem_code_enabled: true,
  created_at: '2026-08-20T12:00:00Z',
  updated_at: '2026-08-20T12:00:00Z',
  notes: '',
  ...overrides
})

const globalStubs = {
  BaseDialog: dialogStub,
  Icon: true,
  TotpStepUpDialog: true,
  UserAttributeForm: true
}

describe('admin user balance redeem code permission', () => {
  beforeEach(() => {
    createUser.mockReset()
    updateUser.mockReset()
    updateUserAttributeValues.mockReset()
    showSuccess.mockReset()
    createUser.mockResolvedValue(baseUser())
    updateUser.mockResolvedValue(baseUser())
  })

  it('submits the enabled permission while creating a user', async () => {
    const wrapper = mount(UserCreateModal, {
      props: { show: true },
      global: { stubs: globalStubs }
    })

    await wrapper.get('input[type="email"]').setValue('transfer@example.com')
    await wrapper.get('input[required][type="text"]').setValue('secret123')
    await wrapper.get('[data-test="balance-redeem-code-toggle"]').setValue(true)
    await wrapper.get('#create-user-form').trigger('submit')
    await flushPromises()

    expect(createUser).toHaveBeenCalledWith(
      expect.objectContaining({
        email: 'transfer@example.com',
        password: 'secret123',
        balance_redeem_code_enabled: true
      })
    )
  })

  it('submits permission changes while editing a user', async () => {
    const wrapper = mount(UserEditModal, {
      props: { show: true, user: baseUser({ balance_redeem_code_enabled: true }) },
      global: { stubs: globalStubs }
    })

    const toggle = wrapper.get<HTMLInputElement>('[data-test="balance-redeem-code-toggle"]')
    expect(toggle.element.checked).toBe(true)

    await toggle.setValue(false)
    await wrapper.get('#edit-user-form').trigger('submit')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(
      7,
      expect.objectContaining({
        balance_redeem_code_enabled: false
      })
    )
  })
})
