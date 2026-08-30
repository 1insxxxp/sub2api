import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { LotteryPrize } from '@/api/lottery'
import LotterySlotMachine from '../LotterySlotMachine.vue'

const prizes: LotteryPrize[] = [
  {
    id: 1,
    activity_id: 9,
    name: '余额 1 元',
    description: '到账奖励',
    type: 'balance',
    weight: 4,
    balance_amount: 1,
    enabled: true,
    sort_order: 1,
    available_item_count: 0,
  },
  {
    id: 2,
    activity_id: 9,
    name: '高级兑换码',
    description: '一次性产品内容',
    type: 'product',
    weight: 1,
    enabled: true,
    balance_amount: null,
    available_item_count: 3,
    sort_order: 2,
  },
]

const mountMachine = (props: Record<string, unknown> = {}) => mount(LotterySlotMachine, {
  props: { prizes, isDrawing: false, ...props },
  global: {
    stubs: {
      Icon: { template: '<span aria-hidden="true" />' },
    },
  },
})

describe('LotterySlotMachine', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('renders three stable reels and repeats a short prize pool across them', () => {
    const wrapper = mountMachine()

    expect(wrapper.find('[data-testid="lottery-slot-machine"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="lottery-slot-reel"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('余额 1 元')
    expect(wrapper.text()).toContain('高级兑换码')
  })

  it('marks the machine busy while drawing and settles on the server winner', async () => {
    const wrapper = mountMachine()

    await wrapper.setProps({ isDrawing: true })
    expect(wrapper.attributes('aria-busy')).toBe('true')
    expect(wrapper.find('[data-slot-state="spinning"]').exists()).toBe(true)

    await wrapper.setProps({ winnerId: 2 })
    await vi.advanceTimersByTimeAsync(2200)

    expect(wrapper.find('[data-testid="lottery-slot-center"]').text()).toContain('高级兑换码')
    expect(wrapper.emitted('settled')).toHaveLength(1)
  })

  it('settles immediately when reduced motion is requested', async () => {
    const wrapper = mountMachine({ reducedMotion: true })

    await wrapper.setProps({ isDrawing: true, winnerId: 2 })
    await vi.runAllTimersAsync()

    expect(wrapper.find('[data-testid="lottery-slot-center"]').text()).toContain('高级兑换码')
    expect(wrapper.emitted('settled')).toHaveLength(1)
  })

  it('returns to idle without announcing a fake result when the request fails', async () => {
    const wrapper = mountMachine()

    await wrapper.setProps({ isDrawing: true })
    await wrapper.setProps({ isDrawing: false })

    expect(wrapper.attributes('aria-busy')).toBe('false')
    expect(wrapper.emitted('settled')).toBeUndefined()
  })
})
