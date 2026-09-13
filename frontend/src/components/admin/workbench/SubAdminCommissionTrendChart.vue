<template>
  <section
    class="min-w-0 max-w-full overflow-hidden rounded-lg border border-gray-100 bg-gray-50/70 p-3 dark:border-dark-800 dark:bg-dark-950/40 sm:p-4"
  >
    <div class="mb-3 flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-sm font-semibold text-gray-950 dark:text-white">
          {{ t('adminWorkbench.commission.dailyChartTitle') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
          {{ t('adminWorkbench.commission.monthTotal') }}
        </p>
      </div>
    </div>

    <div data-test="commission-daily-chart" class="commission-daily-chart min-w-0 w-full h-48 overflow-hidden sm:h-64">
      <div
        v-if="loading"
        data-test="commission-daily-chart-loading"
        class="flex h-full items-center justify-center rounded-md text-sm text-gray-500 dark:text-dark-400"
      >
        <span class="animate-pulse">{{ t('common.loading') }}</span>
      </div>
      <Line
        v-else-if="chartData"
        :data="chartData"
        :options="chartOptions"
        class="relative block h-full w-full min-w-0"
      />
      <div
        v-else
        data-test="commission-daily-chart-empty"
        class="flex h-full items-center justify-center rounded-md text-sm text-gray-500 dark:text-dark-400"
      >
        {{ t('adminWorkbench.commission.dailyChartEmpty') }}
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Title,
  Tooltip,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import type { SubAdminCommissionCalendarDay } from '@/api/admin/subAdminCommission'

ChartJS.register(CategoryScale, Filler, Legend, LineElement, LinearScale, PointElement, Title, Tooltip)

const props = defineProps<{
  days: SubAdminCommissionCalendarDay[]
  loading?: boolean
}>()

const { t } = useI18n()

function readIsDarkMode() {
  if (typeof document === 'undefined') {
    return false
  }
  return document.documentElement?.classList.contains('dark') ?? false
}

const isDarkMode = ref(readIsDarkMode())
let themeObserver: MutationObserver | null = null

function syncTheme() {
  isDarkMode.value = readIsDarkMode()
}

onMounted(() => {
  syncTheme()
  if (typeof MutationObserver === 'undefined' || typeof document === 'undefined') {
    return
  }
  themeObserver = new MutationObserver(syncTheme)
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
  themeObserver = null
})

const colors = computed(() => ({
  text: isDarkMode.value ? '#d1d5db' : '#4b5563',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb',
  commission: '#10b981',
  commissionFill: isDarkMode.value ? 'rgba(16, 185, 129, 0.18)' : 'rgba(16, 185, 129, 0.12)'
}))

const orderedDays = computed(() =>
  [...(props.days ?? [])].sort((left, right) => left.date.localeCompare(right.date))
)

const chartData = computed(() => {
  if (orderedDays.value.length === 0) {
    return null
  }

  return {
    labels: orderedDays.value.map((day) => day.date.slice(5)),
    datasets: [
      {
        label: t('adminWorkbench.commission.dailyChartCommission'),
        data: orderedDays.value.map((day) => day.commission_amount ?? 0),
        borderColor: colors.value.commission,
        backgroundColor: colors.value.commissionFill,
        fill: true,
        tension: 0.32,
        pointRadius: 2,
        pointHoverRadius: 4,
        pointHitRadius: 10
      }
    ]
  }
})

const formatCurrency = (value: unknown) => {
  const number = typeof value === 'number' ? value : Number(value)
  return `$${(Number.isFinite(number) ? number : 0).toFixed(2)}`
}

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'end' as const,
      labels: {
        color: colors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 7,
        padding: 10,
        font: { size: 10 }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: TooltipItem<'line'>) =>
          `${context.dataset.label ?? ''}: ${formatCurrency(context.parsed.y)}`
      },
      backgroundColor: isDarkMode.value ? '#1f2937' : '#ffffff',
      titleColor: isDarkMode.value ? '#f3f4f6' : '#111827',
      bodyColor: isDarkMode.value ? '#d1d5db' : '#4b5563',
      borderColor: colors.value.grid,
      borderWidth: 1,
      padding: 10,
      displayColors: true
    }
  },
  scales: {
    x: {
      type: 'category' as const,
      grid: { display: false },
      ticks: {
        color: colors.value.text,
        font: { size: 10 },
        maxTicksLimit: 6,
        autoSkip: true,
        autoSkipPadding: 4,
        maxRotation: 0
      }
    },
    y: {
      type: 'linear' as const,
      beginAtZero: true,
      grid: { color: colors.value.grid, borderDash: [4, 4] },
      ticks: {
        color: colors.value.text,
        font: { size: 10 },
        maxTicksLimit: 5,
        callback: (value: string | number) => formatCurrency(value)
      }
    }
  }
}))
</script>

<style scoped>
.commission-daily-chart :deep(canvas) {
  display: block;
  width: 100% !important;
  max-width: 100%;
  height: 100% !important;
}
</style>
