import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { flushPromises, mount } from '@vue/test-utils'

import CheckinsView from '../CheckinsView.vue'

const currentDir = dirname(fileURLToPath(import.meta.url))
const checkinsViewPath = resolve(currentDir, '../CheckinsView.vue')

const {
  getStats,
  listRecords,
  listBlacklist,
  addBlacklist,
  removeBlacklist,
  listUsers,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  getStats: vi.fn(),
  listRecords: vi.fn(),
  listBlacklist: vi.fn(),
  addBlacklist: vi.fn(),
  removeBlacklist: vi.fn(),
  listUsers: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    checkins: {
      getStats,
      listRecords,
      listBlacklist,
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

describe('Admin CheckinsView', () => {
  beforeEach(() => {
    getStats.mockReset()
    listRecords.mockReset()
    listBlacklist.mockReset()
    addBlacklist.mockReset()
    removeBlacklist.mockReset()
    listUsers.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

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

  it('keeps check-in settings panels and picker rows on shared admin styles', () => {
    const source = readFileSync(checkinsViewPath, 'utf8')

    expect(source).toContain('admin-form-section')
    expect(source).toContain('admin-list-surface')
    expect(source).toContain('admin-list-row')

    expect(source).not.toContain('rounded-lg bg-gray-50')
    expect(source).not.toContain('hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700')
    expect(source).not.toContain('border-gray-200 bg-gray-50')
  })
})
