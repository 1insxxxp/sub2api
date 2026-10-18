<template>
  <Transition name="overlay">
    <div
      v-if="date"
      ref="dialogRootRef"
      data-test="commission-day-dialog"
      class="fixed inset-0 z-[80] flex items-center justify-center overflow-hidden bg-slate-950/55 p-3 backdrop-blur-[2px] sm:px-4 sm:py-8"
      role="dialog"
      tabindex="-1"
      aria-modal="true"
      aria-labelledby="commission-day-dialog-title"
      @click.self="emit('close')"
    >
      <section class="overlay-motion-panel flex max-h-[calc(100dvh-2rem)] w-full max-w-3xl min-w-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900 sm:max-h-[calc(100vh-4rem)]">
      <header class="flex shrink-0 items-start justify-between gap-3 border-b border-gray-100 bg-gradient-to-br from-blue-50 via-white to-cyan-50 px-4 py-4 dark:border-dark-700 dark:from-blue-500/15 dark:via-dark-900 dark:to-cyan-500/10 sm:px-6">
        <div class="min-w-0">
          <h2 id="commission-day-dialog-title" class="text-base font-semibold text-gray-950 dark:text-white">
            {{ t('adminWorkbench.commission.dayDetails') }}
          </h2>
          <p class="mt-1 font-mono text-sm tabular-nums text-gray-500 dark:text-dark-400">{{ date }}</p>
          <div v-if="day" class="mt-3 grid grid-cols-1 gap-2 min-[360px]:grid-cols-2">
            <span class="min-w-0 rounded-md bg-white/80 px-2.5 py-2 dark:bg-dark-900/70">
              <span class="block text-[10px] font-medium text-blue-700 dark:text-blue-200">{{ t('adminWorkbench.commission.actualCost') }}</span>
              <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-bold tabular-nums text-blue-950 [scrollbar-width:none] dark:text-white" :title="formatCurrency(day.actual_cost)">{{ formatCurrency(day.actual_cost) }}</span>
            </span>
            <span class="min-w-0 rounded-md bg-emerald-50/90 px-2.5 py-2 dark:bg-emerald-500/10">
              <span class="block text-[10px] font-medium text-emerald-700 dark:text-emerald-300">{{ t('adminWorkbench.commission.commissionAmount') }}</span>
              <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-bold tabular-nums text-emerald-700 [scrollbar-width:none] dark:text-emerald-200" :title="formatCurrency(day.commission_amount)">{{ formatCurrency(day.commission_amount) }}</span>
            </span>
          </div>
        </div>
        <button
          ref="closeButtonRef"
          type="button"
          data-test="commission-day-dialog-close"
          class="btn btn-secondary shrink-0 px-3"
          :aria-label="t('common.close')"
          @click="emit('close')"
        >
          <Icon name="x" size="sm" />
        </button>
      </header>

      <div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6 sm:py-5">
      <div v-if="loadingGroups" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.loading') }}
      </div>
      <div v-else-if="errorMessage" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300">
      {{ errorMessage }}
      </div>
      <div v-else-if="groups.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('common.noData') }}
      </div>
      <div v-else class="space-y-3">
      <article
        v-for="group in groups"
        :key="group.group_id"
        :data-test="`commission-day-group-${group.group_id}`"
        class="commission-day-group-card min-w-0 rounded-lg border border-gray-200 p-3 dark:border-dark-700 sm:p-4"
      >
        <div class="grid min-w-0 gap-3">
          <div class="min-w-0">
            <h3
              :data-test="`commission-day-group-${group.group_id}-name`"
              class="break-words text-sm font-semibold leading-5 text-gray-950 dark:text-white"
            >
              {{ group.group_name }}
            </h3>
            <p
              :data-test="`commission-day-group-${group.group_id}-metrics`"
              class="mt-1 text-xs tabular-nums text-gray-500 dark:text-dark-400"
            >
              {{ group.requests }} req · {{ group.total_tokens }} tokens
            </p>
          </div>
          <div
            :data-test="`commission-day-group-${group.group_id}-amounts`"
            class="grid min-w-0 grid-cols-1 gap-2 min-[360px]:grid-cols-2"
          >
            <span class="min-w-0 rounded-md bg-gray-50 px-2.5 py-2 dark:bg-dark-800">
              <span class="block text-[10px] font-medium text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.commission.actualCost') }}
              </span>
              <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-semibold tabular-nums text-gray-900 [scrollbar-width:none] dark:text-white" :title="`$${group.actual_cost.toFixed(2)}`">
                ${{ group.actual_cost.toFixed(2) }}
              </span>
            </span>
            <span class="min-w-0 rounded-md bg-emerald-50 px-2.5 py-2 dark:bg-emerald-500/10">
              <span class="block text-[10px] font-medium text-emerald-700 dark:text-emerald-300">
                {{ t('adminWorkbench.commission.commissionAmount') }}
              </span>
              <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-semibold tabular-nums text-emerald-700 [scrollbar-width:none] dark:text-emerald-300" :title="`$${group.commission_amount.toFixed(2)}`">
                ${{ group.commission_amount.toFixed(2) }}
              </span>
            </span>
          </div>
        </div>
      </article>
      </div>
      </div>
      </section>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SubAdminCommissionCalendarDay, SubAdminCommissionDayGroup } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  date: string | null
  day: SubAdminCommissionCalendarDay | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()

const groups = ref<SubAdminCommissionDayGroup[]>([])
const loadingGroups = ref(false)
const errorMessage = ref('')
let groupRequestID = 0
const dialogRootRef = ref<HTMLElement | null>(null)
const closeButtonRef = ref<HTMLButtonElement | null>(null)

let previousBodyOverflow = ''
let bodyScrollLocked = false
let previouslyFocusedElement: HTMLElement | null = null

function formatCurrency(value: number) {
  return `$${value.toFixed(2)}`
}

function getFocusableElements(): HTMLElement[] {
  if (!dialogRootRef.value) {
    return []
  }
  return Array.from(dialogRootRef.value.querySelectorAll<HTMLElement>(
    'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  ))
}

function handleDialogKeydown(event: KeyboardEvent) {
  if (!props.date) {
    return
  }
  if (event.key === 'Escape') {
    emit('close')
    return
  }
  if (event.key !== 'Tab') {
    return
  }

  const focusable = getFocusableElements()
  if (focusable.length === 0) {
    event.preventDefault()
    dialogRootRef.value?.focus()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const activeElement = document.activeElement
  if (!dialogRootRef.value?.contains(activeElement)) {
    event.preventDefault()
    first.focus()
  } else if (event.shiftKey && activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

function syncDialogSideEffects(open: boolean) {
  if (open && !bodyScrollLocked) {
    previouslyFocusedElement = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', handleDialogKeydown)
    bodyScrollLocked = true
    void nextTick(() => (closeButtonRef.value ?? dialogRootRef.value)?.focus())
    return
  }
  if (!open && bodyScrollLocked) {
    document.body.style.overflow = previousBodyOverflow
    document.removeEventListener('keydown', handleDialogKeydown)
    bodyScrollLocked = false
    const focusTarget = previouslyFocusedElement
    previouslyFocusedElement = null
    void nextTick(() => focusTarget?.focus())
  }
}

async function fetchGroups() {
  const requestID = ++groupRequestID
  const date = props.date
  groups.value = []
  errorMessage.value = ''
  loadingGroups.value = Boolean(date)
  if (!date) return
  try {
    const result = await adminAPI.subAdminCommission.getWorkbenchDayGroups(date)
    if (requestID === groupRequestID && props.date === date) groups.value = result
  } catch (error: any) {
    if (requestID === groupRequestID && props.date === date) {
      errorMessage.value = extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed'))
    }
  } finally {
    if (requestID === groupRequestID) loadingGroups.value = false
  }
}

watch(() => props.date, () => { void fetchGroups() }, { immediate: true })

watch(
  () => Boolean(props.date),
  syncDialogSideEffects,
  { immediate: true }
)

onBeforeUnmount(() => {
  groupRequestID += 1
  syncDialogSideEffects(false)
})
</script>
