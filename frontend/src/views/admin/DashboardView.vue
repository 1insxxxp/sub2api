<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <section class="admin-page-hero" data-test="admin-page-hero">
          <div class="admin-page-hero-grid">
            <div class="min-w-0">
              <span class="admin-page-kicker">{{ t('admin.dashboard.title') }}</span>
              <h2 class="admin-page-title">{{ t('admin.dashboard.title') }}</h2>
              <p class="admin-page-description">
                {{ t('admin.dashboard.description') }}
              </p>
              <div class="admin-page-meta">
                <span class="admin-page-meta-chip">
                  <span>{{ t('admin.dashboard.timeRange') }}</span>
                  <strong>{{ startDate }} - {{ endDate }}</strong>
                </span>
                <span class="admin-page-meta-chip">
                  <span>{{ t('admin.dashboard.granularity') }}</span>
                  <strong>{{ granularity }}</strong>
                </span>
                <span class="admin-page-meta-chip">
                  <span>{{ t('admin.dashboard.activeUsers') }}</span>
                  <strong>{{ stats.active_users }}</strong>
                </span>
              </div>
            </div>

            <div class="admin-page-actions">
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
            </div>
          </div>
        </section>

        <!-- Row 1: Core Stats -->
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
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
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
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
        <div class="space-y-6">
          <!-- Date Range Filter -->
          <div class="admin-toolbar-surface">
            <div class="admin-toolbar">
              <div class="admin-toolbar-group">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="admin-toolbar-group justify-end lg:ml-auto">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
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
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div data-test="dashboard-user-trend-surface" class="admin-surface overflow-hidden">
            <div data-test="dashboard-user-trend-header" class="admin-panel-header">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.dashboard.recentUsage') }} (Top 12)
                </h3>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ startDate }} - {{ endDate }}
                </p>
              </div>
              <div class="badge badge-primary">
                <span>{{ t('admin.dashboard.granularity') }}</span>
                <strong class="font-semibold text-slate-950 dark:text-white">{{ granularity }}</strong>
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
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { formatTokenCount } from '@/utils/format'

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

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
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

  const sortedDates = Array.from(allDates).sort()
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

const formatNumber = (value: number | null | undefined): string => {
  value = safeNumber(value)
  return value.toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  value = safeNumber(value)
  if (value >= 1000) {
    return (value / 1000).toFixed(2) + 'K'
  } else if (value >= 1) {
    return value.toFixed(2)
  } else if (value >= 0.01) {
    return value.toFixed(3)
  }
  return value.toFixed(4)
}

const safeNumber = (value: number | null | undefined): number => {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
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
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
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

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
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

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
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
  await Promise.all([
    loadDashboardSnapshot(true),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

onMounted(() => {
  loadDashboardStats()
})
</script>

<style scoped>
</style>
