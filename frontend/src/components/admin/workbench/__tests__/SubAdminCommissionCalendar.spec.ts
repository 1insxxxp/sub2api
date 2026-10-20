import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import type { SubAdminCommissionCalendarDay, SubAdminCommissionMonthlyReport } from '@/api/admin/subAdminCommission'
import SubAdminCommissionCalendar from '../SubAdminCommissionCalendar.vue'

const { getWorkbenchMonthlyReport } = vi.hoisted(() => ({
  getWorkbenchMonthlyReport: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subAdminCommission: {
      getWorkbenchMonthlyReport,
    },
  },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const TrendChartStub = defineComponent({
  name: 'TrendChartStub',
  props: {
    days: {
      type: Array as PropType<SubAdminCommissionCalendarDay[]>,
      required: true,
    },
    metric: { type: String, default: 'commission' },
    loading: {
      type: Boolean,
      default: false,
    },
  },
  template: '<div data-test="trend-chart-stub" />',
})

const mountedWrappers: Array<ReturnType<typeof mount>> = []

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

afterEach(() => {
  mountedWrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  vi.useRealTimers()
  getWorkbenchMonthlyReport.mockReset()
})

describe('SubAdminCommissionCalendar stale month data', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-14T12:00:00'))
  })

  it('refreshes the selected month when management settings change', async () => {
    getWorkbenchMonthlyReport.mockResolvedValue({ days: [], groups: [] })
    const wrapper = mount(SubAdminCommissionCalendar, { props: { refreshKey: 0 }, global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    await wrapper.get('input[type="month"]').setValue('2026-08')
    await flushPromises()
    expect(getWorkbenchMonthlyReport).toHaveBeenCalledTimes(2)
    await wrapper.setProps({ refreshKey: 1 })
    await flushPromises()
    expect(getWorkbenchMonthlyReport).toHaveBeenCalledTimes(3)
    expect(getWorkbenchMonthlyReport).toHaveBeenLastCalledWith({ month: '2026-08' })
  })

  it('clears the previous month data when the next month request fails', async () => {
    const previousMonthDays: SubAdminCommissionCalendarDay[] = [
      {
        date: '2026-09-03',
        enabled: true,
        actual_cost: 12,
        commission_amount: 3.45,
      },
    ]
    getWorkbenchMonthlyReport
      .mockResolvedValueOnce({ days: previousMonthDays, groups: [] })
      .mockRejectedValueOnce(new Error('calendar unavailable'))

    const wrapper = mount(SubAdminCommissionCalendar, {
      global: {
        stubs: {
          SubAdminCommissionTrendChart: TrendChartStub,
        },
      },
    })
    mountedWrappers.push(wrapper)

    await flushPromises()

    const monthInput = wrapper.get('input[type="month"]')
    const chart = wrapper.findComponent({ name: 'TrendChartStub' })
    expect(getWorkbenchMonthlyReport).toHaveBeenNthCalledWith(1, { month: '2026-09' })
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$12.00')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$3.45')
    expect(chart.props('days')).toEqual(previousMonthDays)

    await monthInput.setValue('2026-08')
    await flushPromises()

    expect(getWorkbenchMonthlyReport).toHaveBeenNthCalledWith(2, { month: '2026-08' })
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$12.00')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$3.45')
    expect(chart.props('days')).toEqual([])
  })

  it('ignores a late response from an older month request', async () => {
    const initial = deferred<SubAdminCommissionMonthlyReport>()
    const stale = deferred<SubAdminCommissionMonthlyReport>()
    const latest = deferred<SubAdminCommissionMonthlyReport>()
    const latestMonthDays: SubAdminCommissionCalendarDay[] = [
      {
        date: '2026-07-03',
        enabled: true,
        actual_cost: 30,
        commission_amount: 6,
      },
    ]
    const staleMonthDays: SubAdminCommissionCalendarDay[] = [
      {
        date: '2026-08-03',
        enabled: true,
        actual_cost: 80,
        commission_amount: 16,
      },
    ]
    getWorkbenchMonthlyReport
      .mockReturnValueOnce(initial.promise)
      .mockReturnValueOnce(stale.promise)
      .mockReturnValueOnce(latest.promise)

    const wrapper = mount(SubAdminCommissionCalendar, {
      global: {
        stubs: {
          SubAdminCommissionTrendChart: TrendChartStub,
        },
      },
    })
    mountedWrappers.push(wrapper)

    await flushPromises()
    initial.resolve({ days: [], groups: [] })
    await flushPromises()

    const monthInput = wrapper.get('input[type="month"]')
    await monthInput.setValue('2026-08')
    await monthInput.setValue('2026-07')
    expect(getWorkbenchMonthlyReport).toHaveBeenNthCalledWith(2, { month: '2026-08' })
    expect(getWorkbenchMonthlyReport).toHaveBeenNthCalledWith(3, { month: '2026-07' })

    latest.resolve({ days: latestMonthDays, groups: [{ group_id: 1, group_name: 'Latest group', days: latestMonthDays }] })
    await flushPromises()
    const chart = wrapper.findComponent({ name: 'TrendChartStub' })
    expect(chart.props('days')).toEqual(latestMonthDays)

    stale.resolve({ days: staleMonthDays, groups: [{ group_id: 99, group_name: 'Stale group', days: staleMonthDays }] })
    await flushPromises()
    expect(chart.props('days')).toEqual(latestMonthDays)
    expect(wrapper.find('[data-test="commission-group-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="commission-group-99"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$30.00')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$80.00')
  })
})


describe('SubAdminCommissionCalendar group scope', () => {
  const day = (actual: number, commission: number): SubAdminCommissionCalendarDay => ({
    date: '2026-09-03', enabled: true, actual_cost: actual, commission_amount: commission
  })
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-14T12:00:00'))
  })

  it('switches the summary, chart, and day details to a group without fetching again', async () => {
    const allDays = [day(40, 8)]
    const firstDays = [day(30, 6)]
    getWorkbenchMonthlyReport.mockResolvedValue({ days: allDays, groups: [
      { group_id: 1, group_name: 'Claude', days: firstDays },
      { group_id: 2, group_name: 'Gemini', days: [day(10, 2)] }
    ] })
    const wrapper = mount(SubAdminCommissionCalendar, { global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$40.00')
    await wrapper.get('[data-test="commission-group-1"]').trigger('click')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$30.00')
    expect(wrapper.findComponent(TrendChartStub).props('days')).toEqual(firstDays)
    expect(wrapper.get('[data-test="commission-calendar-day-2026-09-03"]').attributes('aria-label')).toContain('$6.00')
    await wrapper.get('[data-test="commission-calendar-day-2026-09-03"]').trigger('click')
    expect(wrapper.emitted('select-day')?.at(-1)).toEqual(['2026-09-03', day(30, 6), 1])
    await wrapper.get('[data-test="commission-metric-actual"]').trigger('click')
    expect(wrapper.findComponent(TrendChartStub).props('metric')).toBe('actual')
    await wrapper.get('[data-test="commission-all-groups"]').trigger('click')
    expect(wrapper.findComponent(TrendChartStub).props('days')).toEqual(allDays)
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$40.00')
    expect(getWorkbenchMonthlyReport).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('scope-change')).toHaveLength(3)
  })

  it('clears group selection and group rows while refreshing, including after failure', async () => {
    getWorkbenchMonthlyReport.mockResolvedValueOnce({ days: [day(30, 6)], groups: [{ group_id: 1, group_name: 'Claude', days: [day(30, 6)] }] })
      .mockRejectedValueOnce(new Error('monthly report unavailable'))
    const wrapper = mount(SubAdminCommissionCalendar, { global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    await wrapper.get('[data-test="commission-group-1"]').trigger('click')
    await wrapper.setProps({ refreshKey: 1 })
    await flushPromises()
    expect(wrapper.find('[data-test="commission-group-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="commission-all-groups"]').exists()).toBe(false)
    expect(wrapper.findComponent(TrendChartStub).props('days')).toEqual([])
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$30.00')
  })
  it('sorts groups by earnings, paginates ten rows, and keeps shares relative to the full month when searching', async () => {
    const groups = Array.from({ length: 12 }, (_, index) => ({ group_id: index + 1, group_name: `Group ${index + 1}`, days: [day(10, index + 1)] }))
    getWorkbenchMonthlyReport.mockResolvedValue({ days: [day(120, 156), { ...day(0, 0), date: '2026-09-30' }], groups })
    const wrapper = mount(SubAdminCommissionCalendar, { global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    const rows = wrapper.findAll('[data-test^="commission-group-"]').filter(row => /^commission-group-\d+$/.test(row.attributes('data-test') ?? ''))
    expect(rows).toHaveLength(10)
    expect(wrapper.findAll('svg')).toHaveLength(10)
    const points = rows[0].get('polyline').attributes('points').split(' ').map(point => point.split(',').map(Number))
    expect(points).toHaveLength(30)
    expect(points[0][1]).toBe(44)
    expect(points[2][1]).toBe(4)
    expect(points[29][1]).toBe(44)
    expect(rows[0].attributes('data-test')).toBe('commission-group-12')
    expect(rows[0].text()).toContain('7.7%')
    await wrapper.get('[data-test="commission-groups-next"]').trigger('click')
    expect(wrapper.find('[data-test="commission-group-2"]').exists()).toBe(true)
    await wrapper.get('[data-test="commission-groups-search"]').setValue('Group 12')
    expect(wrapper.get('[data-test="commission-group-12"]').text()).toContain('7.7%')
    expect(wrapper.find('[data-test="commission-group-2"]').exists()).toBe(false)
    await wrapper.get('[data-test="commission-groups-search"]').setValue('9999')
    expect(wrapper.find('[data-test="commission-groups-no-match"]').exists()).toBe(true)
    expect(getWorkbenchMonthlyReport).toHaveBeenCalledTimes(1)
  })

  it('shows zero earnings shares without invalid percentages and retains an empty authorization state', async () => {
    getWorkbenchMonthlyReport.mockResolvedValueOnce({ days: [day(0, 0)], groups: [{ group_id: 1, group_name: 'Zero', days: [day(0, 0)] }] })
      .mockResolvedValueOnce({ days: [], groups: [] })
    const wrapper = mount(SubAdminCommissionCalendar, { global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    expect(wrapper.get('[data-test="commission-group-1"]').text()).toContain('0.0%')
    expect(wrapper.text()).not.toMatch(/NaN|Infinity/)
    expect(wrapper.get('polyline').attributes('points').split(' ').every(point => point.endsWith(',44'))).toBe(true)
    await wrapper.setProps({ refreshKey: 1 })
    await flushPromises()
    expect(wrapper.get('[data-test="commission-groups-empty"]').text()).toContain('adminWorkbench.commission.groupsEmpty')
  })

  it('ends sparklines at the report cutoff instead of drawing future dates as zero', async () => {
    getWorkbenchMonthlyReport.mockResolvedValue({ days: [day(30, 6)], groups: [{ group_id: 1, group_name: 'Claude', days: [day(30, 6)] }] })
    const wrapper = mount(SubAdminCommissionCalendar, { global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
    mountedWrappers.push(wrapper)
    await flushPromises()
    const points = wrapper.get('polyline').attributes('points').split(' ')
    expect(points).toHaveLength(3)
    expect(points.at(-1)).toBe('238,4')
  })

  it('brings the overview into view when selecting a group and respects reduced motion', async () => {
    const scrollIntoView = vi.fn()
    const originalScroll = HTMLElement.prototype.scrollIntoView
    const originalMatchMedia = window.matchMedia
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: scrollIntoView })
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: vi.fn(() => ({ matches: true })) })
    try {
      getWorkbenchMonthlyReport.mockResolvedValue({ days: [day(30, 6)], groups: [{ group_id: 1, group_name: 'Claude', days: [day(30, 6)] }] })
      const wrapper = mount(SubAdminCommissionCalendar, { global: { stubs: { SubAdminCommissionTrendChart: TrendChartStub } } })
      mountedWrappers.push(wrapper)
      await flushPromises()
      await wrapper.get('[data-test="commission-group-1"]').trigger('click')
      expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'auto' })
    } finally {
      Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { configurable: true, value: originalScroll })
      Object.defineProperty(window, 'matchMedia', { configurable: true, value: originalMatchMedia })
    }
  })

})
