import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

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

const days: SubAdminCommissionCalendarDay[] = [
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

function mountChart(props: { days: SubAdminCommissionCalendarDay[]; loading?: boolean }) {
  return mount(SubAdminCommissionTrendChart, { props })
}

describe('SubAdminCommissionTrendChart', () => {
  it('sorts daily points and preserves zero values across aligned datasets', () => {
    const wrapper = mountChart({ days })
    const line = wrapper.findComponent({ name: 'LineChartStub' })

    expect(line.exists()).toBe(true)

    const data = line.props('data') as ChartData
    expect(data.labels).toEqual(['08-01', '08-02', '08-03'])

    const actualCost = data.datasets.find(
      (dataset) => dataset.label === 'adminWorkbench.commission.dailyChartActualCost'
    )
    const commission = data.datasets.find(
      (dataset) => dataset.label === 'adminWorkbench.commission.dailyChartCommission'
    )

    expect(actualCost?.data).toEqual([12.25, 0, 0])
    expect(commission?.data).toEqual([0, 0, 2.5])
    expect(actualCost?.data).toHaveLength(data.labels.length)
    expect(commission?.data).toHaveLength(data.labels.length)
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
    const wrapper = mountChart({ days })
    const chart = wrapper.get('[data-test="commission-daily-chart"]')

    expect(chart.classes()).toEqual(
      expect.arrayContaining(['min-w-0', 'h-56', 'sm:h-64'])
    )
  })
})
