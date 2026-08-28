<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full md:w-80">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.search"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.affiliates.summary.searchPlaceholder')"
              @input="debounceLoad"
            />
          </div>
          <button
            class="btn btn-secondary px-2 md:px-3"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="loadSummaries"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <span class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.affiliates.summary.sortHint') }}
          </span>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="summaries"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="invited_count"
          default-sort-order="desc"
          :sort-storage-key="sortStorageKey"
          @sort="handleSort"
        >
          <template #cell-inviter="{ row }">
            <div class="min-w-0 space-y-0.5 text-left">
              <div class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ row.inviter_id }}</div>
              <div class="max-w-56 truncate text-sm font-medium text-gray-900 dark:text-white">
                {{ row.inviter_email || '-' }}
              </div>
              <div class="max-w-56 truncate text-xs text-gray-500 dark:text-dark-400">
                {{ row.inviter_username || '-' }} · {{ row.aff_code || '-' }}
              </div>
            </div>
          </template>
          <template #cell-invited_count="{ row }">
            <strong class="text-base text-primary-700 dark:text-primary-300">{{ row.invited_count }}</strong>
          </template>
          <template #cell-qualified_invitee_count="{ row }">
            <span class="font-semibold text-emerald-600 dark:text-emerald-400">{{ row.qualified_invitee_count }}</span>
          </template>
          <template #cell-total_rebate="{ row }">
            <AmountText :value="row.total_rebate" strong />
          </template>
          <template #cell-available_quota="{ row }">
            <AmountText :value="row.available_quota" />
          </template>
          <template #cell-transferred_amount="{ row }">
            <AmountText :value="row.transferred_amount" />
          </template>
          <template #cell-rebate_record_count="{ row }">
            <span class="tabular-nums">{{ row.rebate_record_count }}</span>
          </template>
          <template #cell-last_invited_at="{ row }">
            <span class="text-sm text-gray-600 dark:text-dark-300">{{ formatDateTime(row.last_invited_at) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex flex-wrap justify-end gap-2 md:justify-start">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                data-test="view-invite-records"
                @click="openRecords('invites', row.inviter_email)"
              >
                {{ t('admin.affiliates.summary.viewInvites') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                data-test="view-rebate-records"
                @click="openRecords('rebates', row.inviter_email)"
              >
                {{ t('admin.affiliates.summary.viewRebates') }}
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { affiliatesAPI, type AffiliateInviterSummary, type ListAffiliateRecordsParams } from '@/api/admin/affiliates'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDisplayDateTime } from '@/utils/format'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const summaries = ref<AffiliateInviterSummary[]>([])
const filters = reactive({ search: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const sortStorageKey = 'admin-affiliate-summary-table-sort'
const sortableKeys = new Set([
  'inviter',
  'invited_count',
  'qualified_invitee_count',
  'total_rebate',
  'available_quota',
  'transferred_amount',
  'rebate_record_count',
  'last_invited_at',
])

function loadInitialSortState(): { sort_by: string; sort_order: 'asc' | 'desc' } {
  const fallback = { sort_by: 'invited_count', sort_order: 'desc' as 'asc' | 'desc' }
  if (typeof window === 'undefined') return fallback
  try {
    const raw = localStorage.getItem(sortStorageKey)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    if (typeof parsed.key !== 'string' || !sortableKeys.has(parsed.key)) return fallback
    return {
      sort_by: parsed.key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc',
    }
  } catch {
    return fallback
  }
}

const sortState = reactive(loadInitialSortState())
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'inviter', label: t('admin.affiliates.summary.inviter'), sortable: true },
  { key: 'invited_count', label: t('admin.affiliates.summary.invitedCount'), sortable: true },
  { key: 'qualified_invitee_count', label: t('admin.affiliates.summary.qualifiedCount'), sortable: true },
  { key: 'total_rebate', label: t('admin.affiliates.summary.totalRebate'), sortable: true },
  { key: 'available_quota', label: t('admin.affiliates.summary.availableQuota'), sortable: true },
  { key: 'transferred_amount', label: t('admin.affiliates.summary.transferredAmount'), sortable: true },
  { key: 'rebate_record_count', label: t('admin.affiliates.summary.rebateRecordCount'), sortable: true },
  { key: 'last_invited_at', label: t('admin.affiliates.summary.lastInvitedAt'), sortable: true },
  { key: 'actions', label: t('common.actions') },
])

function buildParams(): ListAffiliateRecordsParams {
  return {
    page: pagination.page,
    page_size: pagination.page_size,
    search: filters.search.trim() || undefined,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order,
  }
}

async function loadSummaries() {
  loading.value = true
  try {
    const response = await affiliatesAPI.listInviterSummaries(buildParams())
    summaries.value = response.items || []
    pagination.total = response.total || 0
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => reloadFromFirstPage(), 300)
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadSummaries()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  reloadFromFirstPage()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadSummaries()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  reloadFromFirstPage()
}

function openRecords(type: 'invites' | 'rebates', email: string) {
  void router.push({
    path: `/admin/affiliates/${type}`,
    query: { search: email },
  })
}

function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}

function formatDateTime(value: string | null | undefined): string {
  return value ? formatDisplayDateTime(value) : '-'
}

const AmountText = defineComponent({
  props: {
    value: { type: Number, default: 0 },
    strong: { type: Boolean, default: false },
  },
  setup(props) {
    return () => h('span', {
      class: props.strong
        ? 'font-semibold tabular-nums text-emerald-600 dark:text-emerald-400'
        : 'tabular-nums text-gray-900 dark:text-white',
    }, `$${formatAmount(props.value)}`)
  },
})

onMounted(() => {
  void loadSummaries()
})
</script>
