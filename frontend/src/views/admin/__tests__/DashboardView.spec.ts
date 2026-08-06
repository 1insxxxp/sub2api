import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { h } from 'vue'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking, lineCapture } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn(),
  lineCapture: { props: null as any }
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    setup(props: any) {
      lineCapture.props = props
      return () => null
    }
  }
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
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

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()
    lineCapture.props = null

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
  })

  it('uses the last 24 hours as the default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('renders branded metric cards and dashboard controls without the redundant page hero', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.stat-card')).toHaveLength(8)
    expect(wrapper.findAll('.admin-toolbar-surface').length).toBeGreaterThan(0)
    expect(wrapper.find('.admin-toolbar').exists()).toBe(true)
    expect(wrapper.find('[data-test="admin-page-hero"]').exists()).toBe(false)
    expect(wrapper.find('.admin-page-hero').exists()).toBe(false)
    expect(wrapper.find('[data-test="dashboard-user-trend-surface"]').classes()).toContain('admin-surface')
    expect(wrapper.find('[data-test="dashboard-user-trend-header"]').classes()).toContain('admin-panel-header')
  })

  it('fills the recent usage chart with the latest 24 hourly buckets', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 7, 5, 1, 30))
    getUserUsageTrend.mockResolvedValue({
      trend: [
        { date: '2026-08-04 18:00', user_id: 1, username: 'admin', email: '', requests: 1, tokens: 6200, cost: 0, actual_cost: 0 },
        { date: '2026-08-04 19:00', user_id: 1, username: 'admin', email: '', requests: 1, tokens: 400, cost: 0, actual_cost: 0 }
      ],
      start_date: '2026-08-04',
      end_date: '2026-08-05',
      granularity: 'hour'
    })

    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' }, LoadingSpinner: true, Icon: true,
          DateRangePicker: true, Select: true, ModelDistributionChart: true, TokenUsageTrend: true,
        }
      }
    })
    await flushPromises()

    const chartData = lineCapture.props.data as any
    expect(chartData.labels).toHaveLength(24)
    expect(chartData.labels[0]).toBe('2026-08-04 02:00')
    expect(chartData.labels.at(-1)).toBe('2026-08-05 01:00')
    expect(chartData.datasets[0].data[16]).toBe(6200)
    expect(chartData.datasets[0].data[17]).toBe(400)
    expect(chartData.datasets[0].data[0]).toBe(0)
    vi.useRealTimers()
  })

  it('keeps 24 hourly buckets when today is selected', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 7, 7, 1, 30))
    getUserUsageTrend.mockResolvedValue({
      trend: [
        { date: '2026-08-07 00:00', user_id: 1, username: 'admin', email: '', requests: 1, tokens: 6200, cost: 0, actual_cost: 0 },
        { date: '2026-08-07 01:00', user_id: 1, username: 'admin', email: '', requests: 1, tokens: 400, cost: 0, actual_cost: 0 }
      ],
      start_date: '2026-08-07',
      end_date: '2026-08-07',
      granularity: 'hour'
    })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: {
            emits: ['update:startDate', 'update:endDate', 'change'],
            setup(_, { emit }) {
              return () => h('button', {
                'data-test': 'choose-today',
                onClick: () => {
                  emit('update:startDate', '2026-08-07')
                  emit('update:endDate', '2026-08-07')
                  emit('change', {
                    startDate: '2026-08-07',
                    endDate: '2026-08-07',
                    preset: 'today'
                  })
                }
              }, 'today')
            }
          },
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true
        }
      }
    })
    await flushPromises()

    await wrapper.get('[data-test="choose-today"]').trigger('click')
    await flushPromises()

    const chartData = lineCapture.props.data as any
    expect(chartData.labels).toHaveLength(24)
    expect(chartData.labels[0]).toBe('2026-08-06 02:00')
    expect(chartData.labels.at(-1)).toBe('2026-08-07 01:00')
    vi.useRealTimers()
  })
})
