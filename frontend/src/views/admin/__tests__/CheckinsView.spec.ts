import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { flushPromises, mount } from '@vue/test-utils'

import CheckinsView from '../CheckinsView.vue'

const currentDir = dirname(fileURLToPath(import.meta.url))
const checkinsViewPath = resolve(currentDir, '../CheckinsView.vue')

const {
  getConfig,
  updateConfig,
  getStats,
  listRecords,
  listBlacklist,
  listCampaigns,
  addBlacklist,
  removeBlacklist,
  listUsers,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  getStats: vi.fn(),
  listRecords: vi.fn(),
  listBlacklist: vi.fn(),
  listCampaigns: vi.fn(),
  addBlacklist: vi.fn(),
  removeBlacklist: vi.fn(),
  listUsers: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    checkins: {
      getConfig,
      updateConfig,
      getStats,
      listRecords,
      listBlacklist,
      listCampaigns,
      addBlacklist,
      removeBlacklist,
    },
    users: {
      list: listUsers,
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
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.checkins.userId') return `User #${params?.id}`
        if (key === 'admin.checkins.rewardCampaignLabel') return `Reward campaign: ${params?.name}`
        return key
      },
    }),
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <tbody>
        <tr v-for="row in data" :key="row.id">
          <td v-for="column in columns" :key="column.key">
            <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  `,
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

describe('Admin CheckinsView', () => {
  beforeEach(() => {
    getConfig.mockReset()
    updateConfig.mockReset()
    getStats.mockReset()
    listRecords.mockReset()
    listBlacklist.mockReset()
    listCampaigns.mockReset()
    addBlacklist.mockReset()
    removeBlacklist.mockReset()
    listUsers.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    const config = {
      enabled: true,
      min_total_usage_usd: 5,
      min_total_recharge_usd: 20,
      min_daily_usage_count: 5,
      tiers: [{ amount: 1, probability: 100, sort_order: 1 }],
      streak_enabled: false,
      streak_rules: [],
      usage_rebate_enabled: true,
      usage_rebate_rate_percent: 8,
      usage_rebate_cap: 8,
      total_reward_cap: 10,
      probability_total: 100,
      preview: { min_reward: 1, max_reward: 1, average_reward: 1 },
    }
    getConfig.mockResolvedValue(config)
    updateConfig.mockImplementation(async (request) => ({ ...config, ...request }))

    getStats.mockResolvedValue({
      today_count: 3,
      today_reward_total: 9,
      seven_day_count: 12,
      seven_day_reward_total: 34,
      thirty_day_count: 40,
      thirty_day_reward_total: 120,
      active_blacklist_count: 1,
      current_checkin_day: '2026-06-05',
    })
    listRecords.mockResolvedValue({
      items: [
        {
          id: 1,
          user_id: 99,
          user_email: 'alice@example.com',
          username: 'alice',
          checkin_date: '2026-06-05',
          reward_amount: 3,
          base_reward_amount: 0.5,
          previous_day_usage_amount: 25,
          usage_rebate_amount: 2,
          bonus_reward_amount: 0.5,
          lottery_attempts_reward: 2,
          reward_cap_adjustment: 0,
          total_reward_amount: 3,
          balance_before: 10,
          balance_after: 13,
          created_at: '2026-06-05T01:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    listBlacklist.mockResolvedValue({
      items: [
        {
          id: 8,
          user_id: 88,
          user_email: 'blocked@example.com',
          username: 'blocked',
          reason: 'manual review',
          created_by: 1,
          removed_by: null,
          removed_at: null,
          created_at: '2026-06-04T01:00:00Z',
          updated_at: '2026-06-04T01:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    listCampaigns.mockResolvedValue([])
    listUsers.mockResolvedValue({
      items: [
        {
          id: 99,
          email: 'alice@example.com',
          username: 'alice',
          role: 'user',
          balance: 10,
          concurrency: 1,
          status: 'active',
          allowed_groups: null,
          balance_notify_enabled: false,
          balance_notify_threshold: null,
          balance_notify_extra_emails: [],
          created_at: '2026-06-01T00:00:00Z',
          updated_at: '2026-06-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 8,
      pages: 1,
    })
    addBlacklist.mockResolvedValue({ id: 9, user_id: 99 })
  })

  it('renders check-in stats and lets an admin add a blacklist entry after user search', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('3')
    expect(wrapper.text()).toContain('$9.00')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('blocked@example.com')

    await wrapper.get('[data-test="blacklist-user-search"]').setValue('alice')
    await wrapper.get('[data-test="search-blacklist-user"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="select-blacklist-user-99"]').trigger('click')
    await wrapper.get('[data-test="blacklist-reason"]').setValue('abuse')
    await wrapper.get('[data-test="add-blacklist"]').trigger('click')
    await flushPromises()

    expect(addBlacklist).toHaveBeenCalledWith({ user_id: 99, reason: 'abuse' })
    expect(showSuccess).toHaveBeenCalledWith('admin.checkins.blacklistAdded')
  })

  it('renders structured admin surfaces without the redundant page hero', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-test="admin-page-hero"]').exists()).toBe(false)
    expect(wrapper.find('.admin-page-hero').exists()).toBe(false)
    expect(wrapper.find('[data-test="checkins-action-toolbar"]').classes()).toContain('admin-toolbar-surface')
    expect(wrapper.findAll('.checkins-stat-card').length).toBeGreaterThan(0)
    expect(wrapper.findAll('.admin-toolbar-surface').length).toBeGreaterThan(0)
    expect(wrapper.findAll('.admin-panel-header').length).toBeGreaterThan(0)
  })

  it('mounts campaign management between baseline settings and records without coupling mutations to config saving', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
          CheckinRewardCampaignPanel: {
            props: ['defaultTiers'],
            template: '<section data-test="campaign-panel-stub">{{ defaultTiers.length }}</section>',
          },
        },
      },
    })

    await flushPromises()

    const baseline = wrapper.get('.checkins-editor-card').element
    const campaigns = wrapper.get('[data-test="campaign-panel-stub"]').element
    const records = wrapper.get('[data-test="checkin-records-section"]').element
    expect(baseline.compareDocumentPosition(campaigns) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(campaigns.compareDocumentPosition(records) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(wrapper.get('[data-test="campaign-panel-stub"]').text()).toBe('1')
    expect(updateConfig).not.toHaveBeenCalled()
  })

  it('keeps campaign creation unavailable until baseline tiers finish loading', async () => {
    const configRequest = deferred<{
      enabled: boolean
      min_total_usage_usd: number
      min_total_recharge_usd: number
      min_daily_usage_count: number
      tiers: Array<{ amount: number; probability: number; sort_order: number }>
      streak_enabled: boolean
      streak_rules: Array<{ day: number; lottery_attempts: number }>
      usage_rebate_enabled: boolean
      usage_rebate_rate_percent: number
      usage_rebate_cap: number
      total_reward_cap: number
      probability_total: number
      preview: { min_reward: number; max_reward: number; average_reward: number }
    }>()
    getConfig.mockReturnValueOnce(configRequest.promise)

    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    const createButton = wrapper.get('[data-test="campaign-create"]')
    expect(createButton.attributes('disabled')).toBeDefined()
    expect(createButton.attributes('aria-busy')).toBe('true')
    expect(document.body.querySelector('[data-test="campaign-dialog"]')).toBeNull()
    await createButton.trigger('click')
    expect(document.body.querySelector('[data-test="campaign-dialog"]')).toBeNull()

    configRequest.resolve({
      enabled: true,
      min_total_usage_usd: 5,
      min_total_recharge_usd: 20,
      min_daily_usage_count: 5,
      tiers: [
        { amount: 2.5, probability: 60, sort_order: 1 },
        { amount: 4, probability: 40, sort_order: 2 },
      ],
      streak_enabled: false,
      streak_rules: [],
      usage_rebate_enabled: false,
      usage_rebate_rate_percent: 0,
      usage_rebate_cap: 0,
      total_reward_cap: 0,
      probability_total: 100,
      preview: { min_reward: 2.5, max_reward: 4, average_reward: 3.1 },
    })
    await flushPromises()

    expect(createButton.attributes('disabled')).toBeUndefined()
    expect(createButton.attributes('aria-busy')).toBeUndefined()
    await createButton.trigger('click')
    await flushPromises()
    expect(document.body.querySelector('[data-test="campaign-dialog"]')).not.toBeNull()
    const firstAmount = document.body.querySelector<HTMLInputElement>('[data-test="campaign-tier-amount-0"]')
    const secondAmount = document.body.querySelector<HTMLInputElement>('[data-test="campaign-tier-amount-1"]')
    expect(firstAmount?.value).toBe('2.5')
    expect(secondAmount?.value).toBe('4')
  })

  it('keeps campaign creation disabled when baseline config loading fails', async () => {
    getConfig.mockRejectedValueOnce(new Error('baseline unavailable'))
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    const createButton = wrapper.get('[data-test="campaign-create"]')
    expect(createButton.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="campaign-create-status"]').text()).toContain(
      'admin.checkins.campaigns.defaultTiersUnavailable'
    )
  })

  it('keeps check-in settings panels and picker rows on shared admin styles', () => {
    const source = readFileSync(checkinsViewPath, 'utf8')

    expect(source).toContain('admin-form-section')
    expect(source).toContain('admin-list-surface')
    expect(source).toContain('admin-list-row')

    expect(source).not.toContain('rounded-lg bg-gray-50')
    expect(source).not.toContain('hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700')
    expect(source).not.toContain('border-gray-200 bg-gray-50')
  })

  it('loads and saves the cumulative recharge threshold', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    const rechargeInput = wrapper.get('[data-test="min-total-recharge-usd"]')
    expect((rechargeInput.element as HTMLInputElement).value).toBe('20')
    await rechargeInput.setValue('25')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      min_total_usage_usd: 5,
      min_total_recharge_usd: 25,
    }))
  })

  it('loads and saves the daily usage-count threshold', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    const usageCountInput = wrapper.get('[data-test="min-daily-usage-count"]')
    expect((usageCountInput.element as HTMLInputElement).value).toBe('5')
    await usageCountInput.setValue('8')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      min_daily_usage_count: 8,
    }))
  })

  it('rejects a negative cumulative recharge threshold', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-test="min-total-recharge-usd"]').setValue('-1')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')

    expect(updateConfig).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.checkins.invalidMinTotalRechargeUsd')
  })

  it('loads, previews, and saves usage-linked reward settings', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect((wrapper.get('[data-test="usage-rebate-rate"]').element as HTMLInputElement).value).toBe('8')
    expect((wrapper.get('[data-test="usage-rebate-cap"]').element as HTMLInputElement).value).toBe('8')
    expect((wrapper.get('[data-test="total-reward-cap"]').element as HTMLInputElement).value).toBe('10')
    expect(wrapper.get('[data-test="usage-rebate-preview-0"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-test="usage-rebate-preview-10"]').text()).toContain('$0.80')
    expect(wrapper.get('[data-test="usage-rebate-preview-20"]').text()).toContain('$1.60')
    expect(wrapper.get('[data-test="usage-rebate-preview-50"]').text()).toContain('$4.00')
    expect(wrapper.get('[data-test="usage-rebate-preview-100"]').text()).toContain('$8.00')
    expect(wrapper.get('[data-test="record-previous-day-usage"]').text()).toContain('$25.00')
    expect(wrapper.get('[data-test="record-base-reward"]').text()).toContain('$0.50')
    expect(wrapper.get('[data-test="record-usage-rebate"]').text()).toContain('$2.00')
    expect(wrapper.get('[data-test="record-streak-bonus"]').text()).toContain('$0.50')
    expect(wrapper.get('[data-test="record-lottery-attempts"]').text()).toContain('2')
    expect(wrapper.get('[data-test="record-cap-adjustment"]').text()).toContain('$0.00')

    await wrapper.get('[data-test="usage-rebate-rate"]').setValue('6')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      usage_rebate_enabled: true,
      usage_rebate_rate_percent: 6,
      usage_rebate_cap: 8,
      total_reward_cap: 10,
    }))
  })

  it('shows the stored campaign origin beside an admin reward breakdown without changing amounts', async () => {
    const longCampaignName = 'C'.repeat(120)
    expect(longCampaignName).toHaveLength(120)
    listRecords.mockResolvedValueOnce({
      items: [
        {
          id: 2,
          user_id: 99,
          user_email: 'alice@example.com',
          username: 'alice',
          checkin_date: '2026-08-15',
          reward_amount: 4.8,
          base_reward_amount: 0.8,
          previous_day_usage_amount: 50,
          usage_rebate_amount: 3,
          bonus_reward_amount: 1,
          reward_cap_adjustment: 0,
          total_reward_amount: 4.8,
          reward_campaign_id: 42,
          reward_campaign_name: `  ${longCampaignName}  `,
          balance_before: 10,
          balance_after: 14.8,
          created_at: '2026-08-15T01:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    document.body.appendChild(wrapper.element)

    const campaignChip = wrapper.get('[data-test="record-reward-campaign"]')
    campaignChip.element.parentElement?.setAttribute('style', 'width: 8rem')
    expect(campaignChip.text()).toBe(longCampaignName)
    expect(campaignChip.attributes('title')).toBe(`Reward campaign: ${longCampaignName}`)
    expect(campaignChip.attributes('aria-label')).toBe(`Reward campaign: ${longCampaignName}`)
    expect(campaignChip.attributes('tabindex')).toBe('0')
    expect(campaignChip.attributes('role')).toBe('note')
    expect(campaignChip.classes()).toContain('max-w-full')
    expect(campaignChip.classes()).toContain('whitespace-normal')
    expect(campaignChip.classes()).toContain('break-words')
    expect(campaignChip.classes()).toContain('[overflow-wrap:anywhere]')
    expect(campaignChip.classes()).not.toContain('truncate')
    expect(campaignChip.element).toBeInstanceOf(HTMLElement)
    if (campaignChip.element instanceof HTMLElement) campaignChip.element.focus()
    expect(document.activeElement).toBe(campaignChip.element)
    expect(wrapper.get('[data-test="record-base-reward"]').text()).toBe('$0.80')
    expect(wrapper.get('[data-test="record-usage-rebate"]').text()).toBe('$3.00')
    expect(wrapper.get('[data-test="record-streak-bonus"]').text()).toBe('$1.00')
    expect(wrapper.get('[data-test="record-total-reward"]').text()).toBe('$4.80')
    wrapper.unmount()
  })

  it('does not render an empty campaign chip for baseline admin records', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-test="record-reward-campaign"]').exists()).toBe(false)
  })

  it('configures streak rewards as integer lottery attempts', async () => {
    getConfig.mockResolvedValueOnce({
      enabled: true,
      min_total_usage_usd: 5,
      min_total_recharge_usd: 20,
      min_daily_usage_count: 5,
      tiers: [{ amount: 1, probability: 100, sort_order: 1 }],
      streak_enabled: true,
      streak_rules: [{ day: 7, lottery_attempts: 4 }],
      usage_rebate_enabled: true,
      usage_rebate_rate_percent: 8,
      usage_rebate_cap: 8,
      total_reward_cap: 10,
      probability_total: 100,
      preview: { min_reward: 1, max_reward: 1, average_reward: 1 },
    })

    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    const amountInput = wrapper.get('[data-test="streak-lottery-attempts"]')
    expect((amountInput.element as HTMLInputElement).value).toBe('4')
    expect((amountInput.element as HTMLInputElement).step).toBe('1')

    await amountInput.setValue('5')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({
      usage_rebate_enabled: true,
      streak_rules: [{ day: 7, lottery_attempts: 5 }],
    }))
  })

  it('validates enabled usage-linked reward settings before saving', async () => {
    const wrapper = mount(CheckinsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-test="usage-rebate-rate"]').setValue('101')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    expect(updateConfig).not.toHaveBeenCalled()
    expect(showError).toHaveBeenLastCalledWith('admin.checkins.invalidUsageRebateRate')

    await wrapper.get('[data-test="usage-rebate-rate"]').setValue('8')
    await wrapper.get('[data-test="usage-rebate-cap"]').setValue('0')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    expect(updateConfig).not.toHaveBeenCalled()
    expect(showError).toHaveBeenLastCalledWith('admin.checkins.invalidUsageRebateCap')

    await wrapper.get('[data-test="usage-rebate-cap"]').setValue('8')
    await wrapper.get('[data-test="total-reward-cap"]').setValue('0.5')
    await wrapper.get('[data-test="save-checkin-config"]').trigger('click')
    expect(updateConfig).not.toHaveBeenCalled()
    expect(showError).toHaveBeenLastCalledWith('admin.checkins.totalRewardCapBelowTier')
  })
})
