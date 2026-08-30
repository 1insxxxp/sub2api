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

const mountedWorkbenchWrappers: Array<ReturnType<typeof mount>> = []

const mountWorkbench = () => {
  const wrapper = mount(AdminWorkbenchView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
        AdminAffiliateLeaderboardPanel: {
          template: '<div data-test="affiliate-leaderboard-panel-stub" />'
        }
      }
    }
  })
  mountedWorkbenchWrappers.push(wrapper)
  return wrapper
}

const openCommissionTab = async (wrapper: ReturnType<typeof mountWorkbench>) => {
  await wrapper.get('[data-test="workbench-tab-commission"]').trigger('click')
  await flushPromises()
}

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
    for (const wrapper of mountedWorkbenchWrappers.splice(0)) {
      wrapper.unmount()
    }
    vi.useRealTimers()
    getBoundingClientRectSpy?.mockRestore()
    getBoundingClientRectSpy = undefined
    confirmSpy?.mockRestore()
    confirmSpy = undefined
    Object.defineProperty(window, 'innerWidth', {
      value: defaultWindowInnerWidth,
      configurable: true
    })
  })

  it('separates workbench functions into tabs and mounts only the active panel', async () => {
    const wrapper = mountWorkbench()
    await flushPromises()

    const balanceTab = wrapper.get('[data-test="workbench-tab-balance-transfer"]')
    const commissionTab = wrapper.get('[data-test="workbench-tab-commission"]')
    const leaderboardTab = wrapper.get('[data-test="workbench-tab-affiliate-leaderboard"]')

    expect(wrapper.get('[data-test="admin-workbench-tabs"]').attributes('role')).toBe('tablist')
    expect(balanceTab.attributes('aria-selected')).toBe('true')
    expect(commissionTab.attributes('aria-selected')).toBe('false')
    expect(leaderboardTab.attributes('aria-selected')).toBe('false')
    expect(wrapper.find('[data-test="workbench-balance-transfer-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="sub-admin-commission-panel"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-leaderboard-panel-stub"]').exists()).toBe(false)
    expect(getWorkbenchCommissionCalendar).not.toHaveBeenCalled()

    await leaderboardTab.trigger('click')
    await flushPromises()

    expect(balanceTab.attributes('aria-selected')).toBe('false')
    expect(commissionTab.attributes('aria-selected')).toBe('false')
    expect(leaderboardTab.attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-test="workbench-balance-transfer-panel"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="sub-admin-commission-panel"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="affiliate-leaderboard-panel-stub"]').exists()).toBe(true)
    expect(getWorkbenchCommissionCalendar).not.toHaveBeenCalled()

    await commissionTab.trigger('click')
    await flushPromises()

    expect(balanceTab.attributes('aria-selected')).toBe('false')
    expect(commissionTab.attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-test="workbench-balance-transfer-panel"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="sub-admin-commission-panel"]').exists()).toBe(true)
    expect(getWorkbenchCommissionCalendar).toHaveBeenCalled()
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
        },
        {
          id: 89,
          code: 'WORKBENCH-USED-CODE',
          type: 'balance',
          value: 9,
          status: 'used',
          used_by: 22,
          used_at: '2026-08-20T13:00:00Z',
          created_at: '2026-08-20T12:00:00Z',
          created_by: 7,
          source: 'user_balance_transfer'
        }
      ])
    )

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(getGenerated).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('WORKBENCH-CODE')
    expect(wrapper.text()).toContain('customer campaign A')
    expect(wrapper.text()).toContain('adminWorkbench.balanceTransfer.status.unused')
    expect(wrapper.text()).toContain('adminWorkbench.balanceTransfer.status.used')
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

  it('batch deletes selected generated codes including used codes', async () => {
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
    deleteGeneratedBatch.mockResolvedValueOnce([unusedCode, usedCode])
    confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountWorkbench()
    await flushPromises()

    const batchDeleteButton = wrapper.get('[data-test="delete-selected-generated-codes"]')
    expect(batchDeleteButton.attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="select-generated-code-101"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="select-generated-code-102"]').exists()).toBe(true)

    await wrapper.get('[data-test="select-generated-code-101"]').setValue(true)
    await wrapper.get('[data-test="select-generated-code-102"]').setValue(true)
    expect(wrapper.text()).toContain('adminWorkbench.balanceTransfer.selectedCount')
    expect(batchDeleteButton.attributes('disabled')).toBeUndefined()

    await batchDeleteButton.trigger('click')
    await flushPromises()

    expect(confirmSpy).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.batchDeleteConfirm')
    expect(deleteGeneratedBatch).toHaveBeenCalledWith([101, 102])
    expect(refreshUser).toHaveBeenCalled()
    expect(getGenerated).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledWith('adminWorkbench.balanceTransfer.batchDeleted')
  })

  it('shows generated-code selection when workbench list omits the source field', async () => {
    const codeWithoutSource = {
      id: 103,
      code: 'LEGACY-CODE',
      type: 'balance',
      value: 5,
      status: 'unused',
      used_by: null,
      used_at: null,
      created_at: '2026-08-20T12:00:00Z',
      created_by: 7
    } as GeneratedRedeemCode
    getGenerated.mockResolvedValueOnce(paginated<GeneratedRedeemCode>([codeWithoutSource]))

    const wrapper = mountWorkbench()
    await flushPromises()

    expect(wrapper.find('[data-test="select-generated-code-103"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="delete-selected-generated-codes"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="select-generated-code-103"]').setValue(true)

    expect(wrapper.get('[data-test="delete-selected-generated-codes"]').attributes('disabled')).toBeUndefined()
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
    await openCommissionTab(wrapper)

    expect(getCommissionSettings).toHaveBeenCalled()
    expect(getAllGroups).toHaveBeenCalled()
    expect(listCommissionGrants).toHaveBeenCalled()
    expect(wrapper.text()).toContain('adminWorkbench.commission.sharedGrantsHint')
    expect(wrapper.text()).toContain('Claude 特价')

    await wrapper.get('[data-test="sub-admin-commission-save-grants"]').trigger('click')
    await flushPromises()

    expect(replaceCommissionGrants).toHaveBeenCalledWith({ group_ids: [3] })
  })

  it('uses the local calendar month when loading commission data by default', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 8, 1, 0, 30, 0))
    authState.user = {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    }

    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)

    expect(getWorkbenchCommissionCalendar).toHaveBeenCalledWith({ month: '2026-09' })
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
    await openCommissionTab(wrapper)

    expect(getWorkbenchCommissionCalendar).toHaveBeenCalled()
    expect(wrapper.text()).toContain('$12.00')

    await wrapper.get('[data-test="commission-calendar-day-2026-08-22"]').trigger('click')
    await flushPromises()

    expect(getWorkbenchCommissionDayGroups).toHaveBeenCalledWith('2026-08-22')
    expect(wrapper.get('[data-test="commission-day-dialog"]').attributes('role')).toBe('dialog')
    expect(wrapper.get('[data-test="commission-day-dialog"]').attributes('aria-modal')).toBe('true')
    expect(wrapper.text()).toContain('Claude 特价')

    await wrapper.get('[data-test="commission-day-group-3-toggle"]').trigger('click')
    await flushPromises()

    expect(getWorkbenchCommissionDayGroupLogs).toHaveBeenCalledWith('2026-08-22', 3, {
      page: 1,
      page_size: 10
    })
    expect(wrapper.text()).toContain('req-1')
  })

  it('shows daily spend and commission inside compact mobile calendar cells', async () => {
    Object.defineProperty(window, 'innerWidth', {
      value: 390,
      configurable: true
    })
    getWorkbenchCommissionCalendar.mockResolvedValueOnce([
      {
        date: '2026-08-22',
        enabled: true,
        actual_cost: 3828.41,
        commission_amount: 459.4092
      },
      {
        date: '2026-08-23',
        enabled: true,
        actual_cost: 12.34,
        commission_amount: 1.23
      }
    ])

    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)

    const dayCell = wrapper.get('[data-test="commission-calendar-day-2026-08-22"]')
    const monthSummary = wrapper.get('[data-test="commission-calendar-month-summary"]')
    const amounts = wrapper.get('[data-test="commission-calendar-day-2026-08-22-amounts"]')
    const calendarGrid = wrapper.get('[data-test="commission-calendar-grid"]')

    expect(dayCell.classes()).toEqual(expect.arrayContaining(['commission-calendar-day-cell', 'min-h-12']))
    expect(calendarGrid.classes()).toEqual(expect.arrayContaining(['grid-cols-1', 'min-[360px]:grid-cols-2', 'sm:grid-cols-7']))
    expect(dayCell.text()).toContain('22')
    expect(amounts.text()).toContain('$3.83K')
    expect(amounts.text()).toContain('$459.41')
    expect(amounts.text()).toContain('↓')
    expect(amounts.text()).toContain('↑')
    expect(amounts.text()).not.toContain('actualCostShort')
    expect(amounts.text()).not.toContain('commissionAmountShort')
    expect(monthSummary.classes()).toEqual(expect.arrayContaining(['grid-cols-1', 'min-[480px]:grid-cols-2']))
    expect(monthSummary.text()).toContain('$3840.75')
    expect(monthSummary.text()).toContain('$460.64')
    expect(dayCell.attributes('aria-label')).toContain('$3828.41')
    expect(dayCell.attributes('aria-label')).toContain('$459.41')
    expect(dayCell.attributes('aria-current')).toBeUndefined()

    await dayCell.trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-test="commission-day-dialog"]')
    expect(dayCell.attributes('aria-pressed')).toBe('true')
    expect(dialog.text()).toContain('2026-08-22')
    expect(dialog.text()).toContain('$3828.41')
    expect(dialog.text()).toContain('$459.41')
  })

  it('opens and closes the commission day dialog on mobile', async () => {
    Object.defineProperty(window, 'innerWidth', {
      value: 390,
      configurable: true
    })
    getWorkbenchCommissionCalendar.mockResolvedValueOnce([
      {
        date: '2026-08-22',
        enabled: true,
        actual_cost: 3828.41,
        commission_amount: 459.4092
      }
    ])

    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)

    await wrapper.get('[data-test="commission-calendar-day-2026-08-22"]').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-test="commission-day-dialog"]')
    expect(dialog.classes()).toEqual(expect.arrayContaining(['items-end', 'sm:items-center']))
    expect(dialog.get('[data-test="commission-day-dialog-close"]').attributes('aria-label')).toBe('common.close')

    await dialog.get('[data-test="commission-day-dialog-close"]').trigger('click')
    expect(wrapper.find('[data-test="commission-day-dialog"]').exists()).toBe(false)
  })

  it('closes the commission day dialog with Escape and restores body scrolling', async () => {
    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)
    await wrapper.get('[data-test="commission-calendar-day-2026-08-22"]').trigger('click')
    await flushPromises()

    expect(document.body.style.overflow).toBe('hidden')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()

    expect(wrapper.find('[data-test="commission-day-dialog"]').exists()).toBe(false)
    expect(document.body.style.overflow).toBe('')
  })

  it('uses a responsive dialog and stable day-detail card regions', async () => {
    authState.user = {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    }

    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)

    const layout = wrapper.get('[data-test="commission-calendar-layout"]')
    expect(layout.classes()).toContain('commission-calendar-layout')

    await wrapper.get('[data-test="commission-calendar-day-2026-08-22"]').trigger('click')
    await flushPromises()

    expect(layout.classes()).toContain('commission-calendar-layout')
    expect(layout.classes()).not.toContain('xl:grid-cols-[minmax(0,3fr)_minmax(22rem,2fr)]')
    expect(wrapper.get('[data-test="commission-day-dialog"] > section').classes()).toContain('max-w-3xl')
    const groupCard = wrapper.get('[data-test="commission-day-group-3"]')
    expect(groupCard.classes()).toContain('commission-day-group-card')
    expect(groupCard.get('[data-test="commission-day-group-3-name"]').text()).toContain('Claude 特价')
    expect(groupCard.get('[data-test="commission-day-group-3-metrics"]').text()).toContain('1500 tokens')
    expect(groupCard.get('[data-test="commission-day-group-3-amounts"]').text()).toContain('$12.00')
    expect(groupCard.get('[data-test="commission-day-group-3-amounts"]').text()).toContain('$1.44')
    expect(groupCard.get('[data-test="commission-day-group-3-amounts"]').classes()).toEqual(
      expect.arrayContaining(['grid-cols-1', 'min-[480px]:grid-cols-2'])
    )
    expect(groupCard.get('[data-test="commission-day-group-3-toggle"]').classes()).toContain('w-full')
  })

  it('keeps commission day request logs readable on mobile', async () => {
    Object.defineProperty(window, 'innerWidth', {
      value: 390,
      configurable: true
    })
    authState.user = {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    }

    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)
    await wrapper.get('[data-test="commission-calendar-day-2026-08-22"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="commission-day-group-3-toggle"]').trigger('click')
    await flushPromises()

    const logCard = wrapper.get('[data-test="commission-log-99"]')
    const requestID = wrapper.get('[data-test="commission-log-request-99"]')
    const userLine = wrapper.get('[data-test="commission-log-user-99"]')
    const modelLine = wrapper.get('[data-test="commission-log-model-99"]')

    expect(logCard.classes()).toEqual(expect.arrayContaining(['rounded-lg', 'p-3']))
    expect(requestID.classes()).toEqual(expect.arrayContaining(['break-all', 'sm:truncate']))
    expect(userLine.classes()).toEqual(expect.arrayContaining(['break-all', 'sm:truncate']))
    expect(modelLine.classes()).toEqual(expect.arrayContaining(['break-all', 'sm:truncate']))
    expect(logCard.text()).toContain('customer@example.com')
    expect(logCard.text()).toContain('claude-sonnet-4')
    expect(logCard.text()).toContain('$12.00')
  })

  it('omits the commission explainer copy for sub-admins', async () => {
    authState.user = {
      id: 7,
      email: 'sub-admin@example.com',
      role: 'sub_admin',
      balance: 30,
      concurrency: 5
    }

    const wrapper = mountWorkbench()
    await openCommissionTab(wrapper)

    expect(wrapper.find('[data-test="sub-admin-commission-panel"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('adminWorkbench.commission.title')
    expect(wrapper.text()).not.toContain('adminWorkbench.commission.subtitle')
  })
})
