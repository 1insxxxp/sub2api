import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { DOMWrapper } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import ChannelsView from '../ChannelsView.vue'

const currentDir = dirname(fileURLToPath(import.meta.url))

const {
  listChannels,
  getAllGroups,
  getWebSearchEmulationConfig,
  showError,
  showSuccess,
  triggerDebouncedSearch,
  clearDebouncedSearch,
} = vi.hoisted(() => ({
  listChannels: vi.fn(),
  getAllGroups: vi.fn(),
  getWebSearchEmulationConfig: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  triggerDebouncedSearch: vi.fn(),
  clearDebouncedSearch: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channels: {
      list: listChannels,
      create: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
      syncPricingModels: vi.fn(),
    },
    groups: {
      getAll: getAllGroups,
    },
    settings: {
      getWebSearchEmulationConfig,
    },
    accounts: {
      list: vi.fn(),
      getById: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
}))

vi.mock('@/composables/useKeyedDebouncedSearch', () => ({
  useKeyedDebouncedSearch: () => ({
    trigger: triggerDebouncedSearch,
    clearAll: clearDebouncedSearch,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, arg2?: unknown, arg3?: string) => {
        if (typeof arg3 === 'string') return arg3
        if (typeof arg2 === 'string') return arg2
        return key
      },
    }),
  }
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div data-test="table-page-layout">
      <div data-test="toolbar" class="admin-toolbar-surface"><slot name="filters" /></div>
      <div data-test="table-shell" class="admin-table-stage"><slot name="table" /></div>
      <div data-test="pagination-shell" class="admin-pagination-surface"><slot name="pagination" /></div>
    </div>
  `,
}

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <div data-test="row-count">{{ data.length }}</div>
      <slot v-if="data.length === 0" name="empty" />
    </div>
  `,
}

const EmptyStateStub = defineComponent({
  props: {
    title: {
      type: String,
      default: '',
    },
    description: {
      type: String,
      default: '',
    },
    actionText: {
      type: String,
      default: '',
    },
  },
  emits: ['action'],
  template: `
    <div data-test="empty-state">
      <div data-test="empty-title">{{ title }}</div>
      <div data-test="empty-description">{{ description }}</div>
      <button data-test="empty-action" @click="$emit('action')">{{ actionText }}</button>
    </div>
  `,
})

const BaseDialogStub = defineComponent({
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '',
    },
  },
  emits: ['close'],
  template: `
    <div v-if="show" data-test="base-dialog">
      <div data-test="dialog-title">{{ title }}</div>
      <slot />
      <slot name="footer" />
      <button data-test="dialog-close" @click="$emit('close')">close</button>
    </div>
  `,
})

function findButtonByText(root: DOMWrapper<Element>, text: string): DOMWrapper<HTMLButtonElement> {
  const button = root.findAll<HTMLButtonElement>('button').find((candidate) => candidate.text().includes(text))
  if (!button) {
    throw new Error(`button not found: ${text}`)
  }
  return button
}

describe('admin ChannelsView', () => {
  beforeEach(() => {
    listChannels.mockReset()
    getAllGroups.mockReset()
    getWebSearchEmulationConfig.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    triggerDebouncedSearch.mockReset()
    clearDebouncedSearch.mockReset()

    listChannels.mockResolvedValue({
      items: [],
      total: 0,
    })
    getAllGroups.mockResolvedValue([])
    getWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    })
  })

  it('renders the channels page shell, toolbar, and empty state', async () => {
    const wrapper = mount(ChannelsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          EmptyState: EmptyStateStub,
          BaseDialog: BaseDialogStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: true,
          Icon: true,
          PlatformIcon: true,
          Toggle: true,
          PricingEntryCard: true,
        },
      },
    })

    await flushPromises()

    expect(listChannels).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="table-page-layout"]').exists()).toBe(true)
    expect(wrapper.get('input[placeholder="Search channels..."]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Create Channel')
    expect(wrapper.get('[data-test="columns"]').text()).toBe('name,description,status,group_count,pricing_count,created_at,actions')
    expect(wrapper.get('[data-test="empty-title"]').text()).toBe('No Channels Yet')
    expect(wrapper.get('[data-test="empty-description"]').text()).toBe('Create your first channel to manage model pricing')
  })

  it('opens and closes the create dialog from the toolbar', async () => {
    const wrapper = mount(ChannelsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          EmptyState: EmptyStateStub,
          BaseDialog: BaseDialogStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: true,
          Icon: true,
          PlatformIcon: true,
          Toggle: true,
          PricingEntryCard: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-test="base-dialog"]').exists()).toBe(false)

    const toolbar = wrapper.get('[data-test="toolbar"]')
    await findButtonByText(toolbar, 'Create Channel').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="dialog-title"]').text()).toBe('Create Channel')

    await wrapper.get('[data-test="dialog-close"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="base-dialog"]').exists()).toBe(false)
  })

  it('uses the shared admin toolbar and dialog tab styling hooks', async () => {
    const wrapper = mount(ChannelsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          EmptyState: EmptyStateStub,
          BaseDialog: BaseDialogStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: true,
          Icon: true,
          PlatformIcon: true,
          Toggle: true,
          PricingEntryCard: true,
        },
      },
    })

    await flushPromises()

    const toolbar = wrapper.get('[data-test="toolbar"]')
    expect(toolbar.classes()).toContain('admin-toolbar-surface')
    expect(toolbar.find('.admin-toolbar').exists()).toBe(true)

    await findButtonByText(toolbar, 'Create Channel').trigger('click')
    await flushPromises()

    expect(wrapper.find('.channel-tab').exists()).toBe(true)
  })

  it('keeps channel dialog choice lists on the shared admin surface system', () => {
    const source = readFileSync(resolve(currentDir, '../ChannelsView.vue'), 'utf8')

    expect(source).toContain('admin-form-section')
    expect(source).toContain('admin-choice-card')
    expect(source).toContain('admin-list-row')

    expect(source).not.toContain('border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700')
    expect(source).not.toContain('rounded-lg border border-gray-200 bg-gray-50')
    expect(source).not.toContain('bg-white shadow-lg')
  })
})
