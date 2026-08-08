<template>
  <section class="space-y-4 p-3 sm:p-5">
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <article class="rounded-2xl border border-slate-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
        <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('admin.usage.emptyResponseClaims.rate') }}</p>
        <p class="mt-1 text-2xl font-semibold text-slate-950 dark:text-white">{{ formatPercent(metrics?.empty_response_rate ?? 0) }}</p>
      </article>
      <article class="rounded-2xl border border-slate-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
        <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('admin.usage.emptyResponseClaims.claims') }}</p>
        <p class="mt-1 text-2xl font-semibold text-slate-950 dark:text-white">{{ metrics?.total_claims ?? 0 }}</p>
      </article>
      <article class="rounded-2xl border border-emerald-200 bg-emerald-50/60 p-4 dark:border-emerald-500/20 dark:bg-emerald-500/10">
        <p class="text-xs text-emerald-700 dark:text-emerald-300">{{ t('admin.usage.emptyResponseClaims.refunded') }}</p>
        <p class="mt-1 font-mono text-xl font-semibold text-emerald-700 dark:text-emerald-300">${{ (metrics?.total_refund_amount ?? 0).toFixed(4) }}</p>
      </article>
      <article class="rounded-2xl border border-amber-200 bg-amber-50/60 p-4 dark:border-amber-500/20 dark:bg-amber-500/10">
        <p class="text-xs text-amber-700 dark:text-amber-300">{{ t('admin.usage.emptyResponseClaims.pending') }}</p>
        <p class="mt-1 text-2xl font-semibold text-amber-700 dark:text-amber-300">{{ metrics?.manual_review_claims ?? 0 }}</p>
      </article>
    </div>

    <div v-if="rateWarning" class="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200">
      {{ t('admin.usage.emptyResponseClaims.warning', { rate: formatPercent(metrics?.empty_response_rate ?? 0) }) }}
    </div>

    <div v-if="hasRankings" class="grid gap-3 lg:grid-cols-3">
      <article
        v-for="ranking in rankings"
        :key="ranking.key"
        class="rounded-2xl border border-slate-200 bg-slate-50/70 p-4 dark:border-dark-600 dark:bg-dark-900/40"
      >
        <h3 class="text-sm font-semibold text-slate-800 dark:text-slate-200">{{ ranking.label }}</h3>
        <ol class="mt-3 space-y-2">
          <li v-for="(item, index) in ranking.items.slice(0, 6)" :key="`${ranking.key}-${item.id}-${item.name}`" class="flex items-center gap-3 text-xs">
            <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-white font-semibold text-slate-400 shadow-sm dark:bg-dark-700">{{ index + 1 }}</span>
            <span class="min-w-0 flex-1 truncate text-slate-700 dark:text-slate-300" :title="item.name">{{ item.name }}</span>
            <span
              class="shrink-0"
              :class="item.empty_response_rate >= warningThreshold ? 'font-semibold text-amber-600 dark:text-amber-300' : 'text-slate-500'"
            >
              {{ formatPercent(item.empty_response_rate) }} · {{ item.claims }} · ${{ item.refund_amount.toFixed(4) }}
            </span>
          </li>
        </ol>
      </article>
    </div>

    <div class="flex flex-wrap items-center gap-3 rounded-2xl border border-slate-200 bg-white p-3 dark:border-dark-600 dark:bg-dark-800">
      <select data-testid="claim-status-filter" v-model="status" class="input min-w-44" @change="reload">
        <option value="">{{ t('admin.usage.emptyResponseClaims.allStatuses') }}</option>
        <option value="evaluating">{{ t('admin.usage.emptyResponseClaims.status.evaluating') }}</option>
        <option value="manual_review">{{ t('admin.usage.emptyResponseClaims.status.manual_review') }}</option>
        <option value="approved">{{ t('admin.usage.emptyResponseClaims.status.approved') }}</option>
        <option value="compensated">{{ t('admin.usage.emptyResponseClaims.status.compensated') }}</option>
        <option value="rejected">{{ t('admin.usage.emptyResponseClaims.status.rejected') }}</option>
      </select>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="reload">{{ t('common.refresh') }}</button>
      <div class="ml-auto flex gap-2" v-if="selectedIDs.length">
        <button data-testid="batch-approve-claims" type="button" class="btn btn-primary btn-sm" @click="openBatch('approve')">{{ t('admin.usage.emptyResponseClaims.batchApprove') }}</button>
        <button data-testid="batch-reject-claims" type="button" class="btn btn-secondary btn-sm text-rose-600" @click="openBatch('reject')">{{ t('admin.usage.emptyResponseClaims.batchReject') }}</button>
      </div>
    </div>

    <div v-if="batchResult" data-testid="batch-result" class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm dark:border-dark-600 dark:bg-dark-900/50">
      <p class="font-medium text-slate-800 dark:text-slate-200">{{ t('admin.usage.emptyResponseClaims.batchResult', { succeeded: batchResult.succeeded.length, failed: Object.keys(batchResult.failed).length }) }}</p>
      <ul v-if="Object.keys(batchResult.failed).length" class="mt-2 space-y-1 text-rose-600 dark:text-rose-300">
        <li v-for="(reason, id) in batchResult.failed" :key="id">#{{ id }}: {{ reason }}</li>
      </ul>
    </div>

    <div data-testid="claim-mobile-cards" class="space-y-3 md:hidden">
      <article v-for="claim in claims" :key="claim.id" class="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800">
        <div class="flex items-start gap-3">
          <input type="checkbox" :checked="selected.has(claim.id)" class="mt-1" @change="toggleSelected(claim.id)" />
          <div class="min-w-0 flex-1">
            <div class="flex items-start justify-between gap-3">
              <p class="break-all font-mono text-sm font-semibold text-slate-900 dark:text-white">{{ claim.model }}</p>
              <span :class="statusClass(claim.status)" class="shrink-0 rounded-full px-2 py-1 text-[10px] font-semibold">{{ t(`admin.usage.emptyResponseClaims.status.${claim.status}`) }}</span>
            </div>
            <p class="mt-2 text-xs text-slate-500">{{ claim.user_email }} · {{ claim.group_name }} · {{ claim.account_name }}</p>
            <p class="mt-2 text-xs text-slate-600 dark:text-slate-300">{{ evidenceSummary(claim) }}</p>
            <p class="mt-1 break-all text-xs text-slate-500 dark:text-slate-400">{{ usageSummary(claim) }}</p>
            <p class="mt-1 text-xs text-slate-500">{{ reasonSummary(claim) }}</p>
            <div class="mt-3 flex items-center justify-between">
              <span class="font-mono text-sm font-semibold text-emerald-600">${{ claim.estimated_refund.toFixed(6) }}</span>
              <div v-if="canReview(claim)" class="flex gap-2">
                <button :data-testid="`reject-claim-${claim.id}`" class="btn btn-secondary btn-sm text-rose-600" @click="openReview(claim, 'reject')">{{ t('admin.usage.emptyResponseClaims.reject') }}</button>
                <button :data-testid="`approve-claim-${claim.id}`" class="btn btn-primary btn-sm" @click="openReview(claim, 'approve')">{{ t('admin.usage.emptyResponseClaims.approve') }}</button>
              </div>
            </div>
          </div>
        </div>
      </article>
    </div>

    <div data-testid="claim-desktop-table" class="hidden overflow-x-auto rounded-2xl border border-slate-200 bg-white md:block dark:border-dark-600 dark:bg-dark-800">
      <table class="min-w-full divide-y divide-slate-200 text-sm dark:divide-dark-600">
        <thead class="bg-slate-50 text-left text-xs text-slate-500 dark:bg-dark-900/50 dark:text-slate-400">
          <tr><th class="px-4 py-3"></th><th class="px-4 py-3">{{ t('admin.usage.emptyResponseClaims.identity') }}</th><th class="px-4 py-3">{{ t('usage.model') }}</th><th class="px-4 py-3">{{ t('admin.usage.emptyResponseClaims.evidence') }}</th><th class="px-4 py-3">{{ t('admin.usage.emptyResponseClaims.refund') }}</th><th class="px-4 py-3">{{ t('admin.usage.emptyResponseClaims.statusLabel') }}</th><th class="px-4 py-3 text-right">{{ t('common.actions') }}</th></tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-dark-700">
          <tr v-for="claim in claims" :key="claim.id">
            <td class="px-4 py-3"><input type="checkbox" :checked="selected.has(claim.id)" @change="toggleSelected(claim.id)" /></td>
            <td class="px-4 py-3"><p class="text-slate-900 dark:text-white">{{ claim.user_email }}</p><p class="text-xs text-slate-500">{{ claim.group_name }} · {{ claim.account_name }}</p></td>
            <td class="max-w-56 break-all px-4 py-3 font-mono text-xs">{{ claim.model }}</td>
            <td class="px-4 py-3 text-xs text-slate-600 dark:text-slate-300"><p>{{ evidenceSummary(claim) }}</p><p class="mt-1 break-all text-slate-500">{{ usageSummary(claim) }}</p><p class="mt-1 text-slate-500">{{ reasonSummary(claim) }}</p></td>
            <td class="px-4 py-3 font-mono text-emerald-600">${{ claim.estimated_refund.toFixed(6) }}</td>
            <td class="px-4 py-3"><span :class="statusClass(claim.status)" class="rounded-full px-2 py-1 text-xs font-semibold">{{ t(`admin.usage.emptyResponseClaims.status.${claim.status}`) }}</span></td>
            <td class="px-4 py-3 text-right"><div v-if="canReview(claim)" class="flex justify-end gap-2"><button :data-testid="`reject-claim-${claim.id}`" class="btn btn-secondary btn-sm text-rose-600" @click="openReview(claim, 'reject')">{{ t('admin.usage.emptyResponseClaims.reject') }}</button><button :data-testid="`approve-claim-${claim.id}`" class="btn btn-primary btn-sm" @click="openReview(claim, 'approve')">{{ t('admin.usage.emptyResponseClaims.approve') }}</button></div></td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="!loading && claims.length === 0" class="py-12 text-center text-sm text-slate-500">{{ t('admin.usage.emptyResponseClaims.empty') }}</p>

    <Pagination
      v-if="total > pageSize"
      :page="page"
      :total="total"
      :page-size="pageSize"
      :show-page-size-selector="false"
      @update:page="changePage"
    />

    <EmptyResponseClaimReviewDialog
      :show="Boolean(reviewClaim)"
      :claim="reviewClaim"
      :action="reviewAction"
      :submitting="submitting"
      :batch-count="batchMode ? selectedIDs.length : 1"
      :batch-estimated-refund="batchMode ? selectedEstimatedRefund : reviewClaim?.estimated_refund"
      @close="reviewClaim = null"
      @submit="submitReview"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminUsageAPI } from '@/api/admin/usage'
import type { AdminEmptyResponseClaim, EmptyResponseClaimMetrics } from '@/api/admin/usage'
import { useAppStore } from '@/stores/app'
import Pagination from '@/components/common/Pagination.vue'
import EmptyResponseClaimReviewDialog from './EmptyResponseClaimReviewDialog.vue'

const props = defineProps<{ startDate: string; endDate: string }>()
const { t } = useI18n()
const appStore = useAppStore()
const claims = ref<AdminEmptyResponseClaim[]>([])
const metrics = ref<EmptyResponseClaimMetrics | null>(null)
const status = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const submitting = ref(false)
const selected = ref(new Set<number>())
const reviewClaim = ref<AdminEmptyResponseClaim | null>(null)
const reviewAction = ref<'approve' | 'reject'>('approve')
const batchMode = ref(false)
const batchResult = ref<{ succeeded: number[]; failed: Record<number, string>; claims: AdminEmptyResponseClaim[] } | null>(null)
const selectedIDs = computed(() => Array.from(selected.value))
const selectedClaims = computed(() => claims.value.filter((claim) => selected.value.has(claim.id)))
const selectedEstimatedRefund = computed(() => selectedClaims.value.reduce((sum, claim) => sum + claim.estimated_refund, 0))
const warningThreshold = 0.01
const rateWarning = computed(() => (metrics.value?.empty_response_rate ?? 0) >= warningThreshold)
const rankings = computed(() => [
  { key: 'group', label: t('admin.usage.emptyResponseClaims.rankings.group'), items: metrics.value?.by_group ?? [] },
  { key: 'account', label: t('admin.usage.emptyResponseClaims.rankings.account'), items: metrics.value?.by_account ?? [] },
  { key: 'model', label: t('admin.usage.emptyResponseClaims.rankings.model'), items: metrics.value?.by_model ?? [] },
])
const hasRankings = computed(() => rankings.value.some((ranking) => ranking.items.length > 0))

const load = async () => {
  loading.value = true
  try {
    const [list, summary] = await Promise.all([
      adminUsageAPI.listClaims({ page: page.value, page_size: pageSize, status: status.value || undefined, start_date: props.startDate, end_date: props.endDate }),
      adminUsageAPI.getClaimMetrics({ start_date: props.startDate, end_date: props.endDate }),
    ])
    claims.value = list.items
    total.value = list.total
    metrics.value = summary
    selected.value = new Set()
  } catch (error) {
    console.error('Failed to load empty-response claims:', error)
    appStore.showError(t('admin.usage.emptyResponseClaims.loadFailed'))
  } finally {
    loading.value = false
  }
}
const reload = () => {
  page.value = 1
  void load()
}
const changePage = (value: number) => {
  if (value === page.value || value < 1) return
  page.value = value
  void load()
}
const toggleSelected = (id: number) => {
  const next = new Set(selected.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selected.value = next
}
const canReview = (claim: AdminEmptyResponseClaim) => ['manual_review', 'evaluating', 'approved'].includes(claim.status)
const evidenceSummary = (claim: AdminEmptyResponseClaim) => {
  const e = claim.evidence
  return t('admin.usage.emptyResponseClaims.evidenceSummary', {
    http: e.http_status,
    upstream: e.upstream_status,
    events: e.event_count,
    completion: t(e.stream_completed
      ? 'admin.usage.emptyResponseClaims.complete'
      : 'admin.usage.emptyResponseClaims.interrupted'),
  })
}
const reasonSummary = (claim: AdminEmptyResponseClaim) => t(`usage.emptyResponse.reasonCode.${claim.reason_code}`)
const usageSummary = (claim: AdminEmptyResponseClaim) => `${claim.request_id || '—'} · ${claim.input_tokens}/${claim.output_tokens} tokens · ${claim.inbound_endpoint || '—'}`
const statusClass = (value: string) => value === 'compensated'
  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  : value === 'rejected'
    ? 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300'
    : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
const openReview = (claim: AdminEmptyResponseClaim, action: 'approve' | 'reject') => {
  reviewClaim.value = claim
  reviewAction.value = action
  batchMode.value = false
}
const openBatch = (action: 'approve' | 'reject') => {
  const first = claims.value.find((claim) => selected.value.has(claim.id))
  if (!first) return
  reviewClaim.value = first
  reviewAction.value = action
  batchMode.value = true
}
const submitReview = async (note: string) => {
  if (!reviewClaim.value) return
  submitting.value = true
  try {
    if (batchMode.value) {
      const result = await adminUsageAPI.batchClaims({
        ids: selectedIDs.value,
        action: reviewAction.value === 'approve' ? 'approved' : 'rejected',
        note,
      })
      batchResult.value = result
      if (Object.keys(result.failed).length) {
        appStore.showError(t('admin.usage.emptyResponseClaims.batchPartialFailure'))
      }
      const failedIDs = new Set(Object.keys(result.failed).map(Number))
      reviewClaim.value = null
      await load()
      selected.value = new Set(claims.value.filter((claim) => failedIDs.has(claim.id)).map((claim) => claim.id))
      return
    } else if (reviewAction.value === 'approve') {
      await adminUsageAPI.approveClaim(reviewClaim.value.id, { note })
    } else {
      await adminUsageAPI.rejectClaim(reviewClaim.value.id, { note })
    }
    reviewClaim.value = null
    appStore.showSuccess(t('admin.usage.emptyResponseClaims.reviewSuccess'))
    await load()
  } catch (error) {
    console.error('Failed to review empty-response claim:', error)
    appStore.showError(t('admin.usage.emptyResponseClaims.reviewFailed'))
  } finally {
    submitting.value = false
  }
}
const formatPercent = (value: number) => `${(value * 100).toFixed(2)}%`

watch(() => [props.startDate, props.endDate], reload)
onMounted(reload)
defineExpose({ reload })
</script>
