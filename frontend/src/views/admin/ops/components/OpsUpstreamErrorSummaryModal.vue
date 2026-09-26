<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { opsAPI, type OpsErrorListQueryParams, type OpsUpstreamErrorSummary, type OpsUpstreamErrorSummaryGroup } from '@/api/admin/ops'
import { buildOpsErrorTimeParams } from '../utils/opsErrorParams'

interface Props { show: boolean; timeRange: string; customStartTime?: string | null; customEndTime?: string | null; platform?: string; groupId?: number | null; filters?: OpsErrorListQueryParams }
interface SummaryRow { id: number; model: string; accountId: number | null; account: string; errorType: string; message: string; statusCode: number; count: number; latestAt: string | null }
interface AccountGroup { accountId: number | null; account: string; rows: SummaryRow[]; totalCount: number; latestAt: string | null }
const GROUP_PAGE_SIZE = 10
const props = defineProps<Props>()
const emit = defineEmits<{ (e:'close'): void; (e:'openErrorDetail', id:number): void }>()
const { t } = useI18n()
const loading = ref(false); const error = ref(false); const summary = ref<OpsUpstreamErrorSummary | null>(null)
const groupPages = reactive<Record<string, number>>({})
const collapsedAccounts = reactive<Record<string, boolean>>({})
function buildParams(): OpsErrorListQueryParams { const p: OpsErrorListQueryParams = { ...(props.filters || {}) }; Object.assign(p, buildOpsErrorTimeParams(props.timeRange, props.customStartTime, props.customEndTime)); if (props.timeRange === 'custom' && props.customStartTime && props.customEndTime) { p.start_time = props.customStartTime; p.end_time = props.customEndTime; delete p.time_range } if (props.platform) p.platform = props.platform; if (typeof props.groupId === 'number' && props.groupId > 0) p.group_id = props.groupId; delete p.page; delete p.page_size; return p }
function resetGroupPages() { Object.keys(groupPages).forEach(key => delete groupPages[key]); Object.keys(collapsedAccounts).forEach(key => delete collapsedAccounts[key]) }
async function load() { if (!props.show) return; loading.value = true; error.value = false; try { summary.value = await opsAPI.getUpstreamErrorSummary(buildParams()); resetGroupPages() } catch (e) { console.error('[OpsUpstreamErrorSummaryModal] Failed to load summary', e); error.value = true; summary.value = null; resetGroupPages() } finally { loading.value = false } }
function close() { emit('close') }
function openDetail(id:number) { emit('openErrorDetail', id) }
function formatDate(value:string|null|undefined) { if (!value) return '—'; const d = new Date(value); return Number.isNaN(d.getTime()) ? value : d.toLocaleString() }
function rowsForGroup(group: OpsUpstreamErrorSummaryGroup): SummaryRow[] {
  return (group.models || []).flatMap(model => (model.accounts || []).flatMap(account => (account.reasons || []).map(reason => ({ id: reason.representative_error_id, model: model.model, accountId: account.account_id, account: account.account_name, errorType: reason.error_type, message: reason.message, statusCode: reason.status_code, count: reason.count, latestAt: reason.latest_at })))).sort((a, b) => {
    const aTime = a.latestAt ? Date.parse(a.latestAt) : 0
    const bTime = b.latestAt ? Date.parse(b.latestAt) : 0
    return bTime - aTime || b.id - a.id
  })
}
function groupKey(group: OpsUpstreamErrorSummaryGroup, index: number) { return `${group.group_id ?? 'none'}-${index}` }
function totalPagesForGroup(group: OpsUpstreamErrorSummaryGroup) { return Math.max(1, Math.ceil(rowsForGroup(group).length / GROUP_PAGE_SIZE)) }
function pageForGroup(group: OpsUpstreamErrorSummaryGroup, index: number) {
  const totalPages = totalPagesForGroup(group)
  return Math.min(Math.max(groupPages[groupKey(group, index)] ?? 1, 1), totalPages)
}
function pagedRowsForGroup(group: OpsUpstreamErrorSummaryGroup, index: number) {
  const rows = rowsForGroup(group)
  const page = pageForGroup(group, index)
  return rows.slice((page - 1) * GROUP_PAGE_SIZE, page * GROUP_PAGE_SIZE)
}
function setGroupPage(group: OpsUpstreamErrorSummaryGroup, index: number, page: number) {
  groupPages[groupKey(group, index)] = Math.min(Math.max(page, 1), totalPagesForGroup(group))
}
function accountKey(group: OpsUpstreamErrorSummaryGroup, index: number, accountId: number | null, account: string) { return `${groupKey(group, index)}-${accountId ?? 'unknown'}-${account}` }
function accountGroupsForGroup(group: OpsUpstreamErrorSummaryGroup, index: number): AccountGroup[] {
  const groups = new Map<string, AccountGroup>()
  for (const row of pagedRowsForGroup(group, index)) {
    const account = row.account || '未知账号'
    const key = `${row.accountId ?? 'unknown'}-${account}`
    const current = groups.get(key)
    if (current) {
      current.rows.push(row)
      current.totalCount += row.count
      if (!current.latestAt || (row.latestAt && Date.parse(row.latestAt) > Date.parse(current.latestAt))) current.latestAt = row.latestAt
    } else groups.set(key, { accountId: row.accountId, account, rows: [row], totalCount: row.count, latestAt: row.latestAt })
  }
  return Array.from(groups.values()).sort((a, b) => (b.latestAt ? Date.parse(b.latestAt) : 0) - (a.latestAt ? Date.parse(a.latestAt) : 0))
}
function isAccountCollapsed(group: OpsUpstreamErrorSummaryGroup, index: number, accountGroup: AccountGroup) { return collapsedAccounts[accountKey(group, index, accountGroup.accountId, accountGroup.account)] !== false }
function toggleAccount(group: OpsUpstreamErrorSummaryGroup, index: number, accountGroup: AccountGroup) {
  const key = accountKey(group, index, accountGroup.accountId, accountGroup.account)
  collapsedAccounts[key] = isAccountCollapsed(group, index, accountGroup) ? false : true
}
function statusClass(statusCode: number) { if (statusCode >= 500) return 'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-300'; if (statusCode >= 400) return 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300'; return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300' }
watch(() => [props.show, props.timeRange, props.customStartTime, props.customEndTime, props.platform, props.groupId, props.filters] as const, () => { if (props.show) void load() }, { deep: true, immediate: true })
</script>
<template>
  <BaseDialog :show="show" :title="t('admin.ops.errorDetails.summaryTitle')" width="extra-wide" appearance="neutral" @close="close">
    <div class="space-y-3">
      <div v-if="loading" class="flex flex-col items-center justify-center py-16 text-sm text-gray-500 dark:text-gray-400"><div class="mb-3 h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600" />{{ t('admin.ops.errorDetails.summaryLoading') }}</div>
      <div v-else-if="error" class="rounded-xl border border-red-200/70 bg-red-50/70 p-6 text-center text-sm text-red-700 dark:border-red-500/20 dark:bg-red-900/20 dark:text-red-300"><p>{{ t('admin.ops.errorDetails.summaryError') }}</p><button type="button" class="admin-inline-action mt-3 !min-h-9 !px-3" @click="load">{{ t('admin.ops.errorDetails.summaryRetry') }}</button></div>
      <div v-else-if="!summary || !summary.total_errors" class="py-16 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.ops.errorDetails.summaryEmpty') }}</div>
      <template v-else>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-4"><div class="admin-form-section !space-y-1 !p-3"><div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.errorDetails.summaryTotalErrors') }}</div><div class="text-lg font-black text-gray-900 dark:text-white">{{ summary.total_errors }}</div></div><div class="admin-form-section !space-y-1 !p-3"><div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.errorDetails.summaryGroups') }}</div><div class="text-lg font-black text-gray-900 dark:text-white">{{ summary.group_count }}</div></div><div class="admin-form-section !space-y-1 !p-3"><div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.errorDetails.summaryLatest') }}</div><div class="break-words text-sm font-semibold text-gray-900 dark:text-white">{{ formatDate(summary.latest_at) }}</div></div><div v-if="summary.groups_truncated" class="col-span-2 rounded-xl border border-amber-200/70 bg-amber-50/70 p-3 text-xs text-amber-800 dark:border-amber-500/20 dark:bg-amber-900/20 dark:text-amber-200 sm:col-span-1">{{ t('admin.ops.errorDetails.summaryTruncated') }}</div></div>
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-primary-200/70 bg-primary-50/60 px-3 py-2 text-xs text-primary-800 dark:border-primary-500/20 dark:bg-primary-500/10 dark:text-primary-200"><span class="font-semibold">{{ t('admin.ops.errorDetails.summaryPath') }}</span><span class="text-primary-700/80 dark:text-primary-200/80">{{ t('admin.ops.errorDetails.summaryPathHint') }}</span></div>
        <div class="space-y-3"><section v-for="(group, gi) in summary.groups" :key="`${group.group_id ?? 'none'}-${gi}`" class="admin-list-surface overflow-hidden rounded-lg" :data-testid="`ops-summary-group-${gi}`"><header class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 border-b border-gray-200/70 px-3 py-2.5 dark:border-dark-700"><div class="min-w-0 flex-1"><div class="break-words text-sm font-bold text-gray-900 dark:text-white">{{ group.group_name }}</div><div class="mt-1 flex flex-wrap items-center gap-1.5 text-[10px] text-gray-500 dark:text-gray-400"><span class="rounded-md bg-rose-50 px-1.5 py-0.5 text-rose-700 dark:bg-rose-500/10 dark:text-rose-300">{{ group.error_count }} {{ t('admin.ops.errorDetails.summaryErrorsShort') }}</span><span class="rounded-md bg-gray-100 px-1.5 py-0.5 dark:bg-dark-700">{{ group.model_count }} {{ t('admin.ops.errorDetails.summaryModelsShort') }}</span><span class="rounded-md bg-gray-100 px-1.5 py-0.5 dark:bg-dark-700">{{ group.account_count }} {{ t('admin.ops.errorDetails.summaryAccountsShort') }}</span></div></div><time class="text-[10px] text-gray-400">{{ formatDate(group.latest_at) }}</time></header><div class="space-y-2 px-2 py-2" :data-testid="`ops-summary-table-${gi}`">
              <section v-for="accountGroup in accountGroupsForGroup(group, gi)" :key="`${accountGroup.accountId ?? 'unknown'}-${accountGroup.account}`" class="overflow-hidden rounded-md border border-gray-200/70 dark:border-dark-700">
                <button type="button" class="flex w-full items-center gap-2 bg-gray-50/80 px-3 py-2 text-left text-xs hover:bg-primary-50/50 dark:bg-dark-900/40 dark:hover:bg-primary-500/10" :aria-expanded="!isAccountCollapsed(group, gi, accountGroup)" @click="toggleAccount(group, gi, accountGroup)">
                  <span class="rounded bg-primary-50 px-1.5 py-0.5 text-[10px] font-semibold text-primary-700 dark:bg-primary-500/10 dark:text-primary-300">{{ t('admin.ops.errorDetails.summaryAccount') }}</span>
                  <span class="min-w-0 flex-1 truncate font-semibold text-gray-800 dark:text-gray-100">{{ accountGroup.account }}</span>
                  <span class="whitespace-nowrap text-[10px] text-gray-500 dark:text-gray-400">×{{ accountGroup.totalCount }} {{ t('admin.ops.errorDetails.summaryErrorsShort') }}</span>
                  <span class="w-4 text-center text-sm font-semibold text-gray-400">{{ isAccountCollapsed(group, gi, accountGroup) ? '＋' : '−' }}</span>
                </button>
                <div v-if="!isAccountCollapsed(group, gi, accountGroup)" class="overflow-x-auto">
                  <table class="min-w-[760px] w-full text-xs">
                    <thead class="bg-white/80 text-left text-[10px] font-semibold uppercase tracking-wide text-gray-500 dark:bg-dark-900/20 dark:text-gray-400"><tr><th class="w-[24%] px-3 py-2">{{ t('admin.ops.errorDetails.summaryModel') }}</th><th class="w-[34%] px-3 py-2">{{ t('admin.ops.errorDetails.summaryReason') }}</th><th class="px-3 py-2">{{ t('admin.ops.errorDetails.summaryStatus') }}</th><th class="px-3 py-2">{{ t('admin.ops.errorDetails.summaryCount') }}</th><th class="px-3 py-2">{{ t('admin.ops.errorDetails.summaryLatestTime') }}</th><th class="px-3 py-2 text-right">{{ t('admin.ops.errorDetails.summaryAction') }}</th></tr></thead>
                    <tbody class="divide-y divide-gray-200/70 dark:divide-dark-700">
                      <tr v-for="row in accountGroup.rows" :key="`${row.id}-${row.model}-${row.account}`" class="align-top hover:bg-primary-50/30 dark:hover:bg-primary-500/5" :data-testid="`ops-summary-row-${row.id}`"><td class="max-w-[180px] break-words px-3 py-2.5 font-medium text-gray-800 dark:text-gray-100">{{ row.model }}</td><td class="max-w-[360px] px-3 py-2.5"><span class="block max-w-[360px] truncate font-medium text-gray-700 dark:text-gray-200" :title="row.message">{{ row.message || t('common.noData') }}</span><span class="mt-1 inline-flex rounded-md bg-gray-100 px-1.5 py-0.5 font-mono text-[10px] text-gray-500 dark:bg-dark-700 dark:text-gray-300">{{ row.errorType }}</span></td><td class="whitespace-nowrap px-3 py-2.5"><span class="rounded-md px-1.5 py-0.5 font-mono text-[10px]" :class="statusClass(row.statusCode)">{{ row.statusCode }}</span></td><td class="whitespace-nowrap px-3 py-2.5 font-semibold text-gray-700 dark:text-gray-200">×{{ row.count }}</td><td class="whitespace-nowrap px-3 py-2.5 text-gray-500 dark:text-gray-400">{{ formatDate(row.latestAt) }}</td><td class="whitespace-nowrap px-3 py-2.5 text-right"><button type="button" class="font-medium text-primary-600 hover:underline dark:text-primary-400" @click="openDetail(row.id)">{{ t('admin.ops.errorDetails.summaryAction') }}</button></td></tr>
                    </tbody>
                  </table>
                </div>
              </section>
              <div v-if="!rowsForGroup(group).length" class="px-3 py-6 text-center text-gray-500">{{ t('common.noData') }}</div>
            </div><div v-if="totalPagesForGroup(group) > 1" :data-testid="`ops-summary-pagination-${gi}`" class="ops-summary-pagination border-t border-gray-200/70 bg-gray-50/50 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-900/20"><Pagination :total="rowsForGroup(group).length" :page="pageForGroup(group, gi)" :page-size="GROUP_PAGE_SIZE" :show-page-size-selector="false" variant="compact" @update:page="setGroupPage(group, gi, $event)" /></div><div v-if="group.models_truncated" class="border-t border-gray-200/70 px-3 py-2 text-xs text-gray-500 dark:border-dark-700">{{ t('admin.ops.errorDetails.summaryNestedTruncated', { count: group.total_models }) }}</div></section></div>
      </template>
    </div>
  </BaseDialog>
</template>
