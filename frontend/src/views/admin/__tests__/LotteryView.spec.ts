import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LotteryView from '../LotteryView.vue'

const { getConfig, saveActivity, showSuccess, showError } = vi.hoisted(() => ({
  getConfig: vi.fn(),
  saveActivity: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/lottery', () => ({
  lotteryAdminAPI: {
    getConfig,
    saveActivity,
  },
  default: {
    getConfig,
    saveActivity,
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError, showWarning: vi.fn() }),
}))

describe('Admin LotteryView', () => {
  beforeEach(() => {
    getConfig.mockReset()
    saveActivity.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    getConfig.mockResolvedValue({
      activity: {
        id: 1,
        name: 'Check-in draw',
        description: '',
        status: 'active',
        attempt_mode: 'daily',
        attempt_limit: 0,
      },
      prizes: [],
    })
    saveActivity.mockImplementation(async request => ({ id: 1, ...request }))
  })

  it('loads and saves a zero free-attempt limit', async () => {
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const input = wrapper.get('[data-test="lottery-attempt-limit"]')
    expect((input.element as HTMLInputElement).value).toBe('0')
    await wrapper.get('[data-test="save-lottery-activity"]').trigger('click')
    await flushPromises()

    expect(saveActivity).toHaveBeenCalledWith(expect.objectContaining({
      attempt_limit: 0,
    }))
  })
})
