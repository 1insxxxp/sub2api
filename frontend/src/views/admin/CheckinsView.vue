<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="item in statsCards"
          :key="item.key"
          class="card rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
              <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-dark-400">
                {{ item.label }}
              </p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ item.value }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ item.meta }}
              </p>
            </div>
            <div
              class="flex h-9 w-9 items-center justify-center rounded-lg"
              :class="item.iconClass"
            >
              <Icon :name="item.icon" size="sm" />
            </div>
          </div>
        </div>
      </div>

      <section class="card rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="border-b border-gray-100 p-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center gap-3">
            <div class="min-w-56 flex-1">
              <input
                v-model="recordFilters.search"
                type="text"
                class="input"
                :placeholder="t('admin.checkins.recordsSearchPlaceholder')"
                @input="handleRecordSearch"
              />
            </div>
            <input
              v-model="recordFilters.date"
              type="date"
              class="input w-44"
              @change="reloadRecordsFromFirstPage"
            />
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="recordsLoading"
              :title="t('common.refresh')"
              @click="loadRecords"
            >
              <Icon name="refresh" size="md" :class="recordsLoading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>

        <DataTable
          :columns="recordColumns"
          :data="records"
          :loading="recordsLoading"
          row-key="id"
          :sticky-actions-column="false"
        >
          <template #cell-user="{ row }">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ userLabel(row) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.checkins.userId', { id: row.user_id }) }}
              </div>
            </div>
          </template>
          <template #cell-reward_amount="{ value }">
            <span class="font-semibold text-emerald-600 dark:text-emerald-400">
              +{{ formatUsd(value) }}
            </span>
          </template>
          <template #cell-balance_before="{ value }">
            {{ formatUsd(value) }}
          </template>
          <template #cell-balance_after="{ value }">
            {{ formatUsd(value) }}
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>
        </DataTable>

        <Pagination
          v-if="recordsPagination.total > 0"
          :page="recordsPagination.page"
          :total="recordsPagination.total"
          :page-size="recordsPagination.page_size"
          @update:page="handleRecordsPageChange"
          @update:pageSize="handleRecordsPageSizeChange"
        />
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(320px,420px),1fr]">
        <div class="card rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="mb-4">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.checkins.addBlacklist') }}
            </h2>
          </div>

          <div class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.checkins.searchUser') }}</label>
              <div class="flex gap-2">
                <input
                  v-model="blacklistUserSearch"
                  data-test="blacklist-user-search"
                  type="text"
                  class="input"
                  :placeholder="t('admin.checkins.userSearchPlaceholder')"
                  @keyup.enter="searchBlacklistUsers"
                />
                <button
                  type="button"
                  data-test="search-blacklist-user"
                  class="btn btn-secondary"
                  :disabled="userSearchLoading"
                  @click="searchBlacklistUsers"
                >
                  <Icon name="search" size="sm" />
                </button>
              </div>
            </div>

            <div
              v-if="userCandidates.length > 0"
              class="max-h-56 overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-700"
            >
              <button
                v-for="candidate in userCandidates"
                :key="candidate.id"
                type="button"
                :data-test="`select-blacklist-user-${candidate.id}`"
                class="flex w-full items-center justify-between gap-3 border-b border-gray-100 px-3 py-2 text-left transition-colors last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700"
                @click="selectBlacklistUser(candidate)"
              >
                <span class="min-w-0">
                  <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">
                    {{ candidate.email }}
                  </span>
                  <span class="block text-xs text-gray-500 dark:text-dark-400">
                    {{ candidate.username || t('admin.checkins.userId', { id: candidate.id }) }}
                  </span>
                </span>
                <Icon v-if="selectedBlacklistUser?.id === candidate.id" name="check" size="sm" class="text-primary-500" />
              </button>
            </div>

            <div
              v-if="selectedBlacklistUser"
              class="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:bg-dark-700 dark:text-gray-200"
            >
              {{ selectedBlacklistUser.email }}
            </div>

            <div>
              <label class="input-label">{{ t('admin.checkins.reason') }}</label>
              <textarea
                v-model="blacklistReason"
                data-test="blacklist-reason"
                rows="3"
                class="input"
                :placeholder="t('admin.checkins.reasonPlaceholder')"
              ></textarea>
            </div>

            <button
              type="button"
              data-test="add-blacklist"
              class="btn btn-primary w-full"
              :disabled="!selectedBlacklistUser || addingBlacklist"
              @click="handleAddBlacklist"
            >
              {{ addingBlacklist ? t('common.saving') : t('admin.checkins.addToBlacklist') }}
            </button>
          </div>
        </div>

        <div class="card rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="flex flex-wrap items-center gap-3">
              <div class="min-w-56 flex-1">
                <input
                  v-model="blacklistFilters.search"
                  type="text"
                  class="input"
                  :placeholder="t('admin.checkins.blacklistSearchPlaceholder')"
                  @input="handleBlacklistSearch"
                />
              </div>
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="blacklistLoading"
                :title="t('common.refresh')"
                @click="loadBlacklist"
              >
                <Icon name="refresh" size="md" :class="blacklistLoading ? 'animate-spin' : ''" />
              </button>
            </div>
          </div>

          <DataTable
            :columns="blacklistColumns"
            :data="blacklist"
            :loading="blacklistLoading"
            row-key="id"
          >
            <template #cell-user="{ row }">
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ userLabel(row) }}
                </div>
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.userId', { id: row.user_id }) }}
                </div>
              </div>
            </template>
            <template #cell-reason="{ value }">
              <span class="text-gray-600 dark:text-gray-300">{{ value || '-' }}</span>
            </template>
            <template #cell-created_at="{ value }">
              <span class="text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
            </template>
            <template #cell-actions="{ row }">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="removingUserId === row.user_id"
                @click="handleRemoveBlacklist(row.user_id)"
              >
                {{ t('admin.checkins.removeBlacklist') }}
              </button>
            </template>
          </DataTable>

          <Pagination
            v-if="blacklistPagination.total > 0"
            :page="blacklistPagination.page"
            :total="blacklistPagination.total"
            :page-size="blacklistPagination.page_size"
            @update:page="handleBlacklistPageChange"
            @update:pageSize="handleBlacklistPageSizeChange"
          />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  AdminCheckinBlacklistEntry,
  AdminCheckinRecord,
  AdminCheckinStats,
} from '@/api/admin'
import type { AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const stats = ref<AdminCheckinStats | null>(null)
const records = ref<AdminCheckinRecord[]>([])
const blacklist = ref<AdminCheckinBlacklistEntry[]>([])
const userCandidates = ref<AdminUser[]>([])
const selectedBlacklistUser = ref<AdminUser | null>(null)

const statsLoading = ref(false)
const recordsLoading = ref(false)
const blacklistLoading = ref(false)
const userSearchLoading = ref(false)
const addingBlacklist = ref(false)
const removingUserId = ref<number | null>(null)

const recordFilters = reactive({
  search: '',
  date: '',
})

const blacklistFilters = reactive({
  search: '',
})

const recordsPagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})

const blacklistPagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})

const blacklistUserSearch = ref('')
const blacklistReason = ref('')
let recordSearchTimeout: ReturnType<typeof setTimeout> | undefined
let blacklistSearchTimeout: ReturnType<typeof setTimeout> | undefined

const statsCards = computed(() => [
  {
    key: 'today',
    label: t('admin.checkins.todayCount'),
    value: stats.value?.today_count ?? 0,
    meta: formatUsd(stats.value?.today_reward_total ?? 0),
    icon: 'calendar' as const,
    iconClass: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-300',
  },
  {
    key: 'seven',
    label: t('admin.checkins.sevenDayCount'),
    value: stats.value?.seven_day_count ?? 0,
    meta: formatUsd(stats.value?.seven_day_reward_total ?? 0),
    icon: 'chart' as const,
    iconClass: 'bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-300',
  },
  {
    key: 'thirty',
    label: t('admin.checkins.thirtyDayCount'),
    value: stats.value?.thirty_day_count ?? 0,
    meta: formatUsd(stats.value?.thirty_day_reward_total ?? 0),
    icon: 'trendingUp' as const,
    iconClass: 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300',
  },
  {
    key: 'blacklist',
    label: t('admin.checkins.activeBlacklist'),
    value: stats.value?.active_blacklist_count ?? 0,
    meta: stats.value?.current_checkin_day || '-',
    icon: 'ban' as const,
    iconClass: 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-300',
  },
])

const recordColumns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.checkins.columns.user') },
  { key: 'checkin_date', label: t('admin.checkins.columns.checkinDate') },
  { key: 'reward_amount', label: t('admin.checkins.columns.reward') },
  { key: 'balance_before', label: t('admin.checkins.columns.balanceBefore') },
  { key: 'balance_after', label: t('admin.checkins.columns.balanceAfter') },
  { key: 'created_at', label: t('admin.checkins.columns.createdAt') },
])

const blacklistColumns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.checkins.columns.user') },
  { key: 'reason', label: t('admin.checkins.columns.reason') },
  { key: 'created_at', label: t('admin.checkins.columns.createdAt') },
  { key: 'actions', label: t('admin.checkins.columns.actions') },
])

function formatUsd(value: number): string {
  const amount = Number.isFinite(value) ? value : 0
  return `$${amount.toFixed(2)}`
}

function userLabel(row: { user_email?: string; username?: string; user_id: number }): string {
  return row.user_email || row.username || t('admin.checkins.userId', { id: row.user_id })
}

async function loadStats() {
  statsLoading.value = true
  try {
    stats.value = await adminAPI.checkins.getStats()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadStats')))
  } finally {
    statsLoading.value = false
  }
}

async function loadRecords() {
  recordsLoading.value = true
  try {
    const response = await adminAPI.checkins.listRecords(
      recordsPagination.page,
      recordsPagination.page_size,
      {
        search: recordFilters.search.trim() || undefined,
        date: recordFilters.date || undefined,
      }
    )
    records.value = response.items
    recordsPagination.total = response.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadRecords')))
  } finally {
    recordsLoading.value = false
  }
}

async function loadBlacklist() {
  blacklistLoading.value = true
  try {
    const response = await adminAPI.checkins.listBlacklist(
      blacklistPagination.page,
      blacklistPagination.page_size,
      {
        active_only: true,
        search: blacklistFilters.search.trim() || undefined,
      }
    )
    blacklist.value = response.items
    blacklistPagination.total = response.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadBlacklist')))
  } finally {
    blacklistLoading.value = false
  }
}

function reloadRecordsFromFirstPage() {
  recordsPagination.page = 1
  void loadRecords()
}

function handleRecordSearch() {
  clearTimeout(recordSearchTimeout)
  recordSearchTimeout = setTimeout(reloadRecordsFromFirstPage, 300)
}

function handleBlacklistSearch() {
  clearTimeout(blacklistSearchTimeout)
  blacklistSearchTimeout = setTimeout(() => {
    blacklistPagination.page = 1
    void loadBlacklist()
  }, 300)
}

function handleRecordsPageChange(page: number) {
  recordsPagination.page = page
  void loadRecords()
}

function handleRecordsPageSizeChange(pageSize: number) {
  recordsPagination.page_size = pageSize
  recordsPagination.page = 1
  void loadRecords()
}

function handleBlacklistPageChange(page: number) {
  blacklistPagination.page = page
  void loadBlacklist()
}

function handleBlacklistPageSizeChange(pageSize: number) {
  blacklistPagination.page_size = pageSize
  blacklistPagination.page = 1
  void loadBlacklist()
}

async function searchBlacklistUsers() {
  const search = blacklistUserSearch.value.trim()
  if (!search) {
    appStore.showError(t('admin.checkins.userSearchRequired'))
    return
  }
  userSearchLoading.value = true
  try {
    const response = await adminAPI.users.list(1, 8, { search })
    userCandidates.value = response.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToSearchUsers')))
  } finally {
    userSearchLoading.value = false
  }
}

function selectBlacklistUser(user: AdminUser) {
  selectedBlacklistUser.value = user
}

async function handleAddBlacklist() {
  if (!selectedBlacklistUser.value) {
    appStore.showError(t('admin.checkins.selectUserRequired'))
    return
  }
  addingBlacklist.value = true
  try {
    await adminAPI.checkins.addBlacklist({
      user_id: selectedBlacklistUser.value.id,
      reason: blacklistReason.value.trim(),
    })
    appStore.showSuccess(t('admin.checkins.blacklistAdded'))
    selectedBlacklistUser.value = null
    blacklistUserSearch.value = ''
    blacklistReason.value = ''
    userCandidates.value = []
    await Promise.all([loadStats(), loadBlacklist()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToAddBlacklist')))
  } finally {
    addingBlacklist.value = false
  }
}

async function handleRemoveBlacklist(userId: number) {
  removingUserId.value = userId
  try {
    await adminAPI.checkins.removeBlacklist(userId)
    appStore.showSuccess(t('admin.checkins.blacklistRemoved'))
    await Promise.all([loadStats(), loadBlacklist()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToRemoveBlacklist')))
  } finally {
    removingUserId.value = null
  }
}

onMounted(() => {
  void Promise.all([loadStats(), loadRecords(), loadBlacklist()])
})

onUnmounted(() => {
  clearTimeout(recordSearchTimeout)
  clearTimeout(blacklistSearchTimeout)
})
</script>
