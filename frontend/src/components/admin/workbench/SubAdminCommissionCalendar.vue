<template>
  <section
    data-test="sub-admin-commission-calendar"
    class="rounded-lg border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-5"
  >
    <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-base font-semibold text-gray-950 dark:text-white">
          {{ t('adminWorkbench.commission.calendar') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          {{ t('adminWorkbench.commission.monthTotal') }}:
          <span class="font-semibold tabular-nums text-gray-900 dark:text-white">
            ${{ monthActualTotal.toFixed(2) }}
          </span>
          <span class="ml-2 tabular-nums text-emerald-600 dark:text-emerald-300">
            ${{ monthCommissionTotal.toFixed(2) }}
          </span>
        </p>
      </div>
      <label class="block sm:w-44">
        <span class="sr-only">{{ t('adminWorkbench.commission.calendar') }}</span>
        <input v-model="month" type="month" class="input" />
      </label>
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="errorMessage" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300">
      {{ errorMessage }}
    </div>
    <div v-else>
      <div class="grid grid-cols-7 gap-1 text-center text-xs font-medium text-gray-500 dark:text-dark-400 sm:gap-2">
        <div v-for="day in weekdays" :key="day" class="py-1">
          {{ day }}
        </div>
      </div>
      <div class="mt-2 grid grid-cols-7 gap-1 sm:gap-2">
        <button
          v-for="cell in calendarCells"
          :key="cell.key"
          type="button"
          :data-test="cell.date ? `commission-calendar-day-${cell.date}` : undefined"
          class="min-h-[68px] min-w-0 overflow-hidden rounded-md border px-1.5 py-1.5 text-left transition sm:min-h-[88px] sm:rounded-lg sm:px-2 sm:py-2"
          :class="calendarCellClasses(cell)"
          :disabled="!cell.selectable"
          @click="cell.selectable && emit('select-day', cell.date)"
        >
          <span v-if="cell.date" class="sr-only">{{ cell.date }}</span>
          <span class="text-[11px] font-semibold tabular-nums sm:text-xs">{{ cell.label }}</span>
          <template v-if="cell.day">
            <span
              :data-test="`commission-calendar-actual-cost-${cell.date}`"
              class="mt-1.5 block min-w-0 max-w-full truncate text-[11px] font-semibold leading-tight tabular-nums sm:mt-2 sm:text-sm"
              :title="formatCurrency(cell.day.actual_cost)"
            >
              <span class="sm:hidden">{{ formatCompactCurrency(cell.day.actual_cost) }}</span>
              <span class="hidden sm:inline">{{ formatCurrency(cell.day.actual_cost) }}</span>
            </span>
            <span
              :data-test="`commission-calendar-commission-${cell.date}`"
              class="mt-0.5 block min-w-0 max-w-full truncate text-[10px] leading-tight tabular-nums text-emerald-600 dark:text-emerald-300 sm:mt-1 sm:text-xs"
              :title="formatCurrency(cell.day.commission_amount)"
            >
              <span class="sm:hidden">{{ formatCompactCurrency(cell.day.commission_amount) }}</span>
              <span class="hidden sm:inline">{{ formatCurrency(cell.day.commission_amount) }}</span>
            </span>
          </template>
        </button>
      </div>
      <div
        v-if="visibleDayRows.length > 0"
        data-test="commission-mobile-day-list"
        class="mt-4 space-y-2 sm:hidden"
      >
        <button
          v-for="day in visibleDayRows"
          :key="day.date"
          type="button"
          :data-test="`commission-mobile-day-row-${day.date}`"
          class="sm:hidden flex w-full items-center justify-between gap-3 rounded-lg border px-3 py-2 text-left transition"
          :class="mobileDayRowClasses(day)"
          :disabled="!day.enabled"
          @click="day.enabled && emit('select-day', day.date)"
        >
          <span class="min-w-0">
            <span class="block font-mono text-sm font-semibold tabular-nums">{{ day.date }}</span>
            <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">
              {{ t('adminWorkbench.commission.dayDetails') }}
            </span>
          </span>
          <span class="grid shrink-0 grid-cols-2 gap-2 text-right">
            <span class="rounded-md bg-white/80 px-2 py-1 dark:bg-dark-900/60">
              <span class="block text-[10px] leading-tight text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.commission.actualCost') }}
              </span>
              <span class="block font-mono text-xs font-semibold tabular-nums text-gray-950 dark:text-white">
                {{ formatCurrency(day.actual_cost) }}
              </span>
            </span>
            <span class="rounded-md bg-emerald-50 px-2 py-1 dark:bg-emerald-500/10">
              <span class="block text-[10px] leading-tight text-emerald-700 dark:text-emerald-300">
                {{ t('adminWorkbench.commission.commissionAmount') }}
              </span>
              <span class="block font-mono text-xs font-semibold tabular-nums text-emerald-700 dark:text-emerald-300">
                {{ formatCurrency(day.commission_amount) }}
              </span>
            </span>
          </span>
        </button>
      </div>
      <p v-if="days.length === 0" class="mt-4 rounded-lg bg-gray-50 px-3 py-3 text-center text-sm text-gray-500 dark:bg-dark-800/70 dark:text-dark-400">
        {{ t('adminWorkbench.commission.emptyGrants') }}
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SubAdminCommissionCalendarDay } from '@/api/admin'

const emit = defineEmits<{
  (event: 'select-day', date: string): void
}>()

const { t } = useI18n()

const month = ref(formatLocalMonth(new Date()))
const days = ref<SubAdminCommissionCalendarDay[]>([])
const loading = ref(false)
const errorMessage = ref('')

const weekdays = ['日', '一', '二', '三', '四', '五', '六']

function formatLocalMonth(value: Date) {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}`
}

const dayByDate = computed(() => {
  const map = new Map<string, SubAdminCommissionCalendarDay>()
  for (const day of days.value) {
    map.set(day.date, day)
  }
  return map
})

const monthActualTotal = computed(() =>
  days.value.reduce((sum, day) => sum + day.actual_cost, 0)
)
const monthCommissionTotal = computed(() =>
  days.value.reduce((sum, day) => sum + day.commission_amount, 0)
)

const visibleDayRows = computed(() =>
  [...days.value]
    .filter((day) => day.enabled || day.actual_cost > 0 || day.commission_amount > 0)
    .sort((a, b) => a.date.localeCompare(b.date))
)

function formatCurrency(value: number) {
  return `$${value.toFixed(2)}`
}

function formatCompactCurrency(value: number) {
  const abs = Math.abs(value)
  if (abs >= 1_000_000) {
    return `$${(value / 1_000_000).toFixed(1)}M`
  }
  if (abs >= 1_000) {
    return `$${(value / 1_000).toFixed(1)}K`
  }
  return formatCurrency(value)
}

const calendarCells = computed(() => {
  const [year, monthIndex] = month.value.split('-').map(Number)
  if (!year || !monthIndex) {
    return []
  }
  const first = new Date(year, monthIndex - 1, 1)
  const count = new Date(year, monthIndex, 0).getDate()
  const cells: Array<{
    key: string
    date: string
    label: string
    day?: SubAdminCommissionCalendarDay
    selectable: boolean
  }> = []
  for (let i = 0; i < first.getDay(); i += 1) {
    cells.push({ key: `blank-${i}`, date: '', label: '', selectable: false })
  }
  for (let date = 1; date <= count; date += 1) {
    const value = `${year}-${String(monthIndex).padStart(2, '0')}-${String(date).padStart(2, '0')}`
    const day = dayByDate.value.get(value)
    cells.push({
      key: value,
      date: value,
      label: String(date),
      day,
      selectable: Boolean(day?.enabled)
    })
  }
  return cells
})

function calendarCellClasses(cell: { date: string; day?: SubAdminCommissionCalendarDay; selectable: boolean }) {
  if (!cell.date) {
    return 'border-transparent bg-transparent'
  }
  if (!cell.selectable) {
    return 'border-gray-100 bg-gray-50 text-gray-400 dark:border-dark-800 dark:bg-dark-950/50 dark:text-dark-500'
  }
  return 'border-blue-200 bg-blue-50/60 text-blue-950 hover:border-blue-400 hover:bg-blue-100 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-50 dark:hover:border-blue-400/60'
}

function mobileDayRowClasses(day: SubAdminCommissionCalendarDay) {
  if (!day.enabled) {
    return 'border-gray-100 bg-gray-50 text-gray-500 dark:border-dark-800 dark:bg-dark-950/50 dark:text-dark-400'
  }
  return 'border-blue-100 bg-blue-50/70 text-blue-950 active:border-blue-300 active:bg-blue-100 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-50'
}

async function fetchCalendar() {
  loading.value = true
  errorMessage.value = ''
  try {
    days.value = await adminAPI.subAdminCommission.getWorkbenchCalendar({ month: month.value })
  } catch (error: any) {
    errorMessage.value = extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed'))
  } finally {
    loading.value = false
  }
}

watch(month, () => {
  void fetchCalendar()
}, { immediate: true })
</script>
