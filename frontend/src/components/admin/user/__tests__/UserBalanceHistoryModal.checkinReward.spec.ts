import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const { getUserBalanceHistory } = vi.hoisted(() => ({
  getUserBalanceHistory: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserBalanceHistory
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.users.balanceHistoryTitle': 'Balance History',
    'admin.users.createdAt': 'Created At',
    'admin.users.currentBalance': 'Current Balance',
    'admin.users.totalRecharged': 'Total Recharged',
    'admin.users.allTypes': 'All Types',
    'admin.users.typeBalance': 'Balance',
    'admin.users.typeAffiliateBalance': 'Affiliate Balance',
    'admin.users.typeAdminBalance': 'Admin Balance',
    'admin.users.typeConcurrency': 'Concurrency',
    'admin.users.typeAdminConcurrency': 'Admin Concurrency',
    'admin.users.typeSubscription': 'Subscription',
    'admin.users.typeCheckinReward': 'Check-in Reward',
    'admin.users.typeEmptyResponse': 'Empty Response Compensation',
    'admin.users.emptyResponseUsageLog': 'Usage record #{id}',
    'admin.users.deposit': 'Deposit',
    'admin.users.withdraw': 'Withdraw',
    'redeem.balanceAddedRedeem': 'Balance Added (Redeem)',
    'redeem.balanceAddedAffiliate': 'Balance Added (Affiliate Transfer)',
    'redeem.balanceAddedAdmin': 'Balance Added (Admin)',
    'redeem.balanceDeductedAdmin': 'Balance Deducted (Admin)',
    'redeem.checkinReward': 'Daily Check-in Reward',
    'redeem.emptyResponseRefund': 'Empty Response Compensation',
    'redeem.emptyResponseRefundDetail': 'Empty response refund',
    'redeem.concurrencyAddedRedeem': 'Concurrency Added',
    'redeem.concurrencyAddedAdmin': 'Concurrency Added (Admin)',
    'redeem.concurrencyReducedAdmin': 'Concurrency Reduced (Admin)',
    'redeem.subscriptionAssigned': 'Subscription Assigned',
    'common.unknown': 'Unknown'
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params: Record<string, string> = {}) => {
        return (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, token) => params[token] ?? `{${token}}`)
      }
    })
  }
})

const SelectStub = {
  props: ['modelValue', 'options'],
  template: '<select><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
}

const user: AdminUser = {
  id: 1,
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin',
  balance: 1061.23,
  concurrency: 5,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-14T01:06:02Z',
  updated_at: '2026-04-14T01:06:02Z',
  notes: '',
  last_active_at: null,
  last_used_at: null,
  current_concurrency: 0
}

describe('UserBalanceHistoryModal check-in reward history', () => {
  beforeEach(() => {
    getUserBalanceHistory.mockReset()
    getUserBalanceHistory.mockResolvedValue({
      items: [
        {
          id: 88,
          code: '890eb190-checkin',
          type: 'checkin_reward',
          value: 2,
          status: 'used',
          used_by: 1,
          used_at: '2026-06-15T02:44:13Z',
          created_at: '2026-06-15T02:44:13Z',
          group_id: null,
          validity_days: 0,
          notes: 'daily check-in reward 2026-06-15'
        }
      ],
      total: 1,
      page: 1,
      page_size: 15,
      pages: 1,
      total_recharged: 1133
    })
  })

  it('renders check-in rewards as USD balance history instead of unknown activity', async () => {
    const wrapper = mount(UserBalanceHistoryModal, {
      props: {
        show: false,
        user
      },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' },
          Select: SelectStub,
          Icon: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(wrapper.text()).toContain('Daily Check-in Reward')
    expect(wrapper.text()).toContain('+$2.00')
    expect(wrapper.text()).not.toContain('Unknown')
  })

  it('renders empty-response refunds as localized balance compensation', async () => {
    getUserBalanceHistory.mockResolvedValueOnce({
      items: [
        {
          id: 99,
          code: 'EMPTY-COMP-88',
          type: 'empty_response',
          value: 0.1875,
          status: 'used',
          used_by: 1,
          used_at: '2026-08-27T14:27:17Z',
          created_at: '2026-08-27T14:27:17Z',
          group_id: null,
          validity_days: 0,
          notes: 'Empty response compensation for usage log #7669084'
        }
      ],
      total: 1,
      page: 1,
      page_size: 15,
      pages: 1,
      total_recharged: 1133
    })

    const wrapper = mount(UserBalanceHistoryModal, {
      props: {
        show: false,
        user
      },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /></div>' },
          Select: SelectStub,
          Icon: true
        }
      }
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(wrapper.text()).toContain('Empty Response Compensation')
    expect(wrapper.text()).toContain('+$0.19')
    expect(wrapper.text()).toContain('Usage record #7669084')
    expect(wrapper.get('select').text()).toContain('Empty Response Compensation')
    expect(wrapper.text()).not.toContain('Unknown')
    expect(wrapper.text()).not.toContain('Empty response compensation for usage log')
    expect(wrapper.text()).not.toContain('EMPTY-CO...')
  })
})
