<template>
  <div v-if="show && primaryClaim" class="fixed inset-0 z-[80] flex items-end justify-center bg-slate-950/55 sm:items-center sm:p-4" role="dialog" aria-modal="true">
    <section class="flex max-h-[92vh] w-full flex-col overflow-hidden rounded-t-3xl border border-slate-200 bg-white shadow-2xl dark:border-dark-600 dark:bg-dark-800 sm:max-w-4xl sm:rounded-2xl">
      <header class="flex shrink-0 items-start justify-between border-b border-slate-100 px-5 py-4 dark:border-dark-700">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-primary-600 dark:text-primary-400">
            {{ isBatch ? t('admin.usage.emptyResponseClaims.batchSelected', { count: batchCount }) : `#${primaryClaim.id}` }}
          </p>
          <h3 class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">{{ dialogTitle }}</h3>
        </div>
        <button type="button" class="h-9 w-9 rounded-xl text-slate-400 hover:bg-slate-100 dark:hover:bg-dark-700" @click="emit('close')">×</button>
      </header>
      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 sm:p-5">
        <div v-if="isBatch" data-testid="batch-review-summary" class="rounded-2xl border border-primary-200 bg-primary-50/70 p-4 text-sm text-primary-800 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200">
          <p class="font-semibold">{{ t('admin.usage.emptyResponseClaims.batchSelected', { count: batchCount }) }}</p>
          <p class="mt-1">{{ t('admin.usage.emptyResponseClaims.batchEstimatedRefund', { amount: batchEstimatedRefund.toFixed(6) }) }}</p>
        </div>

        <div v-if="isBatch" class="space-y-2">
          <p class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">{{ t('admin.usage.emptyResponseClaims.batchReviewItems') }}</p>
          <div data-testid="batch-review-list" class="max-h-[58vh] space-y-2 overflow-y-auto overscroll-contain pr-1 sm:max-h-[54vh]">
            <article
              v-for="item in reviewClaims"
              :key="item.id"
              :data-testid="`batch-review-item-${item.id}`"
              class="overflow-hidden rounded-2xl border border-slate-200 bg-white dark:border-dark-600 dark:bg-dark-800"
            >
              <button
                :data-testid="`toggle-batch-review-item-${item.id}`"
                type="button"
                class="w-full px-4 py-3 text-left transition-colors hover:bg-slate-50 dark:hover:bg-dark-700/60"
                :aria-expanded="expandedClaimID === item.id"
                @click="toggleExpanded(item.id)"
              >
                <div class="flex items-start gap-3">
                  <span class="mt-0.5 flex h-6 min-w-6 items-center justify-center rounded-lg bg-primary-50 px-1.5 font-mono text-[10px] font-semibold text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">#{{ item.id }}</span>
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <p class="break-all font-mono text-sm font-semibold text-slate-900 dark:text-white">{{ item.model }}</p>
                      <span class="shrink-0 font-mono text-sm font-semibold text-emerald-600">${{ item.estimated_refund.toFixed(6) }}</span>
                    </div>
                    <p class="mt-1 break-all text-xs text-slate-500 dark:text-slate-400">{{ item.user_email }} · {{ item.group_name }} · {{ item.account_name }}</p>
                    <div class="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400">
                      <span class="rounded-full bg-slate-100 px-2 py-1 dark:bg-dark-700">{{ t(`usage.emptyResponse.reasonCode.${item.reason_code}`) }}</span>
                      <span>{{ evidenceSummary(item) }}</span>
                    </div>
                  </div>
                  <span class="sr-only">{{ t(expandedClaimID === item.id ? 'admin.usage.emptyResponseClaims.collapseDetails' : 'admin.usage.emptyResponseClaims.expandDetails') }}</span>
                  <span class="mt-1 text-sm text-slate-400" aria-hidden="true">{{ expandedClaimID === item.id ? '⌃' : '⌄' }}</span>
                </div>
              </button>
              <div v-if="expandedClaimID === item.id" :data-testid="`batch-review-detail-${item.id}`" class="border-t border-slate-100 bg-slate-50/50 p-4 dark:border-dark-700 dark:bg-dark-900/30">
                <EmptyResponseClaimDetails :claim="item" :show-summary="false" />
              </div>
            </article>
          </div>
        </div>

        <EmptyResponseClaimDetails v-else data-testid="claim-review-context" :claim="primaryClaim" />

        <label v-if="action !== 'view'" class="block">
          <span class="mb-2 block text-sm font-medium text-slate-800 dark:text-slate-200">
            {{ action === 'reject' ? t('admin.usage.emptyResponseClaims.rejectionNote') : t('admin.usage.emptyResponseClaims.reviewNote') }}
          </span>
          <textarea v-model="note" rows="4" maxlength="2000" class="input w-full resize-none" />
        </label>
      </div>
      <footer class="flex shrink-0 gap-3 border-t border-slate-100 p-4 dark:border-dark-700">
        <button v-if="action === 'view'" type="button" class="btn btn-primary w-full" @click="emit('close')">{{ t('admin.usage.emptyResponseClaims.closeDetail') }}</button>
        <button v-else type="button" class="btn btn-secondary flex-1" :disabled="submitting" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button
          v-if="action !== 'view'"
          data-testid="submit-claim-review"
          type="button"
          class="btn flex-[1.4]"
          :class="action === 'approve' ? 'btn-primary' : 'bg-rose-600 text-white hover:bg-rose-700'"
          :disabled="submitting || (action === 'reject' && !note.trim())"
          @click="emit('submit', note.trim())"
        >
          {{ submitting ? t('common.processing') : t('common.confirm') }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminEmptyResponseClaim } from '@/api/admin/usage'
import EmptyResponseClaimDetails from './EmptyResponseClaimDetails.vue'

const props = defineProps<{
  show: boolean
  claim: AdminEmptyResponseClaim | null
  claims?: AdminEmptyResponseClaim[]
  action: 'view' | 'approve' | 'reject'
  submitting: boolean
  batchCount?: number
  batchEstimatedRefund?: number
}>()
const emit = defineEmits<{ close: []; submit: [note: string] }>()
const { t } = useI18n()
const note = ref('')
const expandedClaimID = ref<number | null>(null)
const reviewClaims = computed(() => props.claims?.length ? props.claims : props.claim ? [props.claim] : [])
const primaryClaim = computed(() => reviewClaims.value[0] ?? null)
const isBatch = computed(() => reviewClaims.value.length > 1)
const batchCount = computed(() => props.batchCount ?? reviewClaims.value.length)
const batchEstimatedRefund = computed(() => props.batchEstimatedRefund ?? reviewClaims.value.reduce((sum, item) => sum + item.estimated_refund, 0))
const dialogTitle = computed(() => props.action === 'view'
  ? t('admin.usage.emptyResponseClaims.detailTitle')
  : props.action === 'approve'
    ? t('admin.usage.emptyResponseClaims.approve')
    : t('admin.usage.emptyResponseClaims.reject'))
const toggleExpanded = (id: number) => {
  expandedClaimID.value = expandedClaimID.value === id ? null : id
}
const evidenceSummary = (claim: AdminEmptyResponseClaim) => t('admin.usage.emptyResponseClaims.evidenceSummary', {
  http: claim.evidence.http_status,
  upstream: claim.evidence.upstream_status,
  events: claim.evidence.event_count,
  completion: t(claim.evidence.stream_completed
    ? 'admin.usage.emptyResponseClaims.complete'
    : 'admin.usage.emptyResponseClaims.interrupted'),
})

watch(() => props.show, (show) => {
  if (!show) return
  note.value = ''
  expandedClaimID.value = reviewClaims.value[0]?.id ?? null
})
</script>
