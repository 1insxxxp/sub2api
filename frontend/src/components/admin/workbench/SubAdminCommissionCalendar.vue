<template>
  <section
    data-test="sub-admin-commission-calendar"
    class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-5"
  >
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-950 dark:text-white">
          {{ t('adminWorkbench.commission.calendar') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.monthTotal') }}</p>
      </div>
      <label class="block w-full sm:w-44 sm:shrink-0">
        <span class="sr-only">{{ t('adminWorkbench.commission.calendar') }}</span>
        <input v-model="month" type="month" class="input" />
      </label>
    </div>

    <div
      data-test="commission-calendar-month-summary"
      class="mt-4 grid grid-cols-1 gap-2 min-[480px]:grid-cols-2 sm:gap-3"
    >
      <div class="min-w-0 rounded-lg border border-blue-100 bg-blue-50/70 px-3 py-2.5 dark:border-blue-500/20 dark:bg-blue-500/10">
        <span class="block text-[11px] font-medium text-blue-700 dark:text-blue-200 sm:text-xs">
          {{ t('adminWorkbench.commission.actualCost') }}
        </span>
        <span class="mt-1 block overflow-x-auto whitespace-nowrap font-mono text-sm font-bold tabular-nums text-blue-950 [scrollbar-width:none] dark:text-white sm:text-lg" :title="formatCurrency(monthActualTotal)">
          {{ formatCurrency(monthActualTotal) }}
        </span>
      </div>
      <div class="min-w-0 rounded-lg border border-emerald-100 bg-emerald-50/70 px-3 py-2.5 dark:border-emerald-500/20 dark:bg-emerald-500/10">
        <span class="block text-[11px] font-medium text-emerald-700 dark:text-emerald-200 sm:text-xs">
          {{ t('adminWorkbench.commission.commissionAmount') }}
        </span>
        <span class="mt-1 block overflow-x-auto whitespace-nowrap font-mono text-sm font-bold tabular-nums text-emerald-700 [scrollbar-width:none] dark:text-emerald-200 sm:text-lg" :title="formatCurrency(monthCommissionTotal)">
          {{ formatCurrency(monthCommissionTotal) }}
        </span>
      </div>
    </div>

    <div v-if="loading" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="errorMessage" class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300">
      {{ errorMessage }}
    </div>
    <div v-else class="mt-5">
      <div class="grid grid-cols-7 gap-1 text-center text-[11px] font-medium text-gray-500 dark:text-dark-400 sm:gap-2 sm:text-xs">
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
          :aria-label="calendarCellAriaLabel(cell)"
          :aria-pressed="cell.selectable ? cell.date === selectedMobileDate : undefined"
          class="commission-calendar-compact-cell relative flex min-h-12 min-w-0 items-start justify-start overflow-hidden rounded-md border p-1.5 text-left transition sm:min-h-[78px] sm:rounded-lg sm:p-2.5"
          :class="calendarCellClasses(cell)"
          :disabled="!cell.selectable"
          @click="cell.selectable && selectDay(cell.date)"
        >
          <span
            v-if="cell.day && (cell.day.actual_cost > 0 || cell.day.commission_amount > 0)"
            class="absolute bottom-1.5 left-1/2 h-1 w-5 -translate-x-1/2 rounded-full bg-blue-500/80 dark:bg-cyan-300/80 sm:bottom-2 sm:left-2.5 sm:w-6 sm:translate-x-0"
            aria-hidden="true"
          />
          <span class="flex min-w-0 flex-col gap-0.5">
            <span class="block text-xs font-semibold tabular-nums sm:text-sm">{{ cell.label }}</span>
            <span
              v-if="cell.day && (cell.day.actual_cost > 0 || cell.day.commission_amount > 0)"
              :data-test="`commission-calendar-day-${cell.date}-amounts`"
              class="mt-0.5 min-w-0 space-y-0.5 text-[9px] font-medium leading-tight tabular-nums sm:text-[11px]"
            >
              <span class="block truncate text-blue-700 dark:text-blue-200" :title="`${t('adminWorkbench.commission.actualCost')} ${formatCurrency(cell.day.actual_cost)}`">
                <span class="mr-0.5 font-normal opacity-75">{{ t('adminWorkbench.commission.actualCostShort') }}</span>{{ compactCurrency(cell.day.actual_cost) }}
              </span>
              <span class="block truncate text-emerald-700 dark:text-emerald-300" :title="`${t('adminWorkbench.commission.commissionAmount')} ${formatCurrency(cell.day.commission_amount)}`">
                <span class="mr-0.5 font-normal opacity-75">{{ t('adminWorkbench.commission.commissionAmountShort') }}</span>{{ compactCurrency(cell.day.commission_amount) }}
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
  (event: 'select-day', date: string, day: SubAdminCommissionCalendarDay): void
}>()

const { t } = useI18n()

const month = ref(formatLocalMonth(new Date()))
const days = ref<SubAdminCommissionCalendarDay[]>([])
const loading = ref(false)
const errorMessage = ref('')
const selectedMobileDate = ref('')

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

function compactCurrency(value: number) {
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000) {
    return `$${trimCompactNumber(value / 1_000_000)}M`
  }
  if (absolute >= 1_000) {
    return `$${trimCompactNumber(value / 1_000)}K`
  }
  return formatCurrency(value)
}

function trimCompactNumber(value: number) {
  return value.toFixed(2).replace(/\.00$/, '').replace(/(\.\d)0$/, '$1')
}

function calendarCellAriaLabel(cell: { date: string; day?: SubAdminCommissionCalendarDay }) {
  if (!cell.date) {
    return undefined
  }
  if (!cell.day) {
    return cell.date
  }
  return [
    cell.date,
    `${t('adminWorkbench.commission.actualCost')} ${formatCurrency(cell.day.actual_cost)}`,
    `${t('adminWorkbench.commission.commissionAmount')} ${formatCurrency(cell.day.commission_amount)}`
  ].join(', ')
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
      selectable: Boolean(day && (day.enabled || day.actual_cost > 0 || day.commission_amount > 0))
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
  if (cell.date === selectedMobileDate.value) {
    return 'border-blue-500 bg-blue-50 text-blue-950 ring-2 ring-blue-500/10 dark:border-blue-400 dark:bg-blue-500/15 dark:text-blue-50'
  }
  return 'border-blue-200 bg-gradient-to-br from-blue-50 via-white to-cyan-50 text-blue-950 shadow-sm shadow-blue-900/5 hover:border-blue-400 dark:border-blue-500/20 dark:from-blue-500/15 dark:via-dark-900 dark:to-cyan-500/10 dark:text-blue-50 dark:hover:border-blue-400/60'
}

function selectDay(date: string) {
  const day = dayByDate.value.get(date)
  if (!day) {
    return
  }
  selectedMobileDate.value = date
  emit('select-day', date, day)
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

watch(visibleDayRows, (rows) => {
  if (rows.length === 0) {
    selectedMobileDate.value = ''
    return
  }
  if (!rows.some((day) => day.date === selectedMobileDate.value)) {
    selectedMobileDate.value = ''
  }
}, { immediate: true })
</script>
