import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AccountsView.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const ACCOUNT_FILTERS_STORAGE_KEY = 'account-list-filters'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
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
  props: ['searchQuery', 'filters'],
  emits: ['update:filters', 'update:searchQuery', 'change'],
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
      this.$emit('update:searchQuery', 'prod api')
      this.$emit('change')
    }
  },
  template: '<button data-test="apply-account-filters" @click="applySavedFilterScenario">apply filters</button>'
}

const DataTableStub = {
  props: ['columns', 'data'],
  template: '<div data-test="data-table"></div>'
}

function resetApiMocks() {
  listAccounts.mockReset()
  listWithEtag.mockReset()
  getBatchTodayStats.mockReset()
  getAllProxies.mockReset()
  getAllGroups.mockReset()

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

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
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
        Icon: true
      }
    }
  })
}

describe('admin AccountsView filter persistence', () => {
  beforeEach(() => {
    localStorage.clear()
    resetApiMocks()
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

  it('keeps the account module and table stage fluid inside the shared workspace', () => {
    expect(componentSource).toContain('accounts-admin-page w-full min-w-0 space-y-6')
    expect(componentSource).toContain('class="flex min-h-0 w-full min-w-0 flex-1 flex-col overflow-hidden"')
  })
})
