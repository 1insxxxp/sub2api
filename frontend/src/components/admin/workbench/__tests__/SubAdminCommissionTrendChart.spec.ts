import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import type { SubAdminCommissionCalendarDay } from '@/api/admin/subAdminCommission'
import SubAdminCommissionTrendChart from '../SubAdminCommissionTrendChart.vue'

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  Filler: {},
  Legend: {},
  LineElement: {},
  LinearScale: {},
  PointElement: {},
  Title: {},
  Tooltip: {},
}))

vi.mock('vue-chartjs', async () => {
  const { defineComponent } = await import('vue')

  return {
    Line: defineComponent({
      name: 'LineChartStub',
      props: {
        data: { type: Object, required: true },
        options: { type: Object, default: () => ({}) },
      },
      template: '<div data-test="line-chart-stub" />',
    }),
  }
})

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

type ChartDataset = {
  label?: string
  data: number[]
}

type ChartData = {
  labels: string[]
  datasets: ChartDataset[]
}

function createDays(): SubAdminCommissionCalendarDay[] {
  return [
    {
      date: '2026-08-03',
      enabled: true,
      actual_cost: 0,
      commission_amount: 2.5,
    },
    {
      date: '2026-08-01',
      enabled: true,
      actual_cost: 12.25,
      commission_amount: 0,
    },
    {
      date: '2026-08-02',
      enabled: true,
      actual_cost: 0,
      commission_amount: 0,
    },
  ]
}

function mountChart(props: { days: SubAdminCommissionCalendarDay[]; loading?: boolean; metric?: 'commission' | 'actual' }) {
  const wrapper = mount(SubAdminCommissionTrendChart, { props })
  mountedWrappers.push(wrapper)
  return wrapper
}

const mountedWrappers: Array<ReturnType<typeof mountChart>> = []

afterEach(() => {
  mountedWrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  document.documentElement.classList.remove('dark')
  vi.unstubAllGlobals()
})

describe('SubAdminCommissionTrendChart', () => {
  it('sorts daily earnings points and preserves zero values', () => {
    const inputDays = createDays()
    const wrapper = mountChart({ days: inputDays })
    const line = wrapper.findComponent({ name: 'LineChartStub' })

    expect(line.exists()).toBe(true)

    const data = line.props('data') as ChartData
    expect(data.labels).toEqual(['08-01', '08-02', '08-03'])

    const commission = data.datasets.find(
      (dataset) => dataset.label === 'adminWorkbench.commission.dailyChartCommission'
    )

    expect(data.datasets).toHaveLength(1)
    expect(commission?.data).toEqual([0, 0, 2.5])
    expect(commission?.data).toHaveLength(data.labels.length)
    expect(inputDays.map((day) => day.date)).toEqual([
      '2026-08-03',
      '2026-08-01',
      '2026-08-02',
    ])
  })

  it('switches to balance consumption without changing daily points', async () => {
    const wrapper = mountChart({ days: createDays() })
    await wrapper.setProps({ metric: 'actual' })
    const data = wrapper.findComponent({ name: 'LineChartStub' }).props('data') as ChartData
    expect(data.datasets[0].label).toBe('adminWorkbench.commission.actualCost')
    expect(data.datasets[0].data).toEqual([12.25, 0, 0])
  })

  it('renders the empty state without mounting a chart when there are no daily points', () => {
    const wrapper = mountChart({ days: [] })

    expect(wrapper.findComponent({ name: 'LineChartStub' }).exists()).toBe(false)
    expect(wrapper.get('[data-test="commission-daily-chart-empty"]').text()).toContain(
      'adminWorkbench.commission.dailyChartEmpty'
    )
  })

  it('renders a loading state before daily points are available', () => {
    const wrapper = mountChart({ days: [], loading: true })

    expect(wrapper.findComponent({ name: 'LineChartStub' }).exists()).toBe(false)
    expect(wrapper.get('[data-test="commission-daily-chart-loading"]').text()).toContain(
      'common.loading'
    )
  })

  it('keeps the chart container shrinkable and responsive on narrow screens', () => {
    const wrapper = mountChart({ days: createDays() })
    const chart = wrapper.get('[data-test="commission-daily-chart"]')

    expect(chart.classes()).toEqual(
      expect.arrayContaining([
        'commission-daily-chart',
        'min-w-0',
        'w-full',
        'h-48',
        'sm:h-64',
        'overflow-hidden'
      ])
    )

    expect(wrapper.findComponent({ name: 'LineChartStub' }).classes()).toEqual(
      expect.arrayContaining(['w-full', 'min-w-0'])
    )

    const options = wrapper.findComponent({ name: 'LineChartStub' }).props('options') as {
      responsive: boolean
      maintainAspectRatio: boolean
      scales: { x: { ticks: { maxTicksLimit: number; maxRotation: number } } }
    }
    expect(options.responsive).toBe(true)
    expect(options.maintainAspectRatio).toBe(false)
    expect(options.scales.x.ticks.maxTicksLimit).toBeLessThanOrEqual(6)
    expect(options.scales.x.ticks.maxRotation).toBe(0)
  })

  it('updates chart colors when the document theme changes', async () => {
    const wrapper = mountChart({ days: createDays() })
    const line = wrapper.findComponent({ name: 'LineChartStub' })
    const lightOptions = line.props('options') as { plugins: { legend: { labels: { color: string } } } }

    document.documentElement.classList.add('dark')
    await new Promise((resolve) => setTimeout(resolve, 0))
    await nextTick()

    const darkOptions = line.props('options') as { plugins: { legend: { labels: { color: string } } } }
    expect(lightOptions.plugins.legend.labels.color).toBe('#4b5563')
    expect(darkOptions.plugins.legend.labels.color).toBe('#d1d5db')
  })

  it('uses a short chart transition when reduced-motion detection is unavailable', () => {
    vi.stubGlobal('matchMedia', undefined)
    const wrapper = mountChart({ days: createDays() })
    const options = wrapper.findComponent({ name: 'LineChartStub' }).props('options') as { animation: { duration: number } }
    expect(options.animation).toEqual({ duration: 180 })
  })

  it('reacts to reduced-motion preferences and removes the modern listener on unmount', async () => {
    const query = {
      matches: true,
      addEventListener: vi.fn<(type: string, listener: () => void) => void>(),
      removeEventListener: vi.fn<(type: string, listener: () => void) => void>()
    }
    vi.stubGlobal('matchMedia', vi.fn(() => query))
    const wrapper = mountChart({ days: createDays() })
    const line = wrapper.findComponent({ name: 'LineChartStub' })
    expect((line.props('options') as { animation: unknown }).animation).toBe(false)
    expect(query.addEventListener).toHaveBeenCalledWith('change', expect.any(Function))
    const listener = query.addEventListener.mock.calls[0][1]
    query.matches = false
    listener()
    await nextTick()
    expect((line.props('options') as { animation: unknown }).animation).toEqual({ duration: 180 })
    query.matches = true
    listener()
    await nextTick()
    expect((line.props('options') as { animation: unknown }).animation).toBe(false)
    wrapper.unmount()
    mountedWrappers.splice(mountedWrappers.indexOf(wrapper), 1)
    expect(query.removeEventListener).toHaveBeenCalledWith('change', listener)
  })

  it('supports and cleans up legacy Safari motion preference listeners', async () => {
    const query = {
      matches: false,
      addListener: vi.fn<(listener: () => void) => void>(),
      removeListener: vi.fn<(listener: () => void) => void>()
    }
    vi.stubGlobal('matchMedia', vi.fn(() => query))
    const wrapper = mountChart({ days: createDays() })
    expect(query.addListener).toHaveBeenCalledWith(expect.any(Function))
    const listener = query.addListener.mock.calls[0][0]
    query.matches = true
    listener()
    await nextTick()
    const line = wrapper.findComponent({ name: 'LineChartStub' })
    expect((line.props('options') as { animation: unknown }).animation).toBe(false)
    wrapper.unmount()
    mountedWrappers.splice(mountedWrappers.indexOf(wrapper), 1)
    expect(query.removeListener).toHaveBeenCalledWith(listener)
  })

})
