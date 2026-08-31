import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'
import DataTable from '@/components/common/DataTable.vue'

const { listRedeemCodes, generateRedeemCodes, batchUpdateRedeemCodes, getAllGroups, showSuccess, showError, showInfo } =
  vi.hoisted(() => ({
    listRedeemCodes: vi.fn(),
    generateRedeemCodes: vi.fn(),
    batchUpdateRedeemCodes: vi.fn(),
    getAllGroups: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn(),
    showInfo: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    redeem: {
      list: listRedeemCodes,
      generate: generateRedeemCodes,
      delete: vi.fn(),
      batchDelete: vi.fn(),
      batchUpdate: batchUpdateRedeemCodes,
      exportCodes: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    showInfo
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
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

const DataTableStub = {
  props: ['columns', 'data', 'selectable', 'selectedKeys', 'rowKey'],
  emits: ['update:selectedKeys'],
  methods: {
    rowId(row: Record<string, unknown>) {
      return this.rowKey ? row[this.rowKey] : row.id
    },
    updateRow(row: Record<string, unknown>, event: Event) {
      const id = this.rowId(row)
      const selected = new Set(this.selectedKeys ?? [])
      if ((event.target as HTMLInputElement).checked) {
        selected.add(id)
      } else {
        selected.delete(id)
      }
      this.$emit('update:selectedKeys', Array.from(selected))
    }
  },
  template: `
    <table>
      <thead>
        <tr>
          <th v-if="selectable"><input data-test="select-all" type="checkbox" /></th>
          <th v-for="column in columns" :key="column.key">
            <slot :name="'header-' + column.key" :column="column">{{ column.label }}</slot>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in data" :key="row.id">
          <td v-if="selectable">
            <input
              data-test="select-row"
              type="checkbox"
              :checked="(selectedKeys || []).includes(rowId(row))"
              @change="updateRow(row, $event)"
            />
          </td>
          <td v-for="column in columns" :key="column.key">
            <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  `
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  setup(props: { options: Array<{ value: unknown; label: string }> }, { emit }: { emit: (event: string, ...args: unknown[]) => void }) {
    const onChange = (event: Event) => {
      const raw = (event.target as HTMLSelectElement).value
      const option = props.options.find((item) => String(item.value ?? '') === raw)
      const value = option ? option.value : raw
      emit('update:modelValue', value)
      emit('change', value, option ?? null)
    }
    return { onChange }
  },
  template: `
    <select v-bind="$attrs" :value="modelValue ?? ''" @change="onChange">
      <option v-for="option in options" :key="String(option.value ?? '')" :value="option.value ?? ''">
        {{ option.label }}
      </option>
    </select>
  `
}

function stubMobileMatchMedia() {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('min-width') ? false : true,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  })
}

describe('admin RedeemView batch update', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''

    listRedeemCodes.mockReset()
    generateRedeemCodes.mockReset()
    batchUpdateRedeemCodes.mockReset()
    getAllGroups.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    showInfo.mockReset()

    listRedeemCodes.mockResolvedValue({
      items: [
        {
          id: 1,
          code: 'CODE-1',
          type: 'balance',
          value: 10,
          threshold_exempt: true,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null,
          single_use_per_user: true,
          batch_id: 'batch-1'
        },
        {
          id: 2,
          code: 'CODE-2',
          type: 'balance',
          value: 20,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    batchUpdateRedeemCodes.mockResolvedValue({ updated: 1, message: 'ok' })
    generateRedeemCodes.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('submits only checked fields for selected redeem codes', async () => {
    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.findAll('[data-test="select-row"]')[0].setValue(true)
    await wrapper.get('[data-test="batch-update-open"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="batch-field-status"]').setValue(true)
    await wrapper.get('[data-test="batch-status-select"]').setValue('disabled')
    await wrapper.get('[data-test="batch-field-notes"]').setValue(true)
    await wrapper.get('[data-test="batch-notes-input"]').setValue('maintenance')
    await wrapper.get('[data-test="batch-update-form"]').trigger('submit')
    await flushPromises()

    expect(batchUpdateRedeemCodes).toHaveBeenCalledWith([1], {
      status: 'disabled',
      notes: 'maintenance'
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.redeem.batchUpdateSuccess')
  })

  it('generates a batch limited to one code per user and shows its badge', async () => {
    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    expect(wrapper.get('[data-test="single-use-badge"]').exists()).toBe(true)

    await wrapper.get('[data-test="generate-codes-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="single-use-per-user"]').setValue(true)
    await wrapper.get('[data-test="generate-codes-form"]').trigger('submit')
    await flushPromises()

    expect(generateRedeemCodes).toHaveBeenCalledWith(1, 'balance', 10, undefined, undefined, undefined, true, false)
  })

  it('shows gift credit only for balance codes, submits it, and resets it after generation', async () => {
    generateRedeemCodes.mockResolvedValueOnce([
      {
        id: 9,
        code: 'GIFT-CODE',
        type: 'balance',
        value: 10,
        threshold_exempt: true,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-01-01T00:00:00Z'
      }
    ])

    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    expect(wrapper.get('[data-test="gift-credit-badge"]').text()).toBe('admin.redeem.giftBadge')

    await wrapper.get('[data-test="generate-codes-open"]').trigger('click')
    await flushPromises()

    const giftOption = wrapper.get('[data-test="threshold-exempt-option"]')
    const giftToggle = wrapper.get<HTMLInputElement>('[data-test="threshold-exempt"]')
    expect(giftToggle.element.checked).toBe(false)
    expect(giftOption.classes()).toEqual(expect.arrayContaining(['w-full', 'min-w-0']))
    expect(giftOption.find('[data-test="threshold-exempt-copy"]').classes()).toContain('min-w-0')

    await giftToggle.setValue(true)
    await wrapper.get('[data-test="generate-codes-form"]').trigger('submit')
    await flushPromises()

    expect(generateRedeemCodes).toHaveBeenCalledWith(1, 'balance', 10, undefined, undefined, undefined, false, true)

    await wrapper.get('[data-test="generate-codes-open"]').trigger('click')
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('[data-test="threshold-exempt"]').element.checked).toBe(false)

    await wrapper.get('[data-test="generate-code-type"]').setValue('subscription')
    await flushPromises()
    expect(wrapper.find('[data-test="threshold-exempt"]').exists()).toBe(false)

    await wrapper.get('[data-test="generate-code-type"]').setValue('concurrency')
    await flushPromises()
    expect(wrapper.find('[data-test="threshold-exempt"]').exists()).toBe(false)
  })

  it('shows selection checkboxes in the mobile redeem-code card layout', async () => {
    stubMobileMatchMedia()
    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const rowCheckboxes = wrapper.findAll<HTMLInputElement>('[data-test="select-row"]')
    expect(rowCheckboxes).toHaveLength(2)

    await rowCheckboxes[0].setValue(true)

    expect(wrapper.text()).toContain('admin.redeem.selectedCount')
    expect(wrapper.get('[data-test="batch-update-open"]').attributes('disabled')).toBeUndefined()
  })

  it('hides and resets the single-use option for invitation codes', async () => {
    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('[data-test="generate-codes-open"]').trigger('click')
    await wrapper.get('[data-test="single-use-per-user"]').setValue(true)
    await wrapper.get('[data-test="generate-code-type"]').setValue('invitation')
    await flushPromises()

    expect(wrapper.find('[data-test="single-use-per-user"]').exists()).toBe(false)
    await wrapper.get('[data-test="generate-codes-form"]').trigger('submit')
    await flushPromises()
    expect(generateRedeemCodes).toHaveBeenCalledWith(1, 'invitation', 0, undefined, undefined, undefined, false, false)
  })
})
