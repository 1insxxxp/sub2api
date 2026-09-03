import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LotteryView from '../LotteryView.vue'

const { getConfig, listDraws, listAttemptBalances, grantAttempts, previewAttemptGrant, listUsers, saveActivity, createPrize, showSuccess, showError } = vi.hoisted(() => ({
  getConfig: vi.fn(),
  listDraws: vi.fn(),
  listAttemptBalances: vi.fn(),
  grantAttempts: vi.fn(),
  previewAttemptGrant: vi.fn(),
  listUsers: vi.fn(),
  saveActivity: vi.fn(),
  createPrize: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/lottery', () => ({
  lotteryAdminAPI: {
    getConfig,
    listDraws,
    listAttemptBalances,
    grantAttempts,
    previewAttemptGrant,
    saveActivity,
    createPrize,
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
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key} ${Object.values(params).join(' ')}` : key }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError, showWarning: vi.fn() }),
}))

describe('Admin LotteryView', () => {
  beforeEach(() => {
    getConfig.mockReset()
    listDraws.mockReset()
    listAttemptBalances.mockReset()
    grantAttempts.mockReset()
    previewAttemptGrant.mockReset()
    listUsers.mockReset()
    saveActivity.mockReset()
    createPrize.mockReset()
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
    listAttemptBalances.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
    grantAttempts.mockResolvedValue({ affected: 1, total_granted: 2 })
    previewAttemptGrant.mockResolvedValue({ count: 0 })
    listUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 8, pages: 0 })
    createPrize.mockResolvedValue({ id: 9, activity_id: 1, name: 'New prize', description: '', type: 'balance', weight: 1, balance_amount: 1, enabled: true, sort_order: 0, available_item_count: 0 })
  })

  it('does not expose the legacy activity attempt policy controls', async () => {
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-test="lottery-attempt-limit"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('lottery.admin.attemptMode')
    expect(wrapper.text()).not.toContain('lottery.admin.attemptLimit')
    await wrapper.get('[data-test="save-lottery-activity"]').trigger('click')
    await flushPromises()

    expect(saveActivity).toHaveBeenCalledWith(expect.not.objectContaining({ attempt_mode: expect.anything(), attempt_limit: expect.anything() }))
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

  it('loads and searches current attempt balances by user', async () => {
    listAttemptBalances.mockResolvedValue({
      items: [{
        user_id: 11,
        user_email: 'alice@example.com',
        user_name: 'Alice',
        user_status: 'active',
        reward_remaining: 3,
        total_remaining: 3,
      }],
      total: 1,
      page: 1,
      page_size: 10,
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

    const row = wrapper.get('[data-test="lottery-attempt-balance-row-11"]')
    expect(row.text()).toContain('alice@example.com')
    expect(row.text()).toContain('3')
    await wrapper.get('[data-test="lottery-attempt-balance-search"]').setValue('alice')
    await wrapper.get('[data-test="lottery-attempt-balance-search-submit"]').trigger('click')
    await flushPromises()
    expect(listAttemptBalances).toHaveBeenLastCalledWith({ page: 1, page_size: 10, search: 'alice' })
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
    expect(listAttemptBalances).toHaveBeenCalledTimes(2)
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

  it('previews and grants attempts to recently active users', async () => {
    previewAttemptGrant.mockResolvedValue({ count: 12 })
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-test="lottery-grant-active"]').setValue(true)
    await flushPromises()

    expect(previewAttemptGrant).toHaveBeenLastCalledWith({ target: 'active', active_days: 7 })
    expect(wrapper.get('[data-test="lottery-grant-active-days"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="lottery-grant-preview"]').text()).toContain('12')

    await wrapper.get('[data-test="lottery-grant-active-days"]').setValue('30')
    await flushPromises()
    expect(previewAttemptGrant).toHaveBeenLastCalledWith({ target: 'active', active_days: 30 })

    await wrapper.get('[data-test="lottery-grant-amount"]').setValue('2')
    await wrapper.get('[data-test="lottery-grant-submit"]').trigger('click')
    await flushPromises()
    expect(grantAttempts).toHaveBeenCalledWith(expect.objectContaining({ target: 'active', active_days: 30, amount: 2, request_key: expect.any(String) }))
  })

  it('blocks active-user grants while the preview request fails', async () => {
    previewAttemptGrant.mockRejectedValue(new Error('preview unavailable'))
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-test="lottery-grant-active"]').setValue(true)
    await flushPromises()

    expect(wrapper.get('[data-test="lottery-grant-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="lottery-grant-submit"]').trigger('click')
    expect(grantAttempts).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalled()
  })

  it('does not submit a balance prize with an empty credit amount', async () => {
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    const addButton = wrapper.findAll('button').find(button => button.text().includes('lottery.admin.addPrize'))
    expect(addButton).toBeDefined()
    await addButton!.trigger('click')
    await wrapper.get('[data-test="lottery-prize-name"]').setValue('余额奖品')
    await wrapper.get('[data-test="lottery-prize-balance-amount"]').setValue('')
    await wrapper.get('[data-test="lottery-save-prize"]').trigger('click')
    await flushPromises()

    expect(createPrize).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('lottery.admin.prizeBalanceAmountInvalid')
  })
})
