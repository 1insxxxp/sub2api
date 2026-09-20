<template>
  <AppLayout>
    <div class="admin-workbench-page dashboard-workbench">
      <nav class="dashboard-shortcuts" :aria-label="t('admin.dashboard.quickActions')">
        <button type="button" @click="router.push('/admin/accounts')"><Icon name="server" size="sm" />{{ t('admin.dashboard.manageAccounts') }}</button>
        <button type="button" @click="router.push('/admin/users')"><Icon name="users" size="sm" />{{ t('admin.dashboard.manageUsers') }}</button>
        <button type="button" @click="router.push('/admin/groups')"><Icon name="grid" size="sm" />{{ t('admin.dashboard.groupPricing') }}</button>
        <button v-if="canUseBatchImage" type="button" @click="router.push('/batch-image')"><Icon name="sparkles" size="sm" />{{ t('admin.dashboard.batchImage') }}</button>
      </nav>
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <!-- Row 1: Core Stats -->
        <div data-test="dashboard-core-metrics" class="dashboard-core-metrics">
          <!-- Total API Keys -->
          <div class="stat-card">
            <div class="stat-icon stat-icon-primary">
              <Icon name="key" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.apiKeys') }}
              </p>
              <p class="stat-value">
                {{ stats.total_api_keys }}
              </p>
              <p class="stat-trend stat-trend-up">
                {{ stats.active_api_keys }} {{ t('common.active') }}
              </p>
            </div>
          </div>

          <!-- Service Accounts -->
          <div class="stat-card">
            <div class="stat-icon bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300">
              <Icon name="server" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.accounts') }}
              </p>
              <p class="stat-value">
                {{ stats.total_accounts }}
              </p>
              <p class="text-xs">
                <span class="text-green-600 dark:text-green-400"
                  >{{ stats.normal_accounts }} {{ t('common.active') }}</span
                >
                <span v-if="stats.error_accounts > 0" class="ml-1 text-red-500"
                  >{{ stats.error_accounts }} {{ t('common.error') }}</span
                >
              </p>
            </div>
          </div>

          <!-- Today Requests -->
          <div class="stat-card">
            <div class="stat-icon bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-300">
              <Icon name="chart" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.todayRequests') }}
              </p>
              <p class="stat-value">
                {{ stats.today_requests }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('common.total') }}: {{ formatNumber(stats.total_requests) }}
              </p>
            </div>
          </div>

          <!-- New Users Today -->
          <div class="stat-card">
            <div class="stat-icon bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
              <Icon name="userPlus" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.users') }}
              </p>
              <p class="stat-value text-primary-600 dark:text-primary-400">
                +{{ stats.today_new_users }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('common.total') }}: {{ formatNumber(stats.total_users) }}
              </p>
            </div>
          </div>
        </div>

        <!-- Row 2: Token Stats -->
        <div data-test="dashboard-secondary-metrics" class="dashboard-secondary-metrics">
          <!-- Today Tokens -->
          <div class="stat-card">
            <div class="stat-icon bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-300">
              <Icon name="cube" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.todayTokens') }}
              </p>
              <p class="stat-value">
                {{ formatTokens(stats.today_tokens) }}
              </p>
              <p class="text-xs">
                <span
                  class="text-green-600 dark:text-green-400"
                  :title="t('admin.dashboard.actual')"
                  >${{ formatCost(stats.today_actual_cost) }}</span
                >
                <span class="text-gray-400 dark:text-gray-500"> / </span>
                <span
                  class="text-orange-500 dark:text-orange-400"
                  :title="t('admin.dashboard.accountCost')"
                  >${{ formatCost(stats.today_account_cost) }}</span
                >
                <span class="text-gray-400 dark:text-gray-500"> / </span>
                <span
                  class="text-gray-400 dark:text-gray-500"
                  :title="t('admin.dashboard.standard')"
                  >${{ formatCost(stats.today_cost) }}</span
                >
              </p>
            </div>
          </div>

          <!-- Total Tokens -->
          <div class="stat-card">
            <div class="stat-icon bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300">
              <Icon name="database" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.totalTokens') }}
              </p>
              <p class="stat-value">
                {{ formatTokens(stats.total_tokens) }}
              </p>
              <p class="text-xs">
                <span
                  class="text-green-600 dark:text-green-400"
                  :title="t('admin.dashboard.actual')"
                  >${{ formatCost(stats.total_actual_cost) }}</span
                >
                <span class="text-gray-400 dark:text-gray-500"> / </span>
                <span
                  class="text-orange-500 dark:text-orange-400"
                  :title="t('admin.dashboard.accountCost')"
                  >${{ formatCost(stats.total_account_cost) }}</span
                >
                <span class="text-gray-400 dark:text-gray-500"> / </span>
                <span
                  class="text-gray-400 dark:text-gray-500"
                  :title="t('admin.dashboard.standard')"
                  >${{ formatCost(stats.total_cost) }}</span
                >
              </p>
            </div>
          </div>

          <!-- Performance (RPM/TPM) -->
          <div class="stat-card">
            <div class="stat-icon bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
              <Icon name="bolt" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div class="flex-1">
              <p class="stat-label">
                {{ t('admin.dashboard.performance') }}
              </p>
              <div class="flex items-baseline gap-2">
                <p class="stat-value">
                  {{ formatTokens(stats.rpm) }}
                </p>
                <span class="text-xs text-gray-500 dark:text-gray-400">RPM</span>
              </div>
              <div class="flex items-baseline gap-2">
                <p class="text-sm font-semibold text-primary-600 dark:text-primary-400">
                  {{ formatTokens(stats.tpm) }}
                </p>
                <span class="text-xs text-gray-500 dark:text-gray-400">TPM</span>
              </div>
            </div>
          </div>

          <!-- Avg Response Time -->
          <div class="stat-card">
            <div class="stat-icon bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-300">
              <Icon name="clock" size="md" class="text-current" :stroke-width="2" />
            </div>
            <div>
              <p class="stat-label">
                {{ t('admin.dashboard.avgResponse') }}
              </p>
              <p class="stat-value">
                {{ formatDuration(stats.average_duration_ms) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}
              </p>
            </div>
          </div>
        </div>

        <!-- Charts Section -->
        <div class="dashboard-analysis">
          <!-- Date Range Filter -->
          <div data-test="dashboard-workbench-controls" class="admin-toolbar-surface dashboard-analysis-controls">
            <div class="admin-toolbar">
              <div class="dashboard-date-controls">
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <div class="dashboard-chart-actions">
                <div class="dashboard-granularity" role="group" :aria-label="t('admin.dashboard.granularity')">
                  <button v-for="option in granularityOptions" :key="option.value" type="button"
                    :aria-pressed="granularity === option.value"
                    @click="granularity = option.value; loadChartData()">{{ option.label }}</button>
                </div>
                <button type="button" data-test="dashboard-refresh" @click="loadDashboardStats" :disabled="chartsLoading"
                  class="btn btn-secondary btn-icon" :title="t('common.refresh')" :aria-label="t('common.refresh')">
                  <Icon name="refresh" size="sm" :class="{ 'animate-spin': chartsLoading }" />
                </button>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="dashboard-charts-grid">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend
              :trend-data="trendData"
              :loading="chartsLoading"
              :latest-hours="latestHourlyBucketCount"
            />
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div data-test="dashboard-user-trend-surface" class="admin-surface dashboard-user-trend">
            <div data-test="dashboard-user-trend-header" class="admin-panel-header">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboard.userUsageTrend') }}
                </h3>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ startDate }} - {{ endDate }}
                </p>
              </div>
            </div>
            <div class="p-4 sm:p-5">
              <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { formatTokenCount } from '@/utils/format'
import { getLatestHourlyBuckets } from '@/utils/hourlyBuckets'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
const rankingLimit = 12

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

const isDefaultRollingRange = computed(
  () =>
    granularity.value === 'hour' &&
    startDate.value === defaultRange.start &&
    endDate.value === defaultRange.end
)

const shouldFillLatestHourlyBuckets = computed(
  () =>
    granularity.value === 'hour' &&
    (
      isDefaultRollingRange.value ||
      (
        startDate.value === formatLocalDate(new Date()) &&
        endDate.value === startDate.value
      )
    )
)

const latestHourlyBucketCount = computed(() => {
  if (!shouldFillLatestHourlyBuckets.value) return undefined
  return isDefaultRollingRange.value ? 25 : 24
})

type DashboardRangeParams = {
  start_date: string
  end_date: string
  start_time?: string
  end_time?: string
  granularity: 'day' | 'hour'
}

const createDashboardRangeParams = (): DashboardRangeParams => {
  const params: DashboardRangeParams = {
    start_date: startDate.value,
    end_date: endDate.value,
    granularity: granularity.value
  }

  if (isDefaultRollingRange.value) {
    const end = new Date()
    const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
    params.start_time = start.toISOString()
    params.end_time = end.toISOString()
  }

  return params
}

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day' as const, label: t('admin.dashboard.day') },
  { value: 'hour' as const, label: t('admin.dashboard.hour') }
])

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, point.tokens)
  })

  const sortedDates = latestHourlyBucketCount.value
    ? getLatestHourlyBuckets(latestHourlyBucketCount.value)
    : Array.from(allDates).sort()
  const colors = [
    '#2563eb',
    '#0891b2',
    '#475569',
    '#60a5fa',
    '#0e7490',
    '#94a3b8',
    '#f59e0b',
    '#ef4444'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

// Format helpers
const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  return formatTokenCount(value)
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  // Auto-select granularity based on date range
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  loadChartData()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean, range: DashboardRangeParams) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      ...range,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadUsersTrend = async (range: DashboardRangeParams) => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      ...range,
      limit: 12
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async (range: DashboardRangeParams) => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      ...range,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadDashboardStats = async () => {
  const range = createDashboardRangeParams()
  await Promise.all([
    loadDashboardSnapshot(true, range),
    loadUsersTrend(range),
    loadUserSpendingRanking(range)
  ])
}

const loadChartData = async () => {
  const range = createDashboardRangeParams()
  await Promise.all([
    loadDashboardSnapshot(false, range),
    loadUsersTrend(range),
    loadUserSpendingRanking(range)
  ])
}

onMounted(() => {
  void refreshBatchImageAccess()
  loadDashboardStats()
})
</script>

<style scoped>
.dashboard-workbench { display: flex; flex-direction: column; gap: 1.5rem; min-width: 0; }
.dashboard-shortcuts { display: flex; flex-wrap: wrap; align-items: center; gap: 0.375rem 1.25rem; }
.dashboard-shortcuts button { display: inline-flex; align-items: center; gap: 0.5rem; min-height: 2rem; color: var(--workspace-muted); font-size: 0.8125rem; font-weight: 500; transition: color 150ms ease; }
.dashboard-shortcuts button:hover { color: rgb(var(--brand-rgb)); }
.dashboard-shortcuts button:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 50%); outline-offset: 4px; border-radius: 4px; }
.dashboard-core-metrics,
.dashboard-secondary-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1rem; }
.dashboard-workbench .stat-card { position: relative; display: flex; flex-direction: column; align-items: stretch; gap: 0.875rem; min-width: 0; min-height: 8.75rem; padding: 1.25rem; border: 1px solid var(--workspace-rule); border-radius: 8px; background: linear-gradient(125deg, var(--workspace-highlight) 15%, transparent 70%), var(--workspace-surface); box-shadow: var(--workspace-shadow); }
.dashboard-workbench .stat-card::before { content: none; }
.dashboard-workbench .stat-icon { position: absolute; top: 1.125rem; right: 1.125rem; width: 1.875rem; height: 1.875rem; border-radius: 7px; }
.dashboard-workbench .stat-icon :deep(svg) { width: 1rem; height: 1rem; }
.dashboard-workbench .stat-label { max-width: calc(100% - 2.25rem); min-height: 1.875rem; padding-top: 0.25rem; font-size: 0.8125rem; }
.dashboard-workbench .stat-value { margin: 0.625rem 0; font-size: 1.75rem; font-weight: 600; line-height: 1.15; letter-spacing: 0; white-space: normal; overflow: visible; text-overflow: clip; }
.dashboard-analysis { display: flex; flex-direction: column; gap: 1.25rem; min-width: 0; }
.dashboard-analysis-controls .admin-toolbar { flex-direction: row; flex-wrap: wrap; gap: 0.75rem; justify-content: space-between; align-items: center; }
.dashboard-date-controls { min-width: 0; }
.dashboard-chart-actions { display: flex; align-items: center; gap: 0.625rem; }
.dashboard-granularity { display: inline-flex; gap: 0.25rem; padding: 0.25rem; border: 1px solid var(--workspace-rule); border-radius: 7px; background: var(--workspace-hover); }
.dashboard-granularity button { min-width: 3.5rem; min-height: 1.875rem; padding: 0.25rem 0.625rem; border: 1px solid transparent; border-radius: 4px; color: var(--workspace-muted); font-size: 0.75rem; font-weight: 500; }
.dashboard-granularity button[aria-pressed='true'] { border-color: var(--workspace-rule); background: var(--workspace-control); color: var(--workspace-ink); box-shadow: var(--workspace-shadow); }
.dashboard-granularity button:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 50%); outline-offset: 1px; }
.dashboard-charts-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1.5rem; }
.dashboard-charts-grid :deep(.card),
.dashboard-workbench .dashboard-user-trend { min-width: 0; border: 1px solid var(--workspace-rule); border-radius: 8px; background: linear-gradient(125deg, var(--workspace-highlight) 15%, transparent 70%), var(--workspace-surface); box-shadow: var(--workspace-shadow); }
.dashboard-charts-grid :deep(.card) { padding: 1rem; }
.dashboard-charts-grid :deep(.card::before) { content: none; }
.dashboard-charts-grid :deep(.rounded-2xl) { border-radius: 7px; border-color: var(--workspace-rule); background: var(--workspace-control); box-shadow: none; }
.dashboard-charts-grid :deep(.rounded-xl) { border-radius: 4px; }
.dashboard-user-trend .admin-panel-header { padding: 1rem; border-bottom: 1px solid var(--workspace-divider); }
.dashboard-user-trend .admin-panel-header p { margin-top: 0.25rem; color: var(--workspace-muted); font-size: 0.75rem; font-variant-numeric: tabular-nums; }
.dashboard-user-trend > .p-4 { padding: 1rem; }
@media (max-width: 1023px) {
  .dashboard-core-metrics,
  .dashboard-secondary-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.75rem; }
  .dashboard-charts-grid { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 639px) {
  .dashboard-workbench { gap: 1.125rem; }
  .dashboard-shortcuts { gap: 0.375rem 1rem; }
  .dashboard-shortcuts button { font-size: 0.75rem; }
  .dashboard-workbench .stat-card { min-height: 7.75rem; padding: 0.875rem; }
  .dashboard-workbench .stat-icon { top: 0.75rem; right: 0.75rem; width: 1.5rem; height: 1.5rem; }
  .dashboard-workbench .stat-label { min-height: 1.5rem; padding-top: 0; font-size: 0.75rem; }
  .dashboard-workbench .stat-value { font-size: 1.5rem; margin: 0.375rem 0; }
  .dashboard-charts-grid { gap: 1.125rem; }
  .dashboard-charts-grid :deep(.card),
  .dashboard-user-trend .admin-panel-header,
  .dashboard-user-trend > .p-4 { padding: 0.875rem; }
  .dashboard-chart-actions { margin-left: auto; }
}
@media (prefers-reduced-motion: reduce) {
  .dashboard-shortcuts button { transition: none; }
  .dashboard-chart-actions .animate-spin { animation: none; }
}
</style>
