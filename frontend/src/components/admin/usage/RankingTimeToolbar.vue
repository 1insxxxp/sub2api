<template>
  <div class="border-b border-gray-100 bg-slate-50/70 px-4 py-3 dark:border-dark-700/50 dark:bg-dark-900/30" data-test="ranking-time-toolbar">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex min-w-0 flex-wrap items-center gap-2">
        <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.usage.timeRange.label') }}</span>
        <div class="flex flex-wrap gap-1.5" role="group" :aria-label="t('admin.usage.timeRange.label')">
          <button
            v-for="preset in presets"
            :key="preset.value"
            type="button"
            class="rounded-md border px-2.5 py-1 text-xs font-medium transition-colors"
            :class="isActive(preset.value) ? 'border-primary-500 bg-primary-500 text-white' : 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300'"
            :data-test="`ranking-range-${preset.value}`"
            :aria-pressed="isActive(preset.value)"
            @click="selectPreset(preset.value)"
          >{{ t(preset.label) }}</button>
        </div>
      </div>
      <div class="flex min-w-0 flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <DateRangePicker :start-date="startDate" :end-date="endDate" @change="onCustomRange" />
        <span class="whitespace-nowrap" data-test="ranking-range-value">{{ startDate }} — {{ endDate }}</span>
      </div>
    </div>
    <p class="mt-2 text-[11px] text-gray-400 dark:text-gray-500">{{ t('admin.usage.timeRange.timezone', { timezone }) }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import { getRankingDateRange, type RankingDatePreset } from '@/utils/rankingDateRange'

const props = defineProps<{ startDate: string; endDate: string }>()
const emit = defineEmits<{ (e: 'change', range: { startDate: string; endDate: string; preset: string | null }): void }>()
const { t } = useI18n()
const presets: Array<{ value: RankingDatePreset; label: string }> = [
  { value: 'today', label: 'dates.today' },
  { value: 'yesterday', label: 'dates.yesterday' },
  { value: '7days', label: 'dates.last7Days' },
  { value: '30days', label: 'dates.last30Days' },
]
const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const isActive = (preset: RankingDatePreset) => {
  const range = getRankingDateRange(preset)
  return range.start === props.startDate && range.end === props.endDate
}
const selectPreset = (preset: RankingDatePreset) => {
  const range = getRankingDateRange(preset)
  emit('change', { startDate: range.start, endDate: range.end, preset })
}
const onCustomRange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  if (!range.startDate || !range.endDate || range.endDate < range.startDate) return
  emit('change', range)
}
</script>
