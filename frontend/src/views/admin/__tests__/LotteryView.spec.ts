import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LotteryView from '../LotteryView.vue'

const { getConfig, listDraws, grantAttempts, listUsers, saveActivity, showSuccess, showError } = vi.hoisted(() => ({
  getConfig: vi.fn(),
  listDraws: vi.fn(),
  grantAttempts: vi.fn(),
  listUsers: vi.fn(),
  saveActivity: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/lottery', () => ({
  lotteryAdminAPI: {
    getConfig,
    listDraws,
    grantAttempts,
    saveActivity,
  },
  default: {
    getConfig,
    saveActivity,
  },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: { list: listUsers },
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
    listDraws.mockReset()
    grantAttempts.mockReset()
    listUsers.mockReset()
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
    listDraws.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
    grantAttempts.mockResolvedValue({ affected: 1, total_granted: 2 })
    listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 8, pages: 0 })
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

  it('loads and paginates draw records with user and reward details', async () => {
    listDraws.mockResolvedValue({
      items: [{
        id: 7,
        user_id: 11,
        user_email: 'winner@example.com',
        user_name: 'Winner',
        user_deleted: false,
        prize_name: '高级兑换码',
        prize_type: 'product',
        product_content: 'code-001',
        attempt_source: 'wallet',
        created_at: '2026-09-03T00:00:00.000Z',
      }],
      total: 11,
      page: 1,
      page_size: 10,
      pages: 2,
    })
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          Pagination: {
            template: '<button data-test="lottery-draw-next" @click="$emit(\'update:page\', 2)">next</button>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-test="lottery-draw-record-7"]').text()).toContain('winner@example.com')
    expect(wrapper.get('[data-test="lottery-draw-record-7"]').text()).toContain('高级兑换码')
    await wrapper.get('[data-test="lottery-draw-next"]').trigger('click')
    await flushPromises()
    expect(listDraws).toHaveBeenLastCalledWith({ page: 2, page_size: 10 })
  })

  it('grants attempts to selected users', async () => {
    listUsers.mockResolvedValue({
      items: [{ id: 11, email: 'alice@example.com', username: 'Alice', status: 'active', role: 'user' }],
      total: 1,
      page: 1,
      page_size: 8,
      pages: 1,
    })
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-test="lottery-grant-user-search"]').setValue('alice')
    await wrapper.get('[data-test="lottery-grant-search"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="lottery-grant-user-result-11"]').trigger('click')
    await wrapper.get('[data-test="lottery-grant-amount"]').setValue('3')
    await wrapper.get('[data-test="lottery-grant-submit"]').trigger('click')
    await flushPromises()

    expect(grantAttempts).toHaveBeenCalledWith(expect.objectContaining({ user_ids: [11], amount: 3, description: '', request_key: expect.any(String) }))
    expect(wrapper.get('[data-test="lottery-grant-result"]').text()).toContain('1')
  })

  it('grants attempts to all users when the all-users target is selected', async () => {
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-test="lottery-grant-all"]').setValue(true)
    await wrapper.get('[data-test="lottery-grant-amount"]').setValue('2')
    await wrapper.get('[data-test="lottery-grant-submit"]').trigger('click')
    await flushPromises()

    expect(grantAttempts).toHaveBeenCalledWith(expect.objectContaining({ all: true, amount: 2, description: '', request_key: expect.any(String) }))
  })
})
