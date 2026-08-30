import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdminAffiliateLeaderboardPanel from '../AdminAffiliateLeaderboardPanel.vue'

const { getWorkbenchLeaderboard, showError } = vi.hoisted(() => ({
  getWorkbenchLeaderboard: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin/affiliates', () => ({
  affiliatesAPI: { getWorkbenchLeaderboard },
  default: { getWorkbenchLeaderboard }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('AdminAffiliateLeaderboardPanel', () => {
  beforeEach(() => {
    getWorkbenchLeaderboard.mockReset()
    showError.mockReset()
    getWorkbenchLeaderboard.mockResolvedValue({
      items: [
        {
          inviter_id: 42,
          inviter_email: 'leader@example.com',
          inviter_username: 'leader',
          invited_count: 31,
          qualified_invitee_count: 18,
          total_rebate: 12.34,
          last_invited_at: '2026-08-29T09:30:00Z'
        },
        {
          inviter_id: 7,
          inviter_email: 'runner-up@example.com',
          inviter_username: '',
          invited_count: 20,
          qualified_invitee_count: 9,
          total_rebate: 5,
          last_invited_at: null
        }
      ]
    })
  })

  it('renders a read-only top-20 leaderboard for desktop and mobile', async () => {
    const wrapper = mount(AdminAffiliateLeaderboardPanel, {
      global: { stubs: { Icon: true } }
    })
    await flushPromises()

    expect(getWorkbenchLeaderboard).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="affiliate-leaderboard-desktop"]').text()).toContain('leader@example.com')
    expect(wrapper.get('[data-test="affiliate-leaderboard-mobile"]').text()).toContain('runner-up@example.com')
    expect(wrapper.get('[data-test="affiliate-leaderboard-rank-1"]').text()).toBe('1')
    expect(wrapper.text()).toContain('31')
    expect(wrapper.text()).toContain('18')
    expect(wrapper.text()).toContain('$12.34')
    expect(wrapper.findAll('button')).toHaveLength(0)
    expect(wrapper.findAll('a')).toHaveLength(0)
  })

  it('shows a passive empty state when no invite data exists', async () => {
    getWorkbenchLeaderboard.mockResolvedValue({ items: [] })

    const wrapper = mount(AdminAffiliateLeaderboardPanel, {
      global: { stubs: { Icon: true } }
    })
    await flushPromises()

    expect(wrapper.get('[data-test="affiliate-leaderboard-empty"]').text()).toContain(
      'adminWorkbench.affiliateLeaderboard.empty'
    )
    expect(wrapper.findAll('button')).toHaveLength(0)
  })
})
