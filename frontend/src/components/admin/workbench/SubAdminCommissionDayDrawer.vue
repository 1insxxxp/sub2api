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
      <section class="commission-day-panel overlay-motion-panel flex max-h-[calc(100dvh-2rem)] w-full max-w-3xl min-w-0 flex-col overflow-hidden rounded-lg sm:max-h-[calc(100vh-4rem)]">
        <header class="commission-day-header flex shrink-0 items-start justify-between gap-3 px-4 py-4 sm:px-6">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h2 id="commission-day-dialog-title" class="text-sm font-semibold text-gray-950 dark:text-white">
                {{ t('adminWorkbench.commission.dayDetails') }}
              </h2>
              <p class="text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ date }}</p>
            </div>
            <div v-if="day" class="mt-3 grid min-w-0 grid-cols-1 gap-2 min-[360px]:grid-cols-2">
              <span class="commission-day-summary min-w-0 rounded-md px-3 py-2.5">
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.actualCost') }}</span>
                <span class="mt-1 block overflow-x-auto whitespace-nowrap font-mono text-base font-semibold tabular-nums text-gray-950 [scrollbar-width:none] dark:text-white" :title="formatCurrency(day.actual_cost)">{{ formatCurrency(day.actual_cost) }}</span>
              </span>
              <span class="commission-day-summary min-w-0 rounded-md px-3 py-2.5">
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.commissionAmount') }}</span>
                <span class="mt-1 block overflow-x-auto whitespace-nowrap font-mono text-base font-semibold tabular-nums text-emerald-700 [scrollbar-width:none] dark:text-emerald-300" :title="formatCurrency(day.commission_amount)">{{ formatCurrency(day.commission_amount) }}</span>
              </span>
            </div>
          </div>
          <button
            ref="closeButtonRef"
            type="button"
            data-test="commission-day-dialog-close"
            class="commission-day-close btn btn-secondary shrink-0"
            :aria-label="t('common.close')"
            @click="emit('close')"
          >
            <Icon name="x" size="sm" />
          </button>
        </header>

        <div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6">
          <div v-if="loadingGroups" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="errorMessage" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300">
            {{ errorMessage }}
          </div>
          <div v-else-if="visibleGroups.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.noData') }}
          </div>
          <div v-else class="space-y-3">
            <article
              v-for="group in visibleGroups"
              :key="group.group_id"
              :data-test="`commission-day-group-${group.group_id}`"
              class="commission-day-group-card min-w-0 rounded-lg p-3 sm:p-4"
            >
              <div class="grid min-w-0 gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(16rem,1fr)] sm:items-center">
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
                  <span class="commission-day-amount min-w-0 rounded-md px-2.5 py-2">
                    <span class="block text-[10px] font-medium text-gray-500 dark:text-dark-400">
                      {{ t('adminWorkbench.commission.actualCost') }}
                    </span>
                    <span class="mt-0.5 block overflow-x-auto whitespace-nowrap font-mono text-sm font-semibold tabular-nums text-gray-900 [scrollbar-width:none] dark:text-white" :title="`$${group.actual_cost.toFixed(2)}`">
                      ${{ group.actual_cost.toFixed(2) }}
                    </span>
                  </span>
                  <span class="commission-day-amount min-w-0 rounded-md px-2.5 py-2">
                    <span class="block text-[10px] font-medium text-gray-500 dark:text-dark-400">
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
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { SubAdminCommissionCalendarDay, SubAdminCommissionDayGroup } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  date: string | null
  day: SubAdminCommissionCalendarDay | null
  groupId?: number | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()

const groups = ref<SubAdminCommissionDayGroup[]>([])
const visibleGroups = computed(() => props.groupId == null
  ? groups.value
  : groups.value.filter((group) => group.group_id === props.groupId))
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

<style scoped>
.commission-day-panel {
  border: 1px solid var(--workspace-rule, rgb(148 163 184 / 24%));
  background: var(--workspace-surface, #fff);
  box-shadow: var(--workspace-shadow, inset 0 1px 0 #fff), 0 20px 55px rgb(15 23 42 / 22%);
}

.commission-day-header {
  border-bottom: 1px solid var(--workspace-divider, rgb(148 163 184 / 14%));
  background-image: linear-gradient(132deg, var(--workspace-highlight, rgb(255 255 255 / 82%)), transparent 68%);
}

.commission-day-summary,
.commission-day-group-card {
  border: 1px solid var(--workspace-rule, rgb(148 163 184 / 24%));
  background-color: var(--workspace-control, rgb(255 255 255 / 92%));
  background-image: linear-gradient(132deg, var(--workspace-highlight, rgb(255 255 255 / 82%)), transparent 68%);
  box-shadow: var(--workspace-shadow, inset 0 1px 0 #fff, 0 2px 4px rgb(15 23 42 / 4%));
}

.commission-day-amount {
  background: var(--workspace-hover, rgb(148 163 184 / 7%));
}

.commission-day-close {
  width: 2.75rem;
  min-width: 2.75rem;
  height: 2.75rem;
  min-height: 2.75rem;
  padding: 0;
}
</style>
