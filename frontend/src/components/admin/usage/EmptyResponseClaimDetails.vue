<template>
  <div class="space-y-4">
    <div v-if="showSummary" class="rounded-2xl bg-slate-50 p-4 text-sm dark:bg-dark-900/60">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <p class="break-all font-mono font-semibold text-slate-900 dark:text-white">{{ claim.model }}</p>
        <div class="flex flex-wrap justify-end gap-2">
          <span :class="statusClass" class="rounded-full px-2 py-1 text-[10px] font-semibold">{{ statusLabel }}</span>
          <span class="rounded-full bg-slate-200 px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-slate-600 dark:bg-dark-700 dark:text-slate-300">{{ sourceLabel }}</span>
        </div>
      </div>
      <div class="mt-2 flex flex-wrap justify-between gap-2 text-slate-500 dark:text-slate-400">
        <span>{{ claim.user_email }} · {{ claim.group_name }} · {{ claim.account_name }}</span>
        <span class="font-mono">${{ claim.estimated_refund.toFixed(6) }}</span>
      </div>
    </div>
    <div v-if="claim.reason_code === 'missing_evidence'" class="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
      {{ t('admin.usage.emptyResponseClaims.missingEvidenceWarning') }}
    </div>
    <div v-if="claim.reason_code === 'low_output'" class="rounded-2xl border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-900 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-100">
      {{ t('admin.usage.emptyResponseClaims.lowOutputNotice') }}
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
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminEmptyResponseClaim } from '@/api/admin/usage'

const props = withDefaults(defineProps<{
  claim: AdminEmptyResponseClaim
  showSummary?: boolean
}>(), {
  showSummary: true,
})
const { t } = useI18n()
const sourceLabel = computed(() => t(`admin.usage.emptyResponseClaims.source.${props.claim.compensation_source || 'none'}`))
const statusLabel = computed(() => t(`admin.usage.emptyResponseClaims.status.${props.claim.status || 'evaluating'}`))
const statusClass = computed(() => props.claim.status === 'compensated'
  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  : props.claim.status === 'rejected'
    ? 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300'
    : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300')
const billingTypeLabel = computed(() => t(`admin.usage.emptyResponseClaims.billingType.${props.claim.billing_type === 1 ? 'subscription' : 'balance'}`))
const outputFlags = computed(() => {
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
