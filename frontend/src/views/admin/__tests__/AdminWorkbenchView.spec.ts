import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdminWorkbenchView from '../AdminWorkbenchView.vue'
import type { GeneratedRedeemCode } from '@/api/redeem'

const {
  authState,
  getGenerated,
  generateBalanceTransferCodes,
  deleteGenerated,
  deleteGeneratedBatch,
  getCommissionSettings,
  updateCommissionSettings,
  listCommissionGrants,
  replaceCommissionGrants,
  getWorkbenchCommissionCalendar,
  getWorkbenchCommissionDayGroups,
  getWorkbenchCommissionDayGroupLogs,
  getWorkbenchCommissionGrants,
  listUsers,
  getAllGroups,
  refreshUser,
  showError,
  showSuccess,
  clipboardWriteText
} = vi.hoisted(() => ({
  authState: {
    user: {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    } as Record<string, unknown>
  },
  getGenerated: vi.fn(),
  generateBalanceTransferCodes: vi.fn(),
  deleteGenerated: vi.fn(),
  deleteGeneratedBatch: vi.fn(),
  getCommissionSettings: vi.fn(),
  updateCommissionSettings: vi.fn(),
  listCommissionGrants: vi.fn(),
  replaceCommissionGrants: vi.fn(),
  getWorkbenchCommissionCalendar: vi.fn(),
  getWorkbenchCommissionDayGroups: vi.fn(),
  getWorkbenchCommissionDayGroupLogs: vi.fn(),
  getWorkbenchCommissionGrants: vi.fn(),
  listUsers: vi.fn(),
  getAllGroups: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  clipboardWriteText: vi.fn()
}))

vi.mock('@/api', () => ({
  redeemAPI: {
    getGenerated,
    generateBalanceTransferCodes,
    deleteGenerated,
    deleteGeneratedBatch
  }
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subAdminCommission: {
      getSettings: getCommissionSettings,
      updateSettings: updateCommissionSettings,
      listGrants: listCommissionGrants,
      replaceGrants: replaceCommissionGrants,
      getWorkbenchCalendar: getWorkbenchCommissionCalendar,
      getWorkbenchDayGroups: getWorkbenchCommissionDayGroups,
      getWorkbenchDayGroupLogs: getWorkbenchCommissionDayGroupLogs,
      getWorkbenchGrants: getWorkbenchCommissionGrants
    },
    users: {
      list: listUsers
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get user() {
      return authState.user
    },
    get canAccessAdminWorkbench() {
      return authState.user?.role === 'admin' || authState.user?.role === 'sub_admin'
    },
    refreshUser
  })
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
      t: (key: string) => key
    })
  }
})

const paginated = <T,>(items: T[], total = items.length, page = 1, pageSize = 10) => ({
  items,
  total,
  page,
  page_size: pageSize,
  pages: Math.max(1, Math.ceil(total / pageSize))
})

const mountWorkbench = () =>
  mount(AdminWorkbenchView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true
      }
    }
  })

const defaultWindowInnerWidth = window.innerWidth
let getBoundingClientRectSpy: ReturnType<typeof vi.spyOn> | undefined
let confirmSpy: ReturnType<typeof vi.spyOn> | undefined

function mockRect(height: number): DOMRect {
  return {
    x: 0,
    y: 0,
    width: 420,
    height,
    top: 0,
    left: 0,
    right: 420,
    bottom: height,
    toJSON: () => ({})
  } as DOMRect
}

describe('AdminWorkbenchView balance transfer codes', () => {
  beforeEach(() => {
    authState.user = {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    }
    getGenerated.mockReset()
    generateBalanceTransferCodes.mockReset()
    deleteGenerated.mockReset()
    deleteGeneratedBatch.mockReset()
    getCommissionSettings.mockReset()
    updateCommissionSettings.mockReset()
    listCommissionGrants.mockReset()
    replaceCommissionGrants.mockReset()
    getWorkbenchCommissionCalendar.mockReset()
    getWorkbenchCommissionDayGroups.mockReset()
    getWorkbenchCommissionDayGroupLogs.mockReset()
    getWorkbenchCommissionGrants.mockReset()
    listUsers.mockReset()
    getAllGroups.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    clipboardWriteText.mockReset()
    getGenerated.mockResolvedValue(paginated<GeneratedRedeemCode>([]))
    getCommissionSettings.mockResolvedValue({ commission_rate: 0.12 })
    updateCommissionSettings.mockResolvedValue({ commission_rate: 0.12 })
    listCommissionGrants.mockResolvedValue([
      {
        id: 1,
        sub_admin_id: 8,
        sub_admin_email: 'manager@example.com',
        group_id: 3,
        group_name: 'Claude 特价',
        granted_date: '2026-08-01',
        enabled: true,
        created_at: '2026-08-01T00:00:00Z',
        updated_at: '2026-08-01T00:00:00Z'
      }
    ])
    replaceCommissionGrants.mockResolvedValue([])
    getWorkbenchCommissionGrants.mockResolvedValue([])
    getWorkbenchCommissionCalendar.mockResolvedValue([
      {
        date: '2026-08-22',
        enabled: true,
        actual_cost: 12,
        commission_amount: 1.44
      }
    ])
    getWorkbenchCommissionDayGroups.mockResolvedValue([
      {
        group_id: 3,
        group_name: 'Claude 特价',
        requests: 2,
        total_tokens: 1500,
        actual_cost: 12,
        commission_amount: 1.44
      }
    ])
    getWorkbenchCommissionDayGroupLogs.mockResolvedValue(
      paginated(
        [
          {
            id: 99,
            request_id: 'req-1',
            created_at: '2026-08-22T08:00:00Z',
            user_id: 41,
            user_email: 'customer@example.com',
            api_key_id: 71,
            api_key_name: 'Main key',
            group_id: 3,
            group_name: 'Claude 特价',
            model: 'claude-sonnet-4',
            requested_model: 'claude-sonnet-4',
            input_tokens: 1000,
            output_tokens: 500,
            cache_creation_tokens: 0,
            cache_read_tokens: 0,
            actual_cost: 12,
            total_tokens: 1500
          }
        ],
        1,
        1,
        10
      )
    )
    listUsers.mockResolvedValue(
      paginated([
        {
          id: 8,
          email: 'manager@example.com',
          username: 'manager',
          role: 'sub_admin',
          balance: 0,
          frozen_balance: 0,
          concurrency: 5,
          rpm_limit: 0,
          status: 'active',
          allowed_groups: null,
          created_at: '2026-08-01T00:00:00Z',
          updated_at: '2026-08-01T00:00:00Z'
        }
      ])
    )
    getAllGroups.mockResolvedValue([
      { id: 3, name: 'Claude 特价', platform: 'anthropic', status: 'active' },
      { id: 4, name: 'Gemini 池', platform: 'gemini', status: 'active' }
    ])
    refreshUser.mockResolvedValue(undefined)
    clipboardWriteText.mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: clipboardWriteText },
      configurable: true
    })
  })

  afterEach(() => {
    getBoundingClientRectSpy?.mockRestore()
    getBoundingClientRectSpy = undefined
    confirmSpy?.mockRestore()
    confirmSpy = undefined
    Object.defineProperty(window, 'innerWidth', {
      value: defaultWindowInnerWidth,
      configurable: true
    })
  })

  it('loads generated codes for the workbench owner', async () => {
    getGenerated.mockResolvedValueOnce(
      paginated<GeneratedRedeemCode>([
        {
          id: 88,
          code: 'WORKBENCH-CODE',
          type: 'balance',
          value: 8,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-08-20T12:00:00Z',
          created_by: 7,
          source: 'user_balance_transfer',
          notes: 'customer campaign A'
        }
      ])
    )

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getGenerated).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('WORKBENCH-CODE')
    expect(wrapper.text()).toContain('customer campaign A')
  })

  it('generates balance redeem codes and refreshes account state', async () => {
    generateBalanceTransferCodes.mockResolvedValue([
      {
        id: 89,
        code: 'NEW-CODE',
        type: 'balance',
        value: 5,
        status: 'unused',
        used_by: null,
        used_at: null,
        created_at: '2026-08-20T12:00:00Z',
        created_by: 7,
        source: 'user_balance_transfer',
        notes: 'new batch note'
      }
    ])

    const wrapper = mountWorkbench()
    await flushPromises()

    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('5')
    await wrapper.get('[data-test="workbench-transfer-count"]').setValue('2')
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()

    expect(generateBalanceTransferCodes).toHaveBeenCalledWith({
      amount: 5,
      count: 2,
      expires_in_days: 30,
      notes: '',
      single_use_per_user: false
    })
    expect(refreshUser).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.generated')
    expect(wrapper.text()).toContain('NEW-CODE')
    expect(wrapper.text()).toContain('new batch note')
  })

  it('keeps the latest generated batch in a fixed-height scroll panel', async () => {
    Object.defineProperty(window, 'innerWidth', {
      value: 1280,
      configurable: true
    })
    getBoundingClientRectSpy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function () {
      if ((this as HTMLElement).dataset.test === 'workbench-transfer-form') {
        return mockRect(486)
      }
      return mockRect(0)
    })

    const generatedBatch = Array.from({ length: 25 }, (_, index) => ({
      id: 900 + index,
      code: `BATCH-CODE-${String(index + 1).padStart(2, '0')}`,
      type: 'balance',
      value: 1,
      status: 'unused',
      used_by: null,
      used_at: null,
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7,
      source: 'user_balance_transfer'
    })) as GeneratedRedeemCode[]
    generateBalanceTransferCodes.mockResolvedValue(generatedBatch)

    const wrapper = mountWorkbench()
    await flushPromises()

    await wrapper.get('[data-test="workbench-transfer-amount"]').setValue('1')
    await wrapper.get('[data-test="workbench-transfer-count"]').setValue('25')
    await wrapper.get('[data-test="workbench-transfer-form"]').trigger('submit')
    await flushPromises()

    const resultCard = wrapper.get('[data-test="workbench-generated-now-card"]')
    const resultList = wrapper.get('[data-test="workbench-generated-results"]')
    expect(wrapper.findAll('[data-test="workbench-generated-code"]')).toHaveLength(25)
    expect(resultCard.classes()).toContain('overflow-hidden')
    expect(resultCard.attributes('style')).toContain('height: 486px')
    expect(resultList.classes()).toEqual(expect.arrayContaining(['min-h-0', 'flex-1', 'overflow-y-auto']))
    expect(wrapper.text()).toContain('BATCH-CODE-25')
  })

  it('batch deletes selected deletable generated codes', async () => {
    const unusedCode: GeneratedRedeemCode = {
      id: 101,
      code: 'UNUSED-CODE',
      type: 'balance',
      value: 3,
      status: 'unused',
      used_by: null,
      used_at: null,
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7,
      source: 'user_balance_transfer'
    }
    const usedCode: GeneratedRedeemCode = {
      id: 102,
      code: 'USED-CODE',
      type: 'balance',
      value: 4,
      status: 'used',
      used_by: 22,
      used_at: '2026-08-20T13:00:00Z',
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7,
      source: 'user_balance_transfer'
    }
    getGenerated.mockResolvedValueOnce(paginated<GeneratedRedeemCode>([unusedCode, usedCode]))
    deleteGeneratedBatch.mockResolvedValueOnce([unusedCode])
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountWorkbench()
    await flushPromises()

    const batchDeleteButton = wrapper.get('[data-test="delete-selected-generated-codes"]')
    expect(batchDeleteButton.attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="select-generated-code-101"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="select-generated-code-102"]').exists()).toBe(false)

    await wrapper.get('[data-test="select-generated-code-101"]').setValue(true)
    expect(wrapper.text()).toContain('adminWorkbench.balanceTransfer.selectedCount')
    expect(batchDeleteButton.attributes('disabled')).toBeUndefined()

    await batchDeleteButton.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.batchDeleteConfirm')
    expect(deleteGeneratedBatch).toHaveBeenCalledWith([101])
    expect(refreshUser).toHaveBeenCalled()
    expect(getGenerated).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.batchDeleted')
  })

  it('renders commission management for full admins and saves assigned groups', async () => {
    authState.user = {
      id: 1,
      email: 'admin@example.com',
      role: 'admin',
      balance: 100,
      concurrency: 5
    }

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getCommissionSettings).toHaveBeenCalled()
    expect(listUsers).toHaveBeenCalledWith(1, 1000, { role: 'sub_admin' })
    expect(getAllGroups).toHaveBeenCalled()
    expect(listCommissionGrants).toHaveBeenCalled()
    expect(wrapper.text()).toContain('manager@example.com')
    expect(wrapper.text()).toContain('Claude 特价')

    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    await flushPromises()

    expect(replaceCommissionGrants).toHaveBeenCalledWith(8, { group_ids: [3] })
  })

  it('loads every page of secondary admins for commission management', async () => {
    authState.user = {
      id: 1,
      email: 'admin@example.com',
      role: 'admin',
      balance: 100,
      concurrency: 5
    }
    listUsers
      .mockResolvedValueOnce(
        paginated(
          [
            {
              id: 8,
              email: 'manager@example.com',
              username: 'manager',
              role: 'sub_admin',
              balance: 0,
              frozen_balance: 0,
              concurrency: 5,
              rpm_limit: 0,
              status: 'active',
              allowed_groups: null,
              created_at: '2026-08-01T00:00:00Z',
              updated_at: '2026-08-01T00:00:00Z'
            }
          ],
          1001,
          1,
          1000
        )
      )
      .mockResolvedValueOnce(
        paginated(
          [
            {
              id: 9,
              email: 'later-manager@example.com',
              username: 'later-manager',
              role: 'sub_admin',
              balance: 0,
              frozen_balance: 0,
              concurrency: 5,
              rpm_limit: 0,
              status: 'active',
              allowed_groups: null,
              created_at: '2026-08-01T00:00:00Z',
              updated_at: '2026-08-01T00:00:00Z'
            }
          ],
          1001,
          2,
          1000
        )
      )

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(listUsers).toHaveBeenNthCalledWith(1, 1, 1000, { role: 'sub_admin' })
    expect(listUsers).toHaveBeenNthCalledWith(2, 2, 1000, { role: 'sub_admin' })
    expect(wrapper.text()).toContain('later-manager@example.com')
  })

  it('renders commission calendar for sub-admins and loads day details', async () => {
    authState.user = {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    }

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getWorkbenchCommissionCalendar).toHaveBeenCalled()
    expect(wrapper.text()).toContain('2026-08-22')
    expect(wrapper.text()).toContain('$12.00')

    await wrapper.get('[data-test="commission-calendar-day-2026-08-22"]').trigger('click')
    await flushPromises()

    expect(getWorkbenchCommissionDayGroups).toHaveBeenCalledWith('2026-08-22')
    expect(wrapper.text()).toContain('Claude 特价')

    await wrapper.get('[data-test="commission-day-group-3-toggle"]').trigger('click')
    await flushPromises()

    expect(getWorkbenchCommissionDayGroupLogs).toHaveBeenCalledWith('2026-08-22', 3, {
      page: 1,
      page_size: 10
    })
    expect(wrapper.text()).toContain('req-1')
  })
})
