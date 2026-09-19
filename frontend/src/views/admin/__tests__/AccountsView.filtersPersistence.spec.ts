import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import SearchInput from '@/components/common/SearchInput.vue'
import AccountBulkActionsBar from '@/components/admin/account/AccountBulkActionsBar.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

import AccountsView from '../AccountsView.vue'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AccountsView.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const ACCOUNT_FILTERS_STORAGE_KEY = 'account-list-filters'
let mediaQuerySpy: { mockRestore: () => void } | undefined
enableAutoUnmount(afterEach)

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
  getAccountDetails,
  getUpstreamBillingProbeSettings
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  getAccountDetails: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      getById: getAccountDetails,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token',
    isSimpleMode: false
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const AccountTableFiltersStub = {
  props: ['filters'],
  emits: ['update:filters', 'change'],
  methods: {
    applySavedFilterScenario() {
      this.$emit('update:filters', {
        ...this.filters,
        platform: 'openai',
        type: 'oauth',
        status: 'error',
        privacy_mode: 'training_off',
        group: 'ungrouped'
      })
      this.$emit('change')
    }
  },
  template: '<button data-test="apply-account-filters" @click="applySavedFilterScenario">apply filters</button>'
}

const DataTableStub = {
  props: ['columns', 'data', 'mobileLayout'],
  template: '<div data-test="data-table"></div>'
}

const AdminListToolbarStub = {
  props: ['activeFilters', 'filterId'],
  template: `<div data-test="account-toolbar">
    <div data-test="toolbar-search"><slot name="search" /></div>
    <div data-test="toolbar-actions"><slot name="actions" /></div>
    <div data-test="toolbar-filters"><slot name="filters" /></div>
    <div data-test="toolbar-secondary"><slot name="secondary" /></div>
  </div>`
}

function resetApiMocks() {
  listAccounts.mockReset()
  listWithEtag.mockReset()
  getBatchTodayStats.mockReset()
  getAllProxies.mockReset()
  getAllGroups.mockReset()
  getAccountDetails.mockReset()
  getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: false })

  listAccounts.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
    pages: 0
  })
  listWithEtag.mockResolvedValue({
    notModified: true,
    etag: null,
    data: null
  })
  getBatchTodayStats.mockResolvedValue({ stats: {} })
  getAllProxies.mockResolvedValue([])
  getAllGroups.mockResolvedValue([])
}

function mountView(realTable = false) {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: realTable ? false : DataTableStub,
        AdminListToolbar: AdminListToolbarStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableFilters: AccountTableFiltersStub,
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        UpstreamBillingRateCell: true,
        Icon: true
      }
    }
  })
}

describe('admin AccountsView filter persistence', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'] })
    localStorage.clear()
    resetApiMocks()
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
    mediaQuerySpy?.mockRestore()
    mediaQuerySpy = undefined
  })

  it('restores saved filters before the first account list request', async () => {
    localStorage.setItem(
      ACCOUNT_FILTERS_STORAGE_KEY,
      JSON.stringify({
        platform: 'anthropic',
        type: 'oauth',
        status: 'active',
        privacy_mode: '__unset__',
        group: '42',
        search: 'production'
      })
    )

    mountView()
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        platform: 'anthropic',
        type: 'oauth',
        status: 'active',
        privacy_mode: '__unset__',
        group: '42',
        search: 'production'
      }),
      expect.any(Object)
    )
  })

  it('persists the latest filters when admins change account filters', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="apply-account-filters"]').trigger('click')
    await wrapper.getComponent(SearchInput).get('input').setValue('prod api')
    await flushPromises()

    expect(JSON.parse(localStorage.getItem(ACCOUNT_FILTERS_STORAGE_KEY) || '{}')).toEqual({
      platform: 'openai',
      type: 'oauth',
      status: 'error',
      privacy_mode: 'training_off',
      group: 'ungrouped',
      search: 'prod api'
    })
  })

  it('keeps search and actions above filters and counts only active select filters', async () => {
    const wrapper = mountView()
    await flushPromises()
    const toolbar = wrapper.getComponent(AdminListToolbarStub)
    expect(toolbar.props('activeFilters')).toBe(0)
    expect(wrapper.get('[data-test="toolbar-search"]').findComponent(SearchInput).exists()).toBe(true)
    expect(wrapper.get('[data-test="toolbar-filters"]').findComponent(AccountTableFiltersStub).exists()).toBe(true)
    expect(wrapper.get('[data-test="toolbar-secondary"]').find('account-bulk-actions-bar-stub').exists()).toBe(true)

    await wrapper.getComponent(SearchInput).get('input').setValue('prod api')
    expect(toolbar.props('activeFilters')).toBe(0)
    await wrapper.get('[data-test="apply-account-filters"]').trigger('click')
    expect(toolbar.props('activeFilters')).toBe(5)
  })

  it('opts into compact mobile account fields without removing desktop columns', async () => {
    const wrapper = mountView()
    await flushPromises()
    const table = wrapper.getComponent(DataTableStub)
    expect(wrapper.find('.admin-workbench-page').exists()).toBe(true)
    expect(table.props('mobileLayout')).toEqual({
      title: 'name', subtitle: 'id', leading: 'platform_type', status: 'status', selection: 'select',
      summary: ['capacity', 'usage', 'schedulable']
    })
    const columnKeys = table.props('columns').map((column: { key: string }) => column.key)
    expect(columnKeys).toEqual(expect.arrayContaining(['select', 'name', 'id', 'platform_type', 'capacity', 'usage', 'schedulable', 'priority', 'actions']))
  })

  it('retains refresh and persistent column controls in the action row', async () => {
    const wrapper = mountView()
    await flushPromises()
    listAccounts.mockClear()
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(1)

    await wrapper.get('button[title="admin.accounts.moreActions"]').trigger('click')
    await flushPromises()
    const notesButton = Array.from(document.body.querySelectorAll('button')).find(button => button.textContent?.trim() === 'admin.accounts.columns.notes')
    expect(notesButton).toBeDefined()
    notesButton!.click()
    await flushPromises()
    expect(JSON.parse(localStorage.getItem('account-hidden-columns') || '[]')).not.toContain('notes')
    expect(wrapper.getComponent(DataTableStub).props('columns').some((column: { key: string }) => column.key === 'notes')).toBe(true)
  })

  it('keeps create reachable from the compact action row', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('button[aria-label="admin.accounts.createAccount"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('create-account-modal-stub').attributes('show')).toBe('true')
  })

  it('keeps mobile selection and primary actions outside details without duplicating the provider', async () => {
    const originalMatchMedia = window.matchMedia.bind(window)
    mediaQuerySpy = vi.spyOn(window, 'matchMedia').mockImplementation(query => ({
      ...originalMatchMedia(query), matches: query.includes('max-width')
    }))
    const account = {
      id: 7, name: 'Production account', platform: 'openai', type: 'apikey',
      status: 'active', schedulable: true, concurrency: 10, priority: 50,
      group_ids: [], credentials: {}, extra: {}, created_at: '2026-09-19T00:00:00Z',
      last_used_at: '2026-09-18T00:00:00Z'
    }
    listAccounts.mockResolvedValue({ items: [account], total: 1, page: 1, page_size: 20, pages: 1 })
    getAccountDetails.mockResolvedValue(account)
    const wrapper = mountView(true)
    await flushPromises()
    const card = wrapper.get('[data-mobile-table-row]')
    const details = card.get('details')
    expect(details.attributes('open')).toBeUndefined()
    expect(details.find('[data-field="platform_type"]').exists()).toBe(false)
    expect(card.findAll('[data-field="platform_type"]')).toHaveLength(1)
    expect(card.get('[data-test="mobile-record-summary"]').findAll('.admin-record-metric')).toHaveLength(3)

    const actions = card.get('.admin-record-actions')
    expect(details.find('.admin-record-actions').exists()).toBe(false)
    await actions.get('button[aria-label="common.edit"]').trigger('click')
    await flushPromises()
    expect(getAccountDetails).toHaveBeenCalledWith(7)
    expect(wrapper.find('edit-account-modal-stub').attributes('show')).toBe('true')

    await actions.get('button[aria-label="common.delete"]').trigger('click')
    expect(wrapper.findAllComponents(ConfirmDialog).find(dialog => dialog.props('title') === 'admin.accounts.deleteAccount')?.props('show')).toBe(true)
    await actions.get('button[aria-label="common.more"]').trigger('click')
    expect(wrapper.getComponent(AccountActionMenu).props('show')).toBe(true)
    expect(wrapper.getComponent(AccountActionMenu).props('account')).toMatchObject({ id: 7 })

    const selection = card.get('.admin-record-header input[type="checkbox"]')
    expect(selection.attributes('data-test')).toBe('account-select-row')
    expect(selection.attributes('aria-label')).toContain(account.name)
    expect(details.find('[data-field="select"]').exists()).toBe(false)
    await selection.setValue(true)
    expect(wrapper.getComponent(AccountBulkActionsBar).props('selectedIds')).toEqual([7])
  })

  it('keeps the account module and table stage fluid inside the shared workspace', () => {
    expect(componentSource).toContain('accounts-admin-page w-full min-w-0 space-y-6')
    expect(componentSource).toContain('class="flex min-h-0 w-full min-w-0 flex-1 flex-col overflow-hidden"')
  })
})
