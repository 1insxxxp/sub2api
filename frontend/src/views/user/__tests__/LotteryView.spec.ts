import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import LotteryView from '../LotteryView.vue'

const { getState, history, draw, showError, showSuccess } = vi.hoisted(() => ({
  getState: vi.fn(),
  history: vi.fn(),
  draw: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/lottery', () => ({
  lotteryAPI: { getState, history, draw },
  default: { getState, history, draw },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'en-US' },
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'lottery.attemptBreakdown') return `${params?.activity} / ${params?.reward}`
        return key
      },
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

describe('User LotteryView', () => {
  beforeEach(() => {
    getState.mockReset()
    history.mockReset()
    draw.mockReset()
    getState.mockResolvedValue({
      activity: {
        id: 1,
        name: 'Check-in draw',
        description: '',
        status: 'active',
        attempt_mode: 'daily',
        attempt_limit: 0,
      },
      prizes: [],
      attempts_used: 0,
      activity_attempts_remaining: 0,
      reward_attempts_remaining: 3,
      attempts_remaining: 3,
    })
    history.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
  })

  it('shows activity and check-in reward attempts separately', async () => {
    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          LoadingSpinner: true,
          LotterySlotMachine: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-test="lottery-attempt-breakdown"]').text()).toContain('0')
    expect(wrapper.get('[data-test="lottery-attempt-breakdown"]').text()).toContain('3')
    expect(wrapper.text()).toContain('3')
  })

  it('does not show prize weights in the user-facing prize details', async () => {
    getState.mockResolvedValueOnce({
      activity: {
        id: 1,
        name: 'Check-in draw',
        description: '',
        status: 'active',
        attempt_mode: 'daily',
        attempt_limit: 0,
      },
      prizes: [{
        id: 9,
        activity_id: 1,
        name: 'One dollar',
        description: 'A small reward',
        type: 'balance',
        weight: 9,
        balance_amount: 1,
        enabled: true,
        sort_order: 0,
        available_item_count: 0,
      }],
      attempts_used: 0,
      activity_attempts_remaining: 0,
      reward_attempts_remaining: 3,
      attempts_remaining: 3,
    })

    const wrapper = mount(LotteryView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          LoadingSpinner: true,
          LotterySlotMachine: true,
        },
      },
    })

    await flushPromises()

    const card = wrapper.get('.lottery-prize-card')
    expect(wrapper.get('.lottery-prize-section')).toBeTruthy()
    expect(wrapper.get('.lottery-prize-grid')).toBeTruthy()
    expect(wrapper.get('.lottery-history-section')).toBeTruthy()
    expect(card.text()).toContain('A small reward')
    expect(card.text()).toContain('$1.00')
    expect(card.text()).not.toContain('lottery.weight')
  })
})
