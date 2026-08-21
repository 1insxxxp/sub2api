<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show && log"
        class="fixed inset-0 z-[70] flex items-end justify-center bg-slate-950/55 px-0 pt-12 backdrop-blur-[2px] sm:items-center sm:px-4 sm:py-8"
        role="dialog"
        aria-modal="true"
        :aria-label="t('usage.emptyResponse.title')"
        @click.self="emit('close')"
      >
        <section
          data-testid="empty-response-claim-panel"
          class="max-h-[calc(100dvh-3rem)] w-full overflow-y-auto rounded-t-3xl border border-slate-200 bg-white shadow-2xl max-sm:w-full max-sm:rounded-b-none dark:border-slate-700 dark:bg-dark-800 sm:max-w-lg sm:rounded-2xl"
        >
          <header class="sticky top-0 z-10 flex items-start justify-between gap-4 border-b border-slate-100 bg-white/95 px-5 py-4 backdrop-blur dark:border-dark-700 dark:bg-dark-800/95">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.18em] text-primary-600 dark:text-primary-400">
                {{ t('usage.emptyResponse.eyebrow') }}
              </p>
              <h2 class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">
                {{ t('usage.emptyResponse.title') }}
              </h2>
            </div>
            <button type="button" class="flex h-9 w-9 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-100 dark:hover:bg-dark-700" @click="emit('close')">
              <span aria-hidden="true">×</span>
            </button>
          </header>

          <div class="space-y-5 p-5">
            <div class="rounded-2xl bg-slate-50 p-4 dark:bg-dark-900/60">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('usage.emptyResponse.model') }}</p>
                  <p class="mt-1 break-all font-mono text-sm font-semibold text-slate-900 dark:text-white">{{ log.model }}</p>
                </div>
                <span class="shrink-0 rounded-full bg-amber-100 px-2.5 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">
                  {{ log.compensation_eligibility === 'manual_review' ? t('usage.emptyResponse.manualReview') : t('usage.emptyResponse.autoReview') }}
                </span>
              </div>
              <p class="mt-3 text-xs text-slate-500 dark:text-slate-400">{{ formatDate(log.created_at) }}</p>
            </div>

            <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div class="rounded-2xl border border-slate-200 p-3 dark:border-dark-600">
                <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('usage.emptyResponse.outputTokens') }}</p>
                <p class="mt-1 font-mono text-base font-semibold text-slate-900 dark:text-white">{{ (log.output_tokens ?? 0).toLocaleString() }}</p>
              </div>
              <div class="rounded-2xl border border-slate-200 p-3 dark:border-dark-600">
                <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('usage.emptyResponse.originalCharge') }}</p>
                <p class="mt-1 font-mono text-base font-semibold text-slate-900 dark:text-white">${{ log.actual_cost.toFixed(6) }}</p>
              </div>
              <div class="rounded-2xl border border-emerald-200 bg-emerald-50/70 p-3 dark:border-emerald-500/20 dark:bg-emerald-500/10">
                <p class="text-xs text-emerald-700 dark:text-emerald-300">{{ t('usage.emptyResponse.expectedRefund') }}</p>
                <p class="mt-1 font-mono text-base font-semibold text-emerald-700 dark:text-emerald-300">${{ log.net_actual_cost.toFixed(6) }}</p>
              </div>
            </div>

            <div class="rounded-2xl border border-sky-200 bg-sky-50/70 p-4 text-sm leading-6 text-sky-900 dark:border-sky-500/20 dark:bg-sky-500/10 dark:text-sky-100">
              {{ t('usage.emptyResponse.rules') }}
            </div>

            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-800 dark:text-slate-200">{{ t('usage.emptyResponse.reason') }}</span>
              <textarea
                v-model="reason"
                maxlength="255"
                rows="3"
                class="input min-h-24 w-full resize-none"
                :placeholder="t('usage.emptyResponse.reasonPlaceholder')"
              />
              <span class="mt-1 block text-right text-xs text-slate-400">{{ reason.length }}/255</span>
            </label>
          </div>

          <footer class="sticky bottom-0 flex gap-3 border-t border-slate-100 bg-white/95 p-4 backdrop-blur dark:border-dark-700 dark:bg-dark-800/95">
            <button type="button" class="btn btn-secondary flex-1" :disabled="submitting" @click="emit('close')">
              {{ t('common.cancel') }}
            </button>
            <button
              data-testid="submit-empty-response-claim"
              type="button"
              class="btn btn-primary flex-[1.4]"
              :disabled="submitting"
              @click="emit('submit', reason.trim())"
            >
              {{ submitting ? t('usage.emptyResponse.submitting') : t('usage.emptyResponse.submit') }}
            </button>
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UsageLog } from '@/types'

const props = defineProps<{ show: boolean; log: UsageLog | null; submitting: boolean }>()
const emit = defineEmits<{ close: []; submit: [reason: string] }>()
const { t } = useI18n()
const reason = ref('')

watch(() => props.show, (show) => {
  if (show) reason.value = ''
})

const formatDate = (value: string) => new Date(value).toLocaleString()
</script>
