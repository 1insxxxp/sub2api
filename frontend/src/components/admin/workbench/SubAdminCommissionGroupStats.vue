<template>
  <section class="commission-groups min-w-0 pt-5" :aria-labelledby="headingId">
    <div class="mb-3 flex min-w-0 flex-wrap items-center justify-between gap-3">
      <div class="min-w-0">
        <h3 :id="headingId" class="text-sm font-semibold text-gray-950 dark:text-white">
          {{ t('adminWorkbench.commission.groupStats') }}
          <span v-if="!loading && !error" class="ml-1 text-xs font-normal text-gray-500 dark:text-dark-400">{{ groups.length }}</span>
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.groupStatsHint') }}</p>
      </div>
      <input
        v-model="search"
        data-test="commission-groups-search"
        type="search"
        :aria-label="t('adminWorkbench.commission.searchGroups')"
        :placeholder="t('adminWorkbench.commission.searchGroups')"
        :disabled="loading || Boolean(error)"
        class="input min-h-11 w-full sm:w-64"
      />
    </div>

    <p v-if="loading" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('adminWorkbench.commission.groupsUnavailable') }}</p>
    <p v-else-if="groups.length === 0" data-test="commission-groups-empty" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.groupsEmpty') }}</p>
    <p v-else-if="filteredGroups.length === 0" data-test="commission-groups-no-match" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.groupsNoMatch') }}</p>
    <template v-else>
      <div class="commission-group-grid commission-group-head hidden gap-4 px-3 pb-2 text-xs text-gray-500 dark:text-dark-400 xl:grid" aria-hidden="true">
        <span>{{ t('adminWorkbench.commission.groupName') }}</span>
        <span class="text-right">{{ t('adminWorkbench.commission.actualCost') }}</span>
        <span class="text-right">{{ t('adminWorkbench.commission.commissionAmount') }}</span>
        <span class="text-right">{{ t('adminWorkbench.commission.earningsShare') }}</span>
        <span>{{ t('adminWorkbench.commission.monthTrend') }}</span>
      </div>
      <div class="grid min-w-0 gap-2">
        <button
          v-for="group in pagedGroups"
          :key="group.group_id"
          type="button"
          :data-test="`commission-group-${group.group_id}`"
          :aria-pressed="selectedGroupId === group.group_id"
          :aria-label="`${t('adminWorkbench.commission.viewGroup')} ${group.group_name}, ${t('adminWorkbench.commission.commissionAmount')} ${formatCurrency(group.commission)}, ${t('adminWorkbench.commission.earningsShare')} ${share(group.commission)}`"
          class="commission-group-row commission-group-grid grid min-h-11 min-w-0 grid-cols-3 items-center gap-x-3 gap-y-3 rounded-lg border p-3 text-left"
          :class="selectedGroupId === group.group_id ? 'commission-group-selected' : ''"
          @click="emit('select-group', group.group_id)"
        >
          <span class="col-span-3 min-w-0 xl:col-span-1">
            <span class="block break-words text-sm font-medium text-gray-950 dark:text-white">{{ group.group_name }}</span>
            <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">#{{ group.group_id }}<span v-if="selectedGroupId === group.group_id" class="ml-2 text-blue-600 dark:text-blue-300">{{ t('adminWorkbench.commission.selectedGroup') }}</span></span>
          </span>
          <span class="min-w-0 xl:text-right">
            <span class="mb-1 block text-[11px] text-gray-500 dark:text-dark-400 xl:hidden">{{ t('adminWorkbench.commission.actualCost') }}</span>
            <span class="block break-all text-xs font-medium tabular-nums text-gray-800 dark:text-dark-100 sm:text-sm">{{ formatCurrency(group.actual) }}</span>
          </span>
          <span class="min-w-0 xl:text-right">
            <span class="mb-1 block text-[11px] text-gray-500 dark:text-dark-400 xl:hidden">{{ t('adminWorkbench.commission.commissionAmount') }}</span>
            <span class="block break-all text-xs font-semibold tabular-nums text-emerald-700 dark:text-emerald-300 sm:text-sm">{{ formatCurrency(group.commission) }}</span>
          </span>
          <span class="min-w-0 xl:text-right">
            <span class="mb-1 block text-[11px] text-gray-500 dark:text-dark-400 xl:hidden">{{ t('adminWorkbench.commission.earningsShare') }}</span>
            <span class="block text-xs tabular-nums text-gray-600 dark:text-dark-300 sm:text-sm">{{ share(group.commission) }}</span>
          </span>
          <span class="col-span-3 flex min-w-0 items-center gap-3 xl:col-span-1">
            <span class="shrink-0 text-[11px] text-gray-500 dark:text-dark-400 xl:hidden">{{ t('adminWorkbench.commission.monthTrend') }}</span>
            <svg viewBox="0 0 240 48" class="h-10 min-w-0 flex-1" role="img" :aria-label="`${group.group_name}, ${metricLabel}, ${t('adminWorkbench.commission.monthTrend')}`">
              <path d="M 2 44 H 238" fill="none" stroke="currentColor" class="text-gray-200 dark:text-dark-700" vector-effect="non-scaling-stroke" />
              <polyline :points="sparkline(group.days)" fill="none" :stroke="metric === 'actual' ? '#3b82f6' : '#10b981'" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
            </svg>
          </span>
        </button>
      </div>
      <nav v-if="filteredGroups.length > pageSize" class="mt-3 flex items-center justify-end gap-2 text-xs text-gray-500 dark:text-dark-400" :aria-label="t('adminWorkbench.commission.groupsPagination')">
        <button type="button" data-test="commission-groups-prev" class="btn btn-secondary min-h-11 min-w-11 px-3" :disabled="page === 1" @click="page--">{{ t('adminWorkbench.commission.previousPage') }}</button>
        <span class="tabular-nums">{{ page }} / {{ totalPages }}</span>
        <button type="button" data-test="commission-groups-next" class="btn btn-secondary min-h-11 min-w-11 px-3" :disabled="page === totalPages" @click="page++">{{ t('adminWorkbench.commission.nextPage') }}</button>
      </nav>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubAdminCommissionCalendarDay, SubAdminCommissionMonthlyGroup } from '@/api/admin'

const props = withDefaults(defineProps<{
  groups: SubAdminCommissionMonthlyGroup[]
  month: string
  endDate: string
  totalCommission: number
  metric?: 'commission' | 'actual'
  selectedGroupId?: number | null
  loading?: boolean
  error?: string
}>(), { metric: 'commission', selectedGroupId: null, loading: false, error: '' })
const emit = defineEmits<{ (event: 'select-group', groupId: number): void }>()
const { t } = useI18n()
const headingId = `commission-group-stats-${useId()}`
const search = ref('')
const page = ref(1)
const pageSize = 10
const metricLabel = computed(() => t(`adminWorkbench.commission.${props.metric === 'actual' ? 'actualCost' : 'commissionAmount'}`))
const sortedGroups = computed(() => props.groups.map(group => ({
  ...group,
  actual: group.days.reduce((sum, day) => sum + day.actual_cost, 0),
  commission: group.days.reduce((sum, day) => sum + day.commission_amount, 0)
})).sort((left, right) => right.commission - left.commission || left.group_id - right.group_id))
const filteredGroups = computed(() => {
  const keyword = search.value.trim().toLocaleLowerCase()
  return sortedGroups.value.filter(group => !keyword || group.group_name.toLocaleLowerCase().includes(keyword) || String(group.group_id).includes(keyword))
})
const totalPages = computed(() => Math.max(1, Math.ceil(filteredGroups.value.length / pageSize)))
const pagedGroups = computed(() => filteredGroups.value.slice((page.value - 1) * pageSize, page.value * pageSize))
watch([search, () => props.month, () => props.groups], () => { page.value = 1 })
function formatCurrency(value: number) { return `$${value.toFixed(2)}` }
function share(value: number) { return `${(props.totalCommission > 0 ? value / props.totalCommission * 100 : 0).toFixed(1)}%` }
function sparkline(days: SubAdminCommissionCalendarDay[]) {
  const count = props.endDate.startsWith(`${props.month}-`) ? Number(props.endDate.slice(8, 10)) : 0
  const valuesByDate = new Map(days.map(day => [day.date, props.metric === 'actual' ? day.actual_cost : day.commission_amount]))
  const values = Array.from({ length: count }, (_, index) => valuesByDate.get(`${props.month}-${String(index + 1).padStart(2, '0')}`) ?? 0)
  const max = Math.max(0, ...values)
  return values.map((value, index) => `${2 + index / Math.max(1, count - 1) * 236},${max > 0 ? 44 - value / max * 40 : 44}`).join(' ')
}
</script>

<style scoped>
.commission-groups { border-top: 1px solid var(--workspace-divider); }
.commission-group-row {
  border-color: var(--workspace-rule);
  background: linear-gradient(125deg, var(--workspace-highlight) 15%, transparent 70%), var(--workspace-surface);
  box-shadow: var(--workspace-shadow);
}
.commission-group-row:hover { border-color: rgb(59 130 246 / 40%); }
.commission-group-row:focus-visible { outline: 2px solid var(--workspace-focus); outline-offset: 2px; }
.commission-group-selected { border-color: rgb(59 130 246 / 70%); background: var(--workspace-tint, rgb(59 130 246 / 5%)); }
@media (min-width: 1280px) {
  .commission-group-grid { grid-template-columns: minmax(0, 1.5fr) minmax(0, .9fr) minmax(0, .9fr) minmax(0, .65fr) minmax(0, 1fr); }
}
</style>
