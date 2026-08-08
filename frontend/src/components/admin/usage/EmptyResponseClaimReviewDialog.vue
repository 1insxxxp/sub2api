<template>
  <div v-if="show && claim" class="fixed inset-0 z-[80] flex items-end justify-center bg-slate-950/55 sm:items-center sm:p-4" role="dialog" aria-modal="true">
    <section class="max-h-[92vh] w-full overflow-y-auto rounded-t-3xl border border-slate-200 bg-white shadow-2xl dark:border-dark-600 dark:bg-dark-800 sm:max-w-3xl sm:rounded-2xl">
      <header class="flex items-start justify-between border-b border-slate-100 px-5 py-4 dark:border-dark-700">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.16em] text-primary-600 dark:text-primary-400">#{{ claim.id }}</p>
          <h3 class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">{{ action === 'approve' ? t('admin.usage.emptyResponseClaims.approve') : t('admin.usage.emptyResponseClaims.reject') }}</h3>
        </div>
        <button type="button" class="h-9 w-9 rounded-xl text-slate-400 hover:bg-slate-100 dark:hover:bg-dark-700" @click="emit('close')">×</button>
      </header>
      <div class="space-y-4 p-5">
        <div v-if="batchCount > 1" data-testid="batch-review-summary" class="rounded-2xl border border-primary-200 bg-primary-50/70 p-4 text-sm text-primary-800 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200">
          <p class="font-semibold">{{ t('admin.usage.emptyResponseClaims.batchSelected', { count: batchCount }) }}</p>
          <p class="mt-1">{{ t('admin.usage.emptyResponseClaims.batchEstimatedRefund', { amount: batchEstimatedRefund.toFixed(6) }) }}</p>
        </div>
        <div data-testid="claim-review-context" class="space-y-4">
          <div class="rounded-2xl bg-slate-50 p-4 text-sm dark:bg-dark-900/60">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <p class="break-all font-mono font-semibold text-slate-900 dark:text-white">{{ claim.model }}</p>
              <span class="rounded-full bg-slate-200 px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-slate-600 dark:bg-dark-700 dark:text-slate-300">{{ sourceLabel }}</span>
            </div>
            <div class="mt-2 flex flex-wrap justify-between gap-2 text-slate-500 dark:text-slate-400">
              <span>{{ claim.user_email }} · {{ claim.group_name }} · {{ claim.account_name }}</span>
              <span class="font-mono">${{ claim.estimated_refund.toFixed(6) }}</span>
            </div>
          </div>
          <div v-if="claim.reason_code === 'missing_evidence'" class="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
            {{ t('admin.usage.emptyResponseClaims.missingEvidenceWarning') }}
          </div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="review-card"><span>{{ t('admin.usage.emptyResponseClaims.request') }}</span><strong class="break-all font-mono">{{ claim.request_id || '—' }}</strong></div>
            <div class="review-card"><span>{{ t('admin.usage.emptyResponseClaims.usageTime') }}</span><strong>{{ formatDate(claim.usage_created_at) }}</strong></div>
            <div class="review-card"><span>{{ t('admin.usage.emptyResponseClaims.tokens') }}</span><strong>{{ claim.input_tokens }} ↓ / {{ claim.output_tokens }} ↑ / {{ claim.cache_read_tokens }} cache</strong></div>
            <div class="review-card"><span>{{ t('admin.usage.emptyResponseClaims.cost') }}</span><strong>${{ claim.actual_cost.toFixed(6) }} · {{ claim.request_type || 'unknown' }} · {{ billingTypeLabel }}</strong></div>
            <div class="review-card"><span>{{ t('admin.usage.emptyResponseClaims.endpoint') }}</span><strong class="break-all">{{ claim.inbound_endpoint || '—' }} → {{ claim.upstream_endpoint || '—' }}</strong></div>
            <div class="review-card"><span>{{ t('admin.usage.emptyResponseClaims.timing') }}</span><strong>{{ claim.duration_ms ?? '—' }} ms · {{ claim.first_token_ms ?? '—' }} ms</strong></div>
          </div>
          <div class="rounded-2xl border border-slate-200 p-4 dark:border-dark-600">
            <p class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">{{ t('admin.usage.emptyResponseClaims.evidence') }}</p>
            <div class="mt-3 grid gap-2 text-sm sm:grid-cols-2">
              <span>{{ t('admin.usage.emptyResponseClaims.httpStatus') }}: {{ claim.evidence.http_status }} / {{ claim.evidence.upstream_status }}</span>
              <span>{{ t('admin.usage.emptyResponseClaims.events') }}: {{ claim.evidence.event_count }} · {{ claim.evidence.output_bytes }} bytes</span>
              <span>{{ t('admin.usage.emptyResponseClaims.outputFlags') }}: {{ outputFlags }}</span>
              <span>{{ t('admin.usage.emptyResponseClaims.disconnect') }}: {{ claim.evidence.disconnect_source || 'none' }} · {{ claim.evidence.upstream_error_kind || 'none' }}</span>
            </div>
          </div>
          <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('admin.usage.emptyResponseClaims.privacyNotice') }}</p>
          <div v-if="claim.user_reason" class="rounded-2xl border border-slate-200 bg-slate-50 p-4 text-sm dark:border-dark-600 dark:bg-dark-900/40">
            <span class="text-xs font-semibold uppercase tracking-wide text-slate-500">{{ t('admin.usage.emptyResponseClaims.userReason') }}</span>
            <p class="mt-1 whitespace-pre-wrap break-words text-slate-700 dark:text-slate-300">{{ claim.user_reason }}</p>
          </div>
        </div>
        <label class="block">
          <span class="mb-2 block text-sm font-medium text-slate-800 dark:text-slate-200">
            {{ action === 'reject' ? t('admin.usage.emptyResponseClaims.rejectionNote') : t('admin.usage.emptyResponseClaims.reviewNote') }}
          </span>
          <textarea v-model="note" rows="4" maxlength="2000" class="input w-full resize-none" />
        </label>
      </div>
      <footer class="flex gap-3 border-t border-slate-100 p-4 dark:border-dark-700">
        <button type="button" class="btn btn-secondary flex-1" :disabled="submitting" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button
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

const props = defineProps<{
  show: boolean
  claim: AdminEmptyResponseClaim | null
  action: 'approve' | 'reject'
  submitting: boolean
  batchCount?: number
  batchEstimatedRefund?: number
}>()
const emit = defineEmits<{ close: []; submit: [note: string] }>()
const { t } = useI18n()
const note = ref('')
const batchCount = computed(() => props.batchCount ?? 1)
const batchEstimatedRefund = computed(() => props.batchEstimatedRefund ?? props.claim?.estimated_refund ?? 0)
const sourceLabel = computed(() => t(`admin.usage.emptyResponseClaims.source.${props.claim?.compensation_source || 'none'}`))
const billingTypeLabel = computed(() => t(`admin.usage.emptyResponseClaims.billingType.${props.claim?.billing_type === 1 ? 'subscription' : 'balance'}`))
const outputFlags = computed(() => {
  if (!props.claim) return '—'
  const flags: string[] = []
  if (props.claim.evidence.has_text) flags.push('text')
  if (props.claim.evidence.has_tool_call) flags.push('tool')
  if (props.claim.evidence.has_reasoning) flags.push('reasoning')
  if (props.claim.evidence.has_media) flags.push('media')
  return flags.length
    ? flags.map((flag) => t(`admin.usage.emptyResponseClaims.output.${flag}`)).join(', ')
    : t('admin.usage.emptyResponseClaims.output.none')
})
const formatDate = (value: string) => value ? new Date(value).toLocaleString() : '—'
watch(() => props.show, (show) => { if (show) note.value = '' })
</script>

<style scoped>
.review-card {
  @apply flex min-w-0 flex-col gap-1 rounded-2xl border border-slate-200 bg-white p-3 text-sm dark:border-dark-600 dark:bg-dark-800;
}
.review-card span {
  @apply text-xs text-slate-500 dark:text-slate-400;
}
.review-card strong {
  @apply font-medium text-slate-800 dark:text-slate-200;
}
</style>
