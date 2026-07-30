import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import UserApiKeysModal from '../UserApiKeysModal.vue'

const { getUserApiKeys, getAllGroups } = vi.hoisted(() => ({
  getUserApiKeys: vi.fn(),
  getAllGroups: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { getUserApiKeys },
    groups: { getAll: getAllGroups },
    apiKeys: { updateApiKeyGroup: vi.fn() },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../UserApiKeysModal.vue'), 'utf8')

const apiKey = {
  id: 1,
  name: 'mobile-key',
  key: 'sk-test-mobile-key-1234567890',
  status: 'active',
  group_id: null,
  group: null,
  created_at: '2026-07-31T00:00:00Z',
}

const mountModal = async (viewportWidth: number) => {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: viewportWidth })
  getUserApiKeys.mockResolvedValue({ items: [apiKey] })
  getAllGroups.mockResolvedValue([])

  const wrapper = mount(UserApiKeysModal, {
    props: {
      show: false,
      user: { id: 1, email: 'mobile@example.com', username: 'mobile' } as any,
    },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /></div>' },
        GroupBadge: true,
        GroupOptionItem: true,
        Teleport: { template: '<div><slot /></div>' },
      },
    },
  })

  await wrapper.setProps({ show: true })
  await flushPromises()
  await nextTick()
  return wrapper
}

describe('UserApiKeysModal mobile group selector', () => {
  beforeEach(() => {
    getUserApiKeys.mockReset()
    getAllGroups.mockReset()
  })

  it('wraps selected group names and constrains the dropdown to the viewport', () => {
    expect(source).toContain(':wrap-name="true"')
    expect(source).toContain('w-[calc(100vw-16px)]')
    expect(source).toContain('sm:w-64')
    expect(source).toContain('const DROPDOWN_VIEWPORT_PADDING = 8')
    expect(source).toContain('const dropdownWidth = window.innerWidth < 640')
    expect(source).toContain('? window.innerWidth - DROPDOWN_VIEWPORT_PADDING * 2')
    expect(source).toContain('window.innerWidth - dropdownWidth - DROPDOWN_VIEWPORT_PADDING')
    expect(source).toContain('data-test="group-selector-trigger"')
    expect(source).toContain('max-w-full min-w-0 flex-wrap')
  })

  it('closes the group selector when the viewport size changes', () => {
    expect(source).toContain("window.addEventListener('resize', closeGroupSelector)")
    expect(source).toContain("window.removeEventListener('resize', closeGroupSelector)")
  })

  it.each([320, 390])('keeps a right-edge dropdown within a %dpx viewport', async (width) => {
    const wrapper = await mountModal(width)
    const trigger = wrapper.get('[data-test="group-selector-trigger"]')
    trigger.element.getBoundingClientRect = () => ({
      top: 80,
      right: width,
      bottom: 104,
      left: width - 24,
      width: 24,
      height: 24,
      x: width - 24,
      y: 80,
      toJSON: () => ({}),
    })

    await trigger.trigger('click')

    expect(wrapper.get('.admin-action-menu').attributes('style')).toContain('left: 8px')
    wrapper.unmount()
  })

  it('closes an open dropdown after a resize event', async () => {
    const wrapper = await mountModal(390)
    await wrapper.get('[data-test="group-selector-trigger"]').trigger('click')
    expect(wrapper.find('.admin-action-menu').exists()).toBe(true)

    window.dispatchEvent(new Event('resize'))
    await nextTick()

    expect(wrapper.find('.admin-action-menu').exists()).toBe(false)
    wrapper.unmount()
  })
})
