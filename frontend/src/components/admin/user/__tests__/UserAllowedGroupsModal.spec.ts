import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'

const { listGroups, updateUser, showSuccess, showError } = vi.hoisted(() => ({
  listGroups: vi.fn(),
  updateUser: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
    },
    users: {
      update: updateUser,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

function mountModal() {
  return mount(UserAllowedGroupsModal, {
    props: {
      show: false,
      user: {
        id: 42,
        email: 'tester@example.com',
        allowed_groups: [1, 2],
        restrict_public_groups: true,
        group_rates: {},
      } as never,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        PlatformIcon: true,
      },
    },
  })
}

describe('UserAllowedGroupsModal checkbox sizing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listGroups.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'Exclusive Alpha',
          platform: 'claude',
          is_exclusive: true,
          rate_multiplier: 1.2,
          subscription_type: 'standard',
          status: 'active',
        },
        {
          id: 2,
          name: 'Public Beta',
          platform: 'gemini',
          is_exclusive: false,
          rate_multiplier: 1,
          subscription_type: 'standard',
          status: 'active',
        },
      ],
    })
  })

  it('checkboxes use the same 20px custom size', async () => {
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    const checkboxInputs = wrapper.findAll('input[type="checkbox"]')
    expect(checkboxInputs.length).toBeGreaterThan(0)
    expect(checkboxInputs.every((input) => input.classes().includes('sr-only'))).toBe(true)

    const checkboxVisuals = wrapper.findAll('.h-5.w-5')
    expect(checkboxVisuals).toHaveLength(3)
    expect(wrapper.findAll('input.h-4.w-4').length).toBe(0)
  })
})
