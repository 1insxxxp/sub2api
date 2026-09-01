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
  username: 'operator',
  email: 'operator@example.com',
  role: 'sub_admin',
  balance: 25,
  concurrency: 2,
  status: 'active',
  allowed_groups: null,
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
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

describe('admin user sub-admin role controls', () => {
  beforeEach(() => {
    createUser.mockReset()
    updateUser.mockReset()
    updateUserAttributeValues.mockReset()
    showSuccess.mockReset()
    createUser.mockResolvedValue(baseUser())
    updateUser.mockResolvedValue(baseUser())
  })

  it('submits the sub-admin role while creating a user', async () => {
    const wrapper = mount(UserCreateModal, {
      props: { show: true },
      global: { stubs: globalStubs }
    })

    await wrapper.get('input[type="email"]').setValue('operator@example.com')
    await wrapper.get('input[required][type="text"]').setValue('secret123')
    await wrapper.get('select').setValue('sub_admin')
    await wrapper.get('#create-user-form').trigger('submit')
    await flushPromises()

    expect(createUser).toHaveBeenCalledWith(
      expect.objectContaining({
        email: 'operator@example.com',
        password: 'secret123',
        role: 'sub_admin'
      })
    )
  })

  it('shows and submits the sub-admin role while editing a user', async () => {
    const wrapper = mount(UserEditModal, {
      props: { show: true, user: baseUser({ role: 'sub_admin' }) },
      global: { stubs: globalStubs }
    })

    expect(wrapper.text()).toContain('admin.users.roles.sub_admin')

    await wrapper.get('#edit-user-form').trigger('submit')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(
      7,
      expect.objectContaining({
        role: 'sub_admin'
      })
    )
  })
})
