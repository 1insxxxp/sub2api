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
      <div>
        <p class="stat-label">{{ t('usage.totalTokens') }}</p>
        <p class="stat-value">{{ formatTokens(stats?.total_tokens || 0) }}</p>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('usage.in') }}: {{ formatTokens(stats?.total_input_tokens || 0) }} /
          {{ t('usage.out') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}
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
</script>
