import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UserDashboardCheckinCard from '../UserDashboardCheckinCard.vue'

const { getCheckinStatusMock, checkinMock } = vi.hoisted(() => ({
  getCheckinStatusMock: vi.fn(),
  checkinMock: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  getCheckinStatus: (...args: unknown[]) => getCheckinStatusMock(...args),
  checkin: (...args: unknown[]) => checkinMock(...args),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'dashboard.checkin.streak') return `streak ${params?.days}`
        if (key === 'dashboard.checkin.lifetime') return `lifetime ${params?.days}`
        if (key === 'dashboard.checkin.reward') return `reward ${params?.amount}`
        return key
      },
    }),
  }
})

describe('UserDashboardCheckinCard', () => {
  beforeEach(() => {
    getCheckinStatusMock.mockReset()
    checkinMock.mockReset()
  })

  it('loads check-in status and renders the action card when enabled', async () => {
    getCheckinStatusMock.mockResolvedValue({
      enabled: true,
      checked_in_today: false,
      current_streak: 6,
      lifetime_checkin_days: 12,
    })

    const wrapper = mount(UserDashboardCheckinCard, {
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(getCheckinStatusMock).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="checkin-card"]').text()).toContain('dashboard.checkin.title')
    expect(wrapper.get('[data-testid="checkin-submit"]').text()).toContain('dashboard.checkin.submit')
    expect(wrapper.text()).toContain('streak 6')
    expect(wrapper.text()).toContain('lifetime 12')
  })

  it('checks in, emits the result, and shows the reward', async () => {
    getCheckinStatusMock.mockResolvedValue({
      enabled: true,
      checked_in_today: false,
      current_streak: 6,
      lifetime_checkin_days: 12,
    })
    checkinMock.mockResolvedValue({
      enabled: true,
      checked_in_today: true,
      current_streak: 7,
      lifetime_checkin_days: 13,
      total_reward_amount: 11,
      balance_after: 23.5,
    })

    const wrapper = mount(UserDashboardCheckinCard, {
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="checkin-submit"]').trigger('click')
    await flushPromises()

    expect(checkinMock).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('checked-in')?.[0]?.[0]).toMatchObject({
      checked_in_today: true,
      total_reward_amount: 11,
    })
    expect(wrapper.get('[data-testid="checkin-submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('reward 11.00')
  })

  it('does not render when check-in is disabled', async () => {
    getCheckinStatusMock.mockResolvedValue({
      enabled: false,
      checked_in_today: false,
    })

    const wrapper = mount(UserDashboardCheckinCard, {
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="checkin-card"]').exists()).toBe(false)
  })
})
