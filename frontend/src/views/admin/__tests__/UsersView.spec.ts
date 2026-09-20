import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import type { AdminUser } from '@/types'
import UsersView from '../UsersView.vue'

const currentDir = dirname(fileURLToPath(import.meta.url))
const usersViewSource = readFileSync(resolve(currentDir, '../UsersView.vue'), 'utf8')

const {
  listUsers,
  deleteUser,
  showError,
  showSuccess,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  deleteUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      toggleStatus: vi.fn(),
      delete: deleteUser,
      getPlatformQuotas: vi.fn().mockResolvedValue({ platform_quotas: [] })
    },
    groups: {
      getAll: getAllGroups,
      getAllIncludingInactive: vi.fn().mockResolvedValue([])
    },
    dashboard: {
      getBatchUsersUsage
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count?: number }) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const createAdminUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0,
  ...overrides
})

const DataTableStub = {
  props: ['columns', 'data', 'selectedKeys', 'mobileLayout'],
  emits: ['sort', 'update:selectedKeys'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <div data-test="row-order">{{ data.map(row => row.email).join(',') }}</div>
      <div data-test="selected-keys">{{ (selectedKeys || []).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <button
        v-for="row in data"
        :key="'select-' + row.id"
        :data-test="'select-' + row.id"
        @click="$emit('update:selectedKeys', Array.from(new Set([...(selectedKeys || []), row.id])))"
      >
        select
      </button>
      <template v-for="col in columns" :key="col.key">
        <slot :name="'header-' + col.key" :column="col" />
      </template>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-balance" :value="row.balance" :row="row" />
        <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
        <div data-test="attribute-cell"><slot name="cell-attr_7" :row="row" /></div>
      </div>
    </div>
  `
}

const AdminListToolbarStub = {
  props: ['activeFilters', 'filterId'],
  template: `<div class="admin-toolbar">
    <div data-test="toolbar-search"><slot name="search" /></div>
    <div data-test="toolbar-actions"><slot name="actions" /></div>
    <div data-test="toolbar-filters"><slot name="filters" /></div>
    <div data-test="toolbar-secondary"><slot name="secondary" /></div>
  </div>`
}

const PaginationStub = {
  emits: ['update:page'],
  template: '<button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button>'
}

const BulkEditUserModalStub = {
  props: ['show', 'selectedIds'],
  emits: ['close', 'success'],
  template: `
    <div v-if="show" data-test="bulk-modal">
      <span data-test="bulk-modal-ids">{{ selectedIds.join(',') }}</span>
      <button data-test="bulk-success" @click="$emit('success', selectedIds.length)">success</button>
    </div>
  `
}

const mountBulkDeleteView = (attachTo?: HTMLElement) => mount(UsersView, {
  attachTo,
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: DataTableStub,
      AdminListToolbar: AdminListToolbarStub,
      Pagination: PaginationStub,
      ConfirmDialog: {
        props: ['show', 'message'],
        emits: ['confirm', 'cancel'],
        template: `<div v-if="show" data-test="delete-dialog">
          <span>{{ message }}</span>
          <button data-test="confirm-delete" @click="$emit('confirm')">confirm</button>
          <button data-test="cancel-delete" @click="$emit('cancel')">cancel</button>
        </div>`
      },
      EmptyState: true,
      GroupBadge: true,
      Select: true,
      UserAttributesConfigModal: true,
      UserConcurrencyCell: true,
      UserCreateModal: true,
      UserEditModal: true,
      BulkEditUserModal: true,
      UserPlatformQuotaModal: true,
      UserApiKeysModal: true,
      UserAllowedGroupsModal: true,
      UserBalanceModal: true,
      UserBalanceHistoryModal: true,
      GroupReplaceModal: true,
      Icon: true,
      Teleport: true
    }
  }
})

describe('admin UsersView', () => {
  beforeEach(() => {
    vi.useRealTimers()
    localStorage.clear()

    listUsers.mockReset()
    deleteUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()

    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAllGroups.mockResolvedValue([])
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ attributes: {} })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('keeps the admin workspace toolbar surface and touch sized actions contract', () => {
    expect(usersViewSource).toContain('admin-workbench-page')
    expect(usersViewSource).toContain('<AdminListToolbar')
    expect(usersViewSource).toContain('users-select-all')
    expect(usersViewSource).toContain('min-height: 2.75rem')
    expect(usersViewSource).toContain('min-width: 2.75rem')
    expect(usersViewSource).toMatch(/overflow-wrap:\s*anywhere/)
  })

  it('keeps the mobile total and select-all controls in one summary bar', async () => {
    const wrapper = mountBulkDeleteView()
    await flushPromises()

    const secondary = wrapper.get('[data-test="toolbar-secondary"]')
    expect(secondary.get('[data-test="users-list-count"]').text()).toContain('1')
    const selectAll = secondary.get('[data-test="users-select-all"]')
    expect(selectAll.text()).toContain('common.selectAll')

    await selectAll.get('input').setValue(true)
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')
    wrapper.unmount()
  })

  it('keeps the balance history action accessible without a covering hover layer', async () => {
    const wrapper = mountBulkDeleteView()
    await flushPromises()

    const balanceButton = wrapper.findAll('button').find((button) => button.text().includes('$0.00'))
    expect(balanceButton).toBeDefined()
    expect(balanceButton?.attributes('aria-label')).toBe('admin.users.balanceHistoryTip')
    expect(balanceButton?.attributes('title')).toBeUndefined()
    const balanceCell = balanceButton?.element.parentElement?.parentElement
    expect(balanceCell?.querySelector('[class*="group-hover:opacity-100"]')).toBeNull()
    wrapper.unmount()
  })

  it('separates search, filters and selection actions while counting active saved filters', async () => {
    localStorage.setItem('user-visible-filters', JSON.stringify(['role', 'status', 'group', 'apiKeyGroup', 'attr_7']))
    localStorage.setItem('user-filter-values', JSON.stringify({
      role: 'user', status: '', group: 'Team', apiKeyGroup: 9, attributes: { 7: 'North', 8: '' }
    }))
    listEnabledDefinitions.mockResolvedValue([{ id: 7, name: 'Region', type: 'text', enabled: true }])
    listUsers.mockResolvedValue({ items: [createAdminUser()], total: 123, page: 1, page_size: 20, pages: 7 })
    const wrapper = mountBulkDeleteView()
    await flushPromises()

    expect(wrapper.find('.admin-workbench-page').exists()).toBe(true)
    const toolbar = wrapper.findComponent(AdminListToolbarStub)
    expect(toolbar.props('activeFilters')).toBe(4)
    expect(toolbar.props('filterId')).toBe('users-list-filters')
    expect(wrapper.get('[data-test="toolbar-search"]').find('input').exists()).toBe(true)
    expect(wrapper.get('[data-test="toolbar-search"]').find('select-stub').exists()).toBe(false)
    expect(wrapper.get('[data-test="toolbar-filters"]').findAll('select-stub')).toHaveLength(4)
    expect(wrapper.get('[data-test="toolbar-filters"]').get('input[placeholder="Region"]').element).toHaveProperty('value', 'North')
    const actions = wrapper.get('[data-test="toolbar-actions"]')
    await actions.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()
    expect(listUsers).toHaveBeenCalledTimes(2)
    await actions.get('button[title="admin.users.filterSettings"]').trigger('click')
    expect(actions.find('.dropdown').exists()).toBe(true)
    expect(actions.find('button[title="admin.users.columnSettings"]').exists()).toBe(true)
    const create = actions.get('button[title="admin.users.createUser"]')
    expect(create.get('span').text()).toBe('common.create')
    expect(create.get('span').classes()).not.toContain('hidden')
    await actions.get('button[title="admin.users.attributes.configButton"]').trigger('click')
    expect(wrapper.findComponent({ name: 'UserAttributesConfigModal' }).props('show')).toBe(true)
    expect(wrapper.get('[data-test="users-list-count"]').text()).toContain('123')

    await wrapper.get('[data-test="select-42"]').trigger('click')
    const secondary = wrapper.get('[data-test="toolbar-secondary"]')
    expect(secondary.find('[data-test="bulk-edit-limits"]').exists()).toBe(true)
    expect(secondary.find('[data-test="bulk-delete-users"]').exists()).toBe(true)
    expect(actions.find('[data-test="bulk-delete-users"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('configures compact mobile summaries without dropping visible columns or dynamic attribute values', async () => {
    vi.useFakeTimers()
    localStorage.setItem('user-hidden-columns', '[]')
    localStorage.setItem('user-column-settings-version', '4')
    listEnabledDefinitions.mockResolvedValue([{ id: 7, name: 'Region', type: 'text', enabled: true }])
    getBatchUserAttributes.mockResolvedValue({ attributes: { 42: { 7: 'North' } } })
    const wrapper = mountBulkDeleteView()
    await flushPromises()
    await vi.runAllTimersAsync()
    await flushPromises()

    const table = wrapper.findComponent(DataTableStub)
    expect(table.props('mobileLayout')).toEqual({
      title: 'email', subtitle: 'username', status: 'status', summary: ['balance', 'usage', 'concurrency']
    })
    expect(table.props('columns').map((column: { key: string }) => column.key)).toEqual([
      'email', 'id', 'username', 'notes', 'registration_ip', 'last_login_ip', 'attr_7', 'role',
      'groups', 'subscriptions', 'balance', 'balance_platform_quota', 'usage', 'usage_anthropic',
      'usage_openai', 'usage_gemini', 'usage_antigravity', 'concurrency', 'status', 'last_active_at',
      'last_used_at', 'created_at', 'actions'
    ])
    expect(wrapper.get('[data-test="attribute-cell"]').text()).toBe('North')
    expect(table.props('data')[0]).toMatchObject(createAdminUser())
    wrapper.unmount()
  })

  it.each(['admin.users.filterSettings', 'admin.users.columnSettings'])('dismisses %s with Escape and Tab and restores trigger focus', async (title) => {
    const wrapper = mountBulkDeleteView(document.body)
    await flushPromises()
    const trigger = wrapper.get<HTMLButtonElement>(`button[title="${title}"]`)
    for (const key of ['Escape', 'Tab']) {
      await trigger.trigger('click')
      await trigger.trigger('keydown', { key: 'Tab' })
      const menu = wrapper.get('.dropdown')
      const option = menu.get<HTMLButtonElement>('button')
      await trigger.trigger('keydown', { key: 'ArrowDown' })
      expect(document.activeElement).toBe(option.element)
      await option.trigger('keydown', { key: 'ArrowDown' })
      expect(document.activeElement).toBe(menu.findAll('button')[1].element)
      await option.trigger('keydown', { key })
      expect(wrapper.find('.dropdown').exists()).toBe(false)
      expect(document.activeElement).toBe(trigger.element)
    }
    wrapper.unmount()
  })

  it('cancels bulk deletion without deleting or clearing selected users', async () => {
    const wrapper = mountBulkDeleteView()
    await flushPromises()

    expect(wrapper.find('[data-test="bulk-delete-users"]').exists()).toBe(false)
    await wrapper.get('[data-test="select-42"]').trigger('click')
    await wrapper.get('[data-test="bulk-delete-users"]').trigger('click')
    expect(wrapper.get('[data-test="delete-dialog"]').text()).toContain('admin.users.bulkDelete.confirm:1')
    expect(deleteUser).not.toHaveBeenCalled()

    await wrapper.get('[data-test="cancel-delete"]').trigger('click')
    expect(wrapper.find('[data-test="delete-dialog"]').exists()).toBe(false)
    expect(deleteUser).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')
    wrapper.unmount()
  })

  it.each([
    { failedIds: [], remaining: '', deleted: 2 },
    { failedIds: [43], remaining: '43', deleted: 1 },
    { failedIds: [42, 43], remaining: '42,43', deleted: 0 }
  ])('deletes across pages and retains failures: $remaining', async ({ failedIds, remaining, deleted }) => {
    listUsers.mockImplementation(async (page: number) => ({
      items: [createAdminUser({ id: page === 2 ? 43 : 42 })],
      total: 2, page, page_size: 20, pages: 2
    }))
    deleteUser.mockImplementation(async (id: number) => {
      if (failedIds.includes(id)) throw new Error('Cannot delete user')
    })
    const wrapper = mountBulkDeleteView()
    await flushPromises()
    await wrapper.get('[data-test="select-42"]').trigger('click')
    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="select-43"]').trigger('click')
    await wrapper.get('[data-test="bulk-delete-users"]').trigger('click')
    expect(deleteUser).not.toHaveBeenCalled()

    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    await flushPromises()

    expect(deleteUser.mock.calls).toEqual([[42], [43]])
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe(remaining)
    expect(wrapper.find('[data-test="delete-dialog"]').exists()).toBe(false)
    if (deleted) {
      expect(showSuccess).toHaveBeenCalledWith(`admin.users.bulkDelete.success:${deleted}`)
      expect(listUsers.mock.lastCall?.[0]).toBe(1)
    } else {
      expect(showSuccess).not.toHaveBeenCalled()
    }
    if (failedIds.length) expect(showError).toHaveBeenCalledWith(`admin.users.bulkDelete.failed:${failedIds.length}`)
    else expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('deletes the confirmed selection while preserving users selected during deletion', async () => {
    listUsers.mockResolvedValue({
      items: [createAdminUser({ id: 42 }), createAdminUser({ id: 43 })],
      total: 2, page: 1, page_size: 20, pages: 1
    })
    let finishDelete!: () => void
    deleteUser.mockImplementation(() => new Promise<void>(resolve => { finishDelete = resolve }))
    const wrapper = mountBulkDeleteView()
    await flushPromises()
    await wrapper.get('[data-test="select-42"]').trigger('click')
    await wrapper.get('[data-test="bulk-delete-users"]').trigger('click')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    expect(wrapper.get('[data-test="bulk-delete-users"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="select-43"]').trigger('click')
    finishDelete()
    await flushPromises()

    expect(deleteUser.mock.calls).toEqual([[42]])
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('43')
    wrapper.unmount()
  })

  it('shows active, used, and created activity columns in order and requests last_used_at sort', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><div data-test="filters-shell" class="admin-toolbar-surface"><slot name="filters" /></div><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text()
    const visibleColumns = columns.split(',')
    expect(visibleColumns.slice(-4, -1)).toEqual(['last_active_at', 'last_used_at', 'created_at'])
    expect(visibleColumns).not.toContain('last_login_at')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('uses the shared dropdown shell for filter settings', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><div data-test="filters-shell" class="admin-toolbar-surface"><slot name="filters" /></div><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const filterButton = wrapper.findAll('button').find((candidate) => candidate.attributes('title') === 'admin.users.filterSettings')
    if (!filterButton) {
      throw new Error('filter settings button not found')
    }

    await filterButton.trigger('click')
    await flushPromises()

    const dropdown = wrapper.find('.dropdown')
    expect(dropdown.exists()).toBe(true)
  })

  it('renders the users workspace without the redundant page hero', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><div data-test="filters-shell" class="admin-toolbar-surface"><slot name="filters" /></div><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.find('[data-test="admin-page-hero"]').exists()).toBe(false)
    expect(wrapper.find('.admin-page-hero').exists()).toBe(false)
    expect(wrapper.get('[data-test="filters-shell"]').findComponent({ name: 'AdminListToolbar' }).exists()).toBe(true)
  })

  it('keeps usage sort controls on shared admin inline actions', () => {
    expect(usersViewSource).toContain('admin-inline-action')
    expect(usersViewSource).not.toContain('hover:bg-gray-200 dark:hover:bg-dark-700')
  })

  it('clears usage current-page sort when switching to last_used_at server sort', async () => {
    vi.useFakeTimers()
    localStorage.setItem('user-column-settings-version', '3')
    localStorage.setItem(
      'user-hidden-columns',
      JSON.stringify([
        'notes',
        'groups',
        'subscriptions',
        'concurrency',
        'usage_anthropic',
        'usage_openai',
        'usage_gemini',
        'usage_antigravity',
        'balance_platform_quota'
      ])
    )

    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 1, email: 'last-used-first@example.com' }),
        createAdminUser({ id: 2, email: 'usage-first@example.com' })
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getBatchUsersUsage.mockResolvedValue({
      stats: {
        1: { user_id: 1, today_actual_cost: 1, total_actual_cost: 1, by_platform: [] },
        2: { user_id: 2, today_actual_cost: 9, total_actual_cost: 9, by_platform: [] }
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(50)
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')

    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="usage-sort-usage-today"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('usage-first@example.com,last-used-first@example.com')
    expect(localStorage.getItem('admin-users-usage-sort')).toContain('"key":"usage"')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem('admin-users-usage-sort')).toBeNull()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('keeps selected user IDs across pages and clears them after a successful bulk update', async () => {
    let refreshed = false
    listUsers.mockImplementation(async (page: number) => {
      const user = page === 2
        ? createAdminUser({
            id: 43,
            email: refreshed ? 'refreshed-page-two@example.com' : 'page-two@example.com'
          })
        : createAdminUser({ id: 42, email: 'page-one@example.com' })
      return {
        items: [user],
        total: 2,
        page,
        page_size: 20,
        pages: 2
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    await wrapper.get('[data-test="select-42"]').trigger('click')
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(true)

    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')

    await wrapper.get('[data-test="select-43"]').trigger('click')
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42,43')

    await wrapper.get('[data-test="bulk-edit-limits"]').trigger('click')
    expect(wrapper.get('[data-test="bulk-modal-ids"]').text()).toBe('42,43')

    const callsBeforeSuccess = listUsers.mock.calls.length
    refreshed = true
    await wrapper.get('[data-test="bulk-success"]').trigger('click')
    await flushPromises()

    expect(listUsers.mock.calls.length).toBeGreaterThan(callsBeforeSuccess)
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('refreshed-page-two@example.com')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('')
  })
})
