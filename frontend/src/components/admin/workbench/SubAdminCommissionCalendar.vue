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
          class="commission-calendar-compact-cell relative flex min-h-12 min-w-0 items-start overflow-hidden rounded-md border px-2 py-2 text-left transition sm:block sm:min-h-[104px] sm:rounded-lg"
          :class="calendarCellClasses(cell)"
          :disabled="!cell.selectable"
          @click="cell.selectable && selectDay(cell.date)"
        >
          <span v-if="cell.date" class="sr-only">{{ cell.date }}</span>
          <span
            v-if="cell.day && (cell.day.actual_cost > 0 || cell.day.commission_amount > 0)"
            class="absolute bottom-1.5 left-1/2 h-1 w-5 -translate-x-1/2 rounded-full bg-blue-500/70 dark:bg-cyan-300/80 sm:bottom-auto sm:left-auto sm:right-1.5 sm:top-1.5 sm:h-1.5 sm:w-1.5 sm:translate-x-0"
            aria-hidden="true"
          />
          <span class="block text-xs font-semibold tabular-nums sm:text-xs">{{ cell.label }}</span>
          <span
            v-if="cell.day"
            :data-test="`commission-calendar-desktop-amounts-${cell.date}`"
            class="hidden sm:block"
          >
            <span
              :data-test="`commission-calendar-actual-label-${cell.date}`"
              class="mt-1.5 block text-[9px] font-medium leading-none text-blue-700/80 dark:text-blue-200/80 sm:mt-2 sm:text-[10px]"
            >
              {{ t('adminWorkbench.commission.actualCost') }}
            </span>
            <span
              :data-test="`commission-calendar-actual-cost-${cell.date}`"
              class="commission-calendar-primary-amount mt-0.5 block min-w-0 max-w-full whitespace-normal break-words font-mono text-[12px] font-bold leading-tight tracking-normal tabular-nums text-blue-950 [overflow-wrap:anywhere] dark:text-white sm:text-base"
              :title="formatCurrency(cell.day.actual_cost)"
            >
              {{ formatCurrency(cell.day.actual_cost) }}
            </span>
            <span
              class="mt-1 flex min-w-0 items-center gap-1 sm:mt-1.5"
            >
              <span
                :data-test="`commission-calendar-commission-label-${cell.date}`"
                class="min-w-0 text-[9px] font-medium leading-none text-emerald-700 dark:text-emerald-300 sm:text-[10px]"
              >
                {{ t('adminWorkbench.commission.commissionAmount') }}
              </span>
              <span
                :data-test="`commission-calendar-commission-${cell.date}`"
                class="commission-calendar-commission-pill inline-flex min-w-0 max-w-full items-center rounded-full bg-emerald-100 px-1.5 py-0.5 font-mono text-[10px] font-semibold leading-none tracking-normal tabular-nums text-emerald-700 ring-1 ring-emerald-200/80 dark:bg-emerald-500/15 dark:text-emerald-200 dark:ring-emerald-400/20 sm:text-xs"
                :title="formatCurrency(cell.day.commission_amount)"
              >
                {{ formatCurrency(cell.day.commission_amount) }}
              </span>
            </span>
          </span>
        </button>
      </div>
      <div
        v-if="visibleDayRows.length > 0"
        data-test="commission-mobile-day-list"
        class="mt-4 space-y-3 sm:hidden"
      >
        <div
          data-test="commission-mobile-day-strip"
          class="-mx-1 flex gap-2 overflow-x-auto px-1 pb-1 sm:hidden"
        >
          <button
            v-for="day in visibleDayRows"
            :key="day.date"
            type="button"
            :data-test="`commission-mobile-day-chip-${day.date}`"
            class="shrink-0 rounded-full border px-3 py-1.5 text-left transition"
            :class="mobileDayChipClasses(day)"
            :disabled="!day.enabled"
            @click="day.enabled && selectDay(day.date)"
          >
            <span class="block text-xs font-semibold leading-none tabular-nums">
              {{ day.date.slice(-2) }}
            </span>
            <span class="mt-1 block font-mono text-[11px] leading-none tabular-nums">
              {{ formatCurrency(day.actual_cost) }}
            </span>
          </button>
        </div>
        <button
          v-if="selectedMobileDay"
          :key="selectedMobileDay.date"
          type="button"
          data-test="commission-mobile-selected-day-card"
          class="commission-mobile-ledger-card sm:hidden w-full rounded-lg border p-3 text-left transition"
          :class="mobileDayRowClasses(selectedMobileDay)"
          :disabled="!selectedMobileDay.enabled"
          @click="selectedMobileDay.enabled && selectDay(selectedMobileDay.date)"
        >
          <span class="flex min-w-0 items-center justify-between gap-3">
            <span class="min-w-0">
              <span class="block font-mono text-sm font-semibold tabular-nums">{{ selectedMobileDay.date }}</span>
              <span class="mt-1 inline-flex rounded-full bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-500/15 dark:text-blue-200">
                {{ t('adminWorkbench.commission.dayDetails') }}
              </span>
            </span>
            <span
              v-if="selectedMobileDay.actual_cost > 0 || selectedMobileDay.commission_amount > 0"
              class="h-2.5 w-2.5 shrink-0 rounded-full bg-blue-500/80 dark:bg-cyan-300"
              aria-hidden="true"
            />
          </span>
          <span class="mt-3 grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-2">
            <span class="min-w-0 rounded-lg bg-white/85 p-2 ring-1 ring-blue-100 dark:bg-dark-900/60 dark:ring-blue-400/10">
              <span class="block text-[11px] font-medium leading-tight text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.commission.actualCost') }}
              </span>
              <span class="mt-1 block min-w-0 font-mono text-base font-bold leading-tight tabular-nums text-gray-950 [overflow-wrap:anywhere] dark:text-white">
                {{ formatCurrency(selectedMobileDay.actual_cost) }}
              </span>
            </span>
            <span class="min-w-0 rounded-lg bg-emerald-50 p-2 ring-1 ring-emerald-100 dark:bg-emerald-500/10 dark:ring-emerald-400/10">
              <span class="block text-[11px] font-medium leading-tight text-emerald-700 dark:text-emerald-300">
                {{ t('adminWorkbench.commission.commissionAmount') }}
              </span>
              <span class="mt-1 block min-w-0 font-mono text-base font-bold leading-tight tabular-nums text-emerald-700 [overflow-wrap:anywhere] dark:text-emerald-200">
                {{ formatCurrency(selectedMobileDay.commission_amount) }}
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

const selectedMobileDay = computed(() => {
  if (visibleDayRows.value.length === 0) {
    return null
  }
  return (
    visibleDayRows.value.find((day) => day.date === selectedMobileDate.value) ??
    visibleDayRows.value[visibleDayRows.value.length - 1]
  )
})

function formatCurrency(value: number) {
  return `$${value.toFixed(2)}`
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
  return 'border-blue-200 bg-gradient-to-br from-blue-50 via-white to-cyan-50 text-blue-950 shadow-sm shadow-blue-900/5 hover:border-blue-400 dark:border-blue-500/20 dark:from-blue-500/15 dark:via-dark-900 dark:to-cyan-500/10 dark:text-blue-50 dark:hover:border-blue-400/60'
}

function mobileDayRowClasses(day: SubAdminCommissionCalendarDay) {
  if (!day.enabled) {
    return 'border-gray-100 bg-gray-50 text-gray-500 dark:border-dark-800 dark:bg-dark-950/50 dark:text-dark-400'
  }
  return 'border-blue-100 bg-blue-50/70 text-blue-950 active:border-blue-300 active:bg-blue-100 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-50'
}

function mobileDayChipClasses(day: SubAdminCommissionCalendarDay) {
  if (day.date === selectedMobileDay.value?.date) {
    return 'border-blue-300 bg-blue-600 text-white shadow-sm shadow-blue-900/10 dark:border-blue-400/70 dark:bg-blue-500'
  }
  if (!day.enabled) {
    return 'border-gray-100 bg-gray-50 text-gray-400 dark:border-dark-800 dark:bg-dark-950/50 dark:text-dark-500'
  }
  return 'border-blue-100 bg-blue-50 text-blue-800 active:border-blue-300 active:bg-blue-100 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-100'
}

function selectDay(date: string) {
  selectedMobileDate.value = date
  emit('select-day', date)
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
    selectedMobileDate.value = rows[rows.length - 1].date
  }
}, { immediate: true })
</script>
