import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminUser } from '@/types'

import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'

const { listGroups, updateUser, showSuccess, showError } = vi.hoisted(() => ({
  listGroups: vi.fn(),
  updateUser: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

enableAutoUnmount(afterEach)
afterEach(() => vi.restoreAllMocks())

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

const response = { items: [{ id: 7, name: 'Exclusive', platform: 'openai', is_exclusive: true, subscription_type: 'standard', status: 'active', rate_multiplier: 1 }] }
async function openDialog() {
  const wrapper = mount(UserAllowedGroupsModal, {
    props: { show: false, user: { id: 1, email: 'user@example.com', allowed_groups: [7], group_rates: { 7: 0.5 } } as AdminUser },
    global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }, PlatformIcon: true } }
  })
  await wrapper.setProps({ show: true })
  return wrapper
}

describe('UserAllowedGroupsModal load readiness', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    listGroups.mockResolvedValue(response)
    updateUser.mockResolvedValue(undefined)
  })

  it('cannot save an empty configuration while groups are loading', async () => {
    let resolve!: (value: typeof response) => void
    listGroups.mockReturnValueOnce(new Promise(res => { resolve = res }))
    const wrapper = await openDialog()
    const save = wrapper.get('button.btn-primary')
    expect(save.attributes('disabled')).toBeDefined()
    await save.trigger('click')
    expect(updateUser).not.toHaveBeenCalled()
    resolve(response)
    await flushPromises()
    expect(save.attributes('disabled')).toBeUndefined()
  })

  it('cannot save after loading fails', async () => {
    listGroups.mockRejectedValueOnce(new Error('Offline'))
    const wrapper = await openDialog()
    await flushPromises()
    const save = wrapper.get('button.btn-primary')
    expect(save.attributes('disabled')).toBeDefined()
    await save.trigger('click')
    expect(updateUser).not.toHaveBeenCalled()
  })

  it('does not reuse a previous successful load after reopening fails', async () => {
    const wrapper = await openDialog()
    await flushPromises()
    await wrapper.setProps({ show: false })
    listGroups.mockRejectedValueOnce(new Error('Offline'))
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeDefined()
    expect(updateUser).not.toHaveBeenCalled()
  })

  it('preserves existing grants and rates on a successful save', async () => {
    const wrapper = await openDialog()
    await flushPromises()
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(updateUser).toHaveBeenCalledWith(1, { allowed_groups: [7], restrict_public_groups: false, group_rates: { 7: 0.5 } })
    expect(wrapper.emitted('success')).toHaveLength(1)
  })
})
