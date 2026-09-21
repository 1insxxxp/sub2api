<template>
  <div class="min-w-0">
    <!-- Loading state -->
    <div v-if="props.loading && !props.stats" class="space-y-0.5">
      <div class="h-3 w-12 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
      <div class="h-3 w-16 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
      <div class="h-3 w-10 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
    </div>

    <!-- Error state -->
    <div v-else-if="props.error && !props.stats" class="text-xs text-red-500">
      {{ props.error }}
    </div>

    <!-- Stats data -->
    <div v-else-if="props.stats" class="space-y-0.5 text-xs">
      <!-- Requests -->
      <div class="flex min-w-0 items-center gap-1 whitespace-nowrap">
        <span class="shrink-0 text-gray-500 dark:text-gray-400"
          >{{ t('admin.accounts.stats.requests') }}:</span
        >
        <span class="shrink-0 whitespace-nowrap font-medium text-gray-700 dark:text-gray-300">{{
          formatNumber(props.stats.requests)
        }}</span>
      </div>
      <!-- Tokens -->
      <div class="flex min-w-0 items-center gap-1 whitespace-nowrap">
        <span class="shrink-0 text-gray-500 dark:text-gray-400"
          >{{ t('admin.accounts.stats.tokens') }}:</span
        >
        <span class="shrink-0 whitespace-nowrap font-medium text-gray-700 dark:text-gray-300">{{
          formatTokens(props.stats.tokens)
        }}</span>
      </div>
      <!-- Cost (Account) -->
      <div class="flex min-w-0 items-center gap-1 whitespace-nowrap">
        <span class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('usage.accountBilled') }}:</span>
        <span class="shrink-0 whitespace-nowrap font-medium text-emerald-600 dark:text-emerald-400">{{
          formatCurrency(props.stats.cost)
        }}</span>
      </div>
      <!-- Cost (User/API Key) -->
      <div v-if="props.stats.user_cost != null" class="flex min-w-0 items-center gap-1 whitespace-nowrap">
        <span class="shrink-0 text-gray-500 dark:text-gray-400">{{ t('usage.userBilled') }}:</span>
        <span class="shrink-0 whitespace-nowrap font-medium text-gray-700 dark:text-gray-300">{{
          formatCurrency(props.stats.user_cost)
        }}</span>
      </div>
    </div>

    <!-- No data -->
    <div v-else class="text-xs text-gray-400">-</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { WindowStats } from '@/types'
import { formatNumber, formatCurrency, formatTokenCount } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    stats?: WindowStats | null
    loading?: boolean
    error?: string | null
  }>(),
  {
    stats: null,
    loading: false,
    error: null
  }
)

const { t } = useI18n()

const formatTokens = (tokens: number): string => {
  return formatTokenCount(tokens)
}
</script>
