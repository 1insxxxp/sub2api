<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <div class="stat-card">
      <div class="stat-icon bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
        <Icon name="document" size="md" />
      </div>
      <div>
        <p class="stat-label">{{ t('usage.totalRequests') }}</p>
        <p class="stat-value">{{ stats?.total_requests?.toLocaleString() || '0' }}</p>
        <p class="stat-trend">{{ t('usage.inSelectedRange') }}</p>
      </div>
    </div>
    <div class="stat-card">
      <div class="stat-icon bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-300">
        <Icon name="cube" size="md" />
      </div>
      <div class="min-w-0">
        <p class="stat-label">{{ t('usage.totalTokens') }}</p>
        <p class="stat-value">{{ formatTokens(stats?.total_tokens || 0) }}</p>
        <p class="flex flex-wrap items-center gap-x-1 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ t('usage.in') }}: {{ formatTokens(stats?.total_input_tokens || 0) }}</span>
          <span>/</span>
          <span>{{ t('usage.out') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}</span>
          <span>/</span>
          <span class="group relative inline-flex cursor-help items-center gap-0.5" tabindex="0">
            <span>{{ cacheLabel() }}: {{ formatTokens(stats?.total_cache_tokens || 0) }}</span>
            <svg
              class="h-3.5 w-3.5 text-gray-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span
              class="pointer-events-none absolute left-1/2 top-full z-30 mt-2 w-56 -translate-x-1/2 rounded-lg border border-gray-200 bg-white p-3 text-left text-xs text-gray-700 opacity-0 shadow-lg transition-opacity duration-150 group-hover:opacity-100 group-focus:opacity-100 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200"
            >
              <span class="mb-2 block font-medium text-gray-900 dark:text-white">
                {{ cacheDetailLabel() }}
              </span>
              <span class="flex items-center justify-between gap-3">
                <span>{{ t('usage.cacheCreationTokensLabel') }}</span>
                <span class="tabular-nums">
                  {{ formatTokens(stats?.total_cache_creation_tokens || 0) }}
                </span>
              </span>
              <span class="mt-1 flex items-center justify-between gap-3">
                <span>{{ t('usage.cacheReadTokensLabel') }}</span>
                <span class="tabular-nums">
                  {{ formatTokens(stats?.total_cache_read_tokens || 0) }}
                </span>
              </span>
            </span>
          </span>
        </p>
      </div>
    </div>
    <div class="stat-card">
      <div class="stat-icon bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-300">
        <Icon name="dollar" size="md" />
      </div>
      <div class="min-w-0 flex-1">
        <p class="stat-label">{{ t('usage.totalCost') }}</p>
        <p class="stat-value text-emerald-600 dark:text-emerald-300">
          ${{ (stats?.total_actual_cost || 0).toFixed(4) }}
        </p>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          <span class="text-orange-500">{{ t('usage.accountCost') }} ${{ (stats?.total_account_cost || 0).toFixed(4) }}</span>
          <span> · </span>
          <span>{{ t('usage.standardCost') }} ${{ (stats?.total_cost || 0).toFixed(4) }}</span>
        </p>
      </div>
    </div>
    <div class="stat-card">
      <div class="stat-icon bg-violet-100 text-violet-600 dark:bg-violet-900/30 dark:text-violet-300">
        <Icon name="clock" size="md" />
      </div>
      <div>
        <p class="stat-label">{{ t('usage.avgDuration') }}</p>
        <p class="stat-value">{{ formatDuration(stats?.average_duration_ms || 0) }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import Icon from '@/components/icons/Icon.vue'
import { formatTokenCount } from '@/utils/format'

defineProps<{ stats: AdminUsageStatsResponse | null }>()

const { t } = useI18n()

const formatDuration = (ms: number) =>
  ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`

const formatTokens = (value: number) => {
  return formatTokenCount(value)
}

const cacheLabel = () => t('usage.cacheTotal')
const cacheDetailLabel = () => t('usage.cacheBreakdown')
</script>
