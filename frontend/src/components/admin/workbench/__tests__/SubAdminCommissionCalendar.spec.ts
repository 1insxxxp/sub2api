import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, type PropType } from 'vue'

import type { SubAdminCommissionCalendarDay } from '@/api/admin/subAdminCommission'
import SubAdminCommissionCalendar from '../SubAdminCommissionCalendar.vue'

const { getWorkbenchCalendar } = vi.hoisted(() => ({
  getWorkbenchCalendar: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    subAdminCommission: {
      getWorkbenchCalendar,
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
  getWorkbenchCalendar.mockReset()
})

describe('SubAdminCommissionCalendar stale month data', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-14T12:00:00'))
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
    getWorkbenchCalendar
      .mockResolvedValueOnce(previousMonthDays)
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
    expect(getWorkbenchCalendar).toHaveBeenNthCalledWith(1, { month: '2026-09' })
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$12.00')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$3.45')
    expect(chart.props('days')).toEqual(previousMonthDays)

    await monthInput.setValue('2026-08')
    await flushPromises()

    expect(getWorkbenchCalendar).toHaveBeenNthCalledWith(2, { month: '2026-08' })
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$12.00')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$3.45')
    expect(chart.props('days')).toEqual([])
  })

  it('ignores a late response from an older month request', async () => {
    const initial = deferred<SubAdminCommissionCalendarDay[]>()
    const stale = deferred<SubAdminCommissionCalendarDay[]>()
    const latest = deferred<SubAdminCommissionCalendarDay[]>()
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
    getWorkbenchCalendar
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
    initial.resolve([])
    await flushPromises()

    const monthInput = wrapper.get('input[type="month"]')
    await monthInput.setValue('2026-08')
    await monthInput.setValue('2026-07')
    expect(getWorkbenchCalendar).toHaveBeenNthCalledWith(2, { month: '2026-08' })
    expect(getWorkbenchCalendar).toHaveBeenNthCalledWith(3, { month: '2026-07' })

    latest.resolve(latestMonthDays)
    await flushPromises()
    const chart = wrapper.findComponent({ name: 'TrendChartStub' })
    expect(chart.props('days')).toEqual(latestMonthDays)

    stale.resolve(staleMonthDays)
    await flushPromises()
    expect(chart.props('days')).toEqual(latestMonthDays)
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).toContain('$30.00')
    expect(wrapper.get('[data-test="commission-calendar-month-summary"]').text()).not.toContain('$80.00')
  })
})
