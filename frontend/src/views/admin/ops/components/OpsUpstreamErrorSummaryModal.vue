<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { opsAPI, type OpsErrorListQueryParams, type OpsUpstreamErrorSummary, type OpsUpstreamErrorSummaryGroup, type OpsUpstreamErrorSummaryModel, type OpsUpstreamErrorSummaryAccount } from '@/api/admin/ops'
import { buildOpsErrorTimeParams } from '../utils/opsErrorParams'

interface Props {
  show: boolean
  timeRange: string
  customStartTime?: string | null
  customEndTime?: string | null
  platform?: string
  groupId?: number | null
  filters?: OpsErrorListQueryParams
}
const props = defineProps<Props>()
const emit = defineEmits<{ (e:'close'): void; (e:'openErrorDetail', id:number): void }>()
const { t } = useI18n()
const loading = ref(false)
const error = ref(false)
const summary = ref<OpsUpstreamErrorSummary | null>(null)
const expandedGroups = ref<Set<string>>(new Set())
const expandedModels = ref<Set<string>>(new Set())
const expandedAccounts = ref<Set<string>>(new Set())

const groupKey = (g: OpsUpstreamErrorSummaryGroup, i:number) => `${g.group_id ?? 'none'}-${i}`
const modelKey = (g: OpsUpstreamErrorSummaryGroup, m: OpsUpstreamErrorSummaryModel, i:number) => `${groupKey(g, i)}:${m.model}`
const accountKey = (g: OpsUpstreamErrorSummaryGroup, m: OpsUpstreamErrorSummaryModel, a: OpsUpstreamErrorSummaryAccount, i:number, j:number) => `${modelKey(g,m,i)}:${a.account_id ?? a.account_name}-${j}`
const isGroupExpanded = (k:string) => expandedGroups.value.has(k)
const isModelExpanded = (k:string) => expandedModels.value.has(k)
const isAccountExpanded = (k:string) => expandedAccounts.value.has(k)
function toggleGroup(key:string) { const next = new Set(expandedGroups.value); next.has(key) ? next.delete(key) : next.add(key); expandedGroups.value = next }
function toggleModel(key:string) { const next = new Set(expandedModels.value); next.has(key) ? next.delete(key) : next.add(key); expandedModels.value = next }
function toggleAccount(key:string) { const next = new Set(expandedAccounts.value); next.has(key) ? next.delete(key) : next.add(key); expandedAccounts.value = next }
function buildParams(): OpsErrorListQueryParams {
  const p: OpsErrorListQueryParams = { ...(props.filters || {}) }
  Object.assign(p, buildOpsErrorTimeParams(props.timeRange, props.customStartTime, props.customEndTime))
  if (props.timeRange === 'custom' && props.customStartTime && props.customEndTime) { p.start_time = props.customStartTime; p.end_time = props.customEndTime; delete p.time_range }
  if (props.platform) p.platform = props.platform
  if (typeof props.groupId === 'number' && props.groupId > 0) p.group_id = props.groupId
  delete p.page; delete p.page_size
  return p
}
async function load() {
  if (!props.show) return
  loading.value = true; error.value = false
  try {
    const data = await opsAPI.getUpstreamErrorSummary(buildParams())
    summary.value = data
    const groups = data.groups || []
    expandedGroups.value = new Set(groups.map((g, i) => groupKey(g, i)))
    expandedModels.value = new Set(groups.flatMap((g, gi) => (g.models || []).map((m) => modelKey(g, m, gi))))
    expandedAccounts.value = new Set(groups.flatMap((g, gi) => (g.models || []).flatMap((m) => (m.accounts || []).map((a, ai) => accountKey(g, m, a, gi, ai)))))
  } catch (e) { console.error('[OpsUpstreamErrorSummaryModal] Failed to load summary', e); error.value = true; summary.value = null }
  finally { loading.value = false }
}
function close() { emit('close') }
function openDetail(id:number) { emit('openErrorDetail', id) }
function nodeId(prefix:string, key:string) { return `ops-summary-${prefix}-${key.replace(/[^a-zA-Z0-9_-]/g, '-')}` }
function formatDate(value:string|null|undefined) { if (!value) return '—'; const d = new Date(value); return Number.isNaN(d.getTime()) ? value : d.toLocaleString() }
watch(() => [props.show, props.timeRange, props.customStartTime, props.customEndTime, props.platform, props.groupId, props.filters] as const, () => { if (props.show) void load() }, { deep: true, immediate: true })
</script>
<template>
  <BaseDialog :show="show" :title="t('admin.ops.errorDetails.summaryTitle')" width="extra-wide" appearance="neutral" @close="close">
    <div class="space-y-3">
      <div v-if="loading" class="flex flex-col items-center justify-center py-16 text-sm text-gray-500 dark:text-gray-400"><div class="mb-3 h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600" />{{ t('admin.ops.errorDetails.summaryLoading') }}</div>
      <div v-else-if="error" class="rounded-xl border border-red-200/70 bg-red-50/70 p-6 text-center text-sm text-red-700 dark:border-red-500/20 dark:bg-red-900/20 dark:text-red-300"><p>{{ t('admin.ops.errorDetails.summaryError') }}</p><button type="button" class="admin-inline-action mt-3 !min-h-9 !px-3" @click="load">{{ t('admin.ops.errorDetails.summaryRetry') }}</button></div>
      <div v-else-if="!summary || !summary.total_errors" class="py-16 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.ops.errorDetails.summaryEmpty') }}</div>
      <template v-else>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
          <div class="admin-form-section !space-y-1 !p-3"><div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.errorDetails.summaryTotalErrors') }}</div><div class="text-lg font-black text-gray-900 dark:text-white">{{ summary.total_errors }}</div></div>
          <div class="admin-form-section !space-y-1 !p-3"><div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.errorDetails.summaryGroups') }}</div><div class="text-lg font-black text-gray-900 dark:text-white">{{ summary.group_count }}</div></div>
          <div class="admin-form-section !space-y-1 !p-3"><div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.errorDetails.summaryLatest') }}</div><div class="break-words text-sm font-semibold text-gray-900 dark:text-white">{{ formatDate(summary.latest_at) }}</div></div>
          <div v-if="summary.groups_truncated" class="col-span-2 rounded-xl border border-amber-200/70 bg-amber-50/70 p-3 text-xs text-amber-800 dark:border-amber-500/20 dark:bg-amber-900/20 dark:text-amber-200 sm:col-span-1">{{ t('admin.ops.errorDetails.summaryTruncated') }}</div>
        </div>
        <div class="space-y-1.5">
          <div v-for="(group, gi) in summary.groups" :key="groupKey(group, gi)" class="admin-list-surface overflow-hidden rounded-lg">
            <button type="button" class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left" :aria-expanded="isGroupExpanded(groupKey(group, gi))" :aria-controls="nodeId('group', groupKey(group, gi))" @click="toggleGroup(groupKey(group, gi))"><span class="min-w-0 break-words text-sm font-bold text-gray-900 dark:text-white">{{ group.group_name }} <span class="ml-1 text-xs font-medium text-gray-500">{{ group.error_count }}</span> <span class="ml-2 text-[10px] font-normal text-gray-400">{{ formatDate(group.latest_at) }}</span></span><span class="text-xs text-gray-500"><span :aria-label="isGroupExpanded(groupKey(group, gi)) ? t('admin.ops.errorDetails.summaryCollapse') : t('admin.ops.errorDetails.summaryExpand')">{{ isGroupExpanded(groupKey(group, gi)) ? '−' : '+' }}</span></span></button>
            <div v-if="isGroupExpanded(groupKey(group, gi))" :id="nodeId('group', groupKey(group, gi))" class="space-y-1.5 border-t border-gray-200/70 p-2 dark:border-dark-700">
              <div v-for="model in group.models" :key="model.model" class="rounded-md border border-gray-200/70 bg-white/50 dark:border-dark-700 dark:bg-white/[0.02]">
                <button type="button" class="flex w-full flex-wrap items-center justify-between gap-x-2 gap-y-1 px-2.5 py-1.5 text-left" :aria-expanded="isModelExpanded(modelKey(group, model, gi))" :aria-controls="nodeId('model', modelKey(group, model, gi))" @click="toggleModel(modelKey(group, model, gi))"><span class="min-w-0 flex-1 break-words text-sm font-semibold text-gray-800 dark:text-gray-100">{{ model.model }} <span class="ml-1 text-xs font-medium text-gray-500">{{ model.error_count }}</span> <span class="ml-2 text-[10px] font-normal text-gray-400">{{ formatDate(model.latest_at) }}</span></span><span class="flex flex-wrap items-center gap-1"> <span v-for="(count, status) in model.status_codes" :key="status" class="rounded-md bg-gray-200/70 px-1.5 py-0.5 font-mono text-[10px] text-gray-500 dark:bg-dark-700">{{ status }} ×{{ count }}</span></span><span class="text-xs text-gray-500"><span :aria-label="isModelExpanded(modelKey(group, model, gi)) ? t('admin.ops.errorDetails.summaryCollapse') : t('admin.ops.errorDetails.summaryExpand')">{{ isModelExpanded(modelKey(group, model, gi)) ? '−' : '+' }}</span></span></button>
                <div v-if="isModelExpanded(modelKey(group, model, gi))" :id="nodeId('model', modelKey(group, model, gi))" class="grid gap-1.5 border-t border-gray-200/60 p-1.5 dark:border-dark-700 md:grid-cols-2">
                  <div v-for="(account, ai) in model.accounts" :key="accountKey(group, model, account, gi, ai)" class="rounded-md border border-gray-200/60 p-1.5 dark:border-dark-700">
                    <button type="button" class="flex w-full items-center justify-between gap-3 text-left" :aria-expanded="isAccountExpanded(accountKey(group, model, account, gi, ai))" :aria-controls="nodeId('account', accountKey(group, model, account, gi, ai))" @click="toggleAccount(accountKey(group, model, account, gi, ai))"><span class="min-w-0 break-words text-sm font-semibold text-gray-800 dark:text-gray-100">{{ account.account_name }} <span class="ml-1 text-xs text-gray-500">{{ account.error_count }}</span> <span class="ml-2 rounded-md bg-gray-200/70 px-1.5 py-0.5 font-mono text-[10px] text-gray-500 dark:bg-dark-700">{{ account.latest_status_code }}</span> <span class="ml-2 text-[10px] font-normal text-gray-400">{{ formatDate(account.latest_at) }}</span></span><span class="text-xs text-gray-500"><span :aria-label="isAccountExpanded(accountKey(group, model, account, gi, ai)) ? t('admin.ops.errorDetails.summaryCollapse') : t('admin.ops.errorDetails.summaryExpand')">{{ isAccountExpanded(accountKey(group, model, account, gi, ai)) ? '−' : '+' }}</span></span></button>
                    <div v-if="isAccountExpanded(accountKey(group, model, account, gi, ai))" :id="nodeId('account', accountKey(group, model, account, gi, ai))" class="mt-1.5 space-y-1.5">
                      <div v-for="reason in account.reasons" :key="`${reason.representative_error_id}-${reason.message}`" class="rounded-md bg-gray-50/80 p-2 text-xs dark:bg-dark-900/50"><div class="flex flex-wrap items-center gap-1.5"><span class="rounded-md bg-gray-200/70 px-1.5 py-0.5 font-mono dark:bg-dark-700">{{ reason.error_type }}</span><span class="rounded-md bg-gray-200/70 px-1.5 py-0.5 font-mono dark:bg-dark-700">{{ reason.status_code }}</span><span class="text-gray-500">×{{ reason.count }}</span><span class="ml-auto text-gray-400">{{ formatDate(reason.latest_at) }}</span></div><div class="mt-1.5 max-h-24 overflow-y-auto break-words whitespace-pre-wrap text-gray-700 dark:text-gray-200">{{ reason.message || t('common.noData') }}</div><button type="button" class="mt-1.5 font-mono text-primary-600 underline underline-offset-2 dark:text-primary-400" @click="openDetail(reason.representative_error_id)">{{ t('admin.ops.errorDetails.summaryOpenError', { id: reason.representative_error_id }) }}</button></div>
                    </div>
                    <div v-if="account.reasons_truncated" class="px-2 text-xs text-gray-500">{{ t('admin.ops.errorDetails.summaryNestedTruncated', { count: account.total_reasons }) }}</div>
                  </div>
                  <div v-if="model.accounts_truncated" class="px-2 text-xs text-gray-500">{{ t('admin.ops.errorDetails.summaryNestedTruncated', { count: model.total_accounts }) }}</div>
                </div>
              </div>
              <div v-if="group.models_truncated" class="px-2 text-xs text-gray-500">{{ t('admin.ops.errorDetails.summaryNestedTruncated', { count: group.total_models }) }}</div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </BaseDialog>
</template>
