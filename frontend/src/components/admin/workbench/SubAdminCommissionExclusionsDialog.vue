<template>
  <BaseDialog
    :show="true"
    :title="t('adminWorkbench.commission.exclusions')"
    appearance="neutral"
    width="normal"
    :close-on-escape="!mutating"
    :show-close-button="!mutating"
    @close="close"
  >
    <div class="min-w-0 space-y-5">
      <div class="rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 dark:border-blue-900/40 dark:bg-blue-950/20">
        <p class="text-xs leading-5 text-gray-600 dark:text-dark-300">
        {{ t('adminWorkbench.commission.exclusionsHint') }}
        </p>
      </div>

      <div class="min-w-0 space-y-2.5">
        <div class="flex items-center justify-between gap-3">
          <label class="text-sm font-semibold text-gray-900 dark:text-white" for="commission-exclusion-search">
            {{ t('adminWorkbench.commission.exclusionSearch') }}
          </label>
          <span class="text-[11px] font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">ID / Email</span>
        </div>
        <label class="relative block">
          <span class="sr-only">{{ t('adminWorkbench.commission.exclusionSearch') }}</span>
          <input
            id="commission-exclusion-search"
            v-model="query"
            data-test="commission-exclusions-search"
            class="input h-11 rounded-xl border-gray-200 bg-white pl-4 shadow-sm transition focus:border-blue-400 focus:ring-2 focus:ring-blue-100 dark:border-dark-600 dark:bg-dark-800 dark:focus:border-blue-500 dark:focus:ring-blue-950"
            type="search"
            autocomplete="off"
            :disabled="mutating"
            :placeholder="t('adminWorkbench.commission.exclusionSearchPlaceholder')"
          />
        </label>
        <p v-if="searching" class="py-2 text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
        <p v-else-if="searchError" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ searchError }}</p>
        <ul v-else-if="results.length" class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <li v-for="user in results" :key="user.id" class="flex min-w-0 items-center gap-3 border-b border-gray-100 px-3.5 py-3 last:border-b-0 dark:border-dark-700">
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-xs font-semibold text-blue-600 dark:bg-blue-950/40 dark:text-blue-300">#</div>
            <div class="min-w-0 flex-1 space-y-0.5">
              <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
              <p class="break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400">#{{ user.id }} · {{ t('admin.users.username') }}：{{ user.username?.trim() || '—' }}</p>
              <p v-if="user.notes?.trim()" class="break-words text-[11px] text-gray-400 [overflow-wrap:anywhere] dark:text-dark-500">{{ t('admin.users.notes') }}：{{ user.notes.trim() }}</p>
            </div>
            <button
              type="button"
              :data-test="`commission-exclusion-add-${user.id}`"
              class="btn btn-secondary min-h-9 shrink-0 rounded-lg px-3 text-xs"
              :disabled="mutating || isExcluded(user.id)"
              @click="add(user.id)"
            >
              {{ t(isExcluded(user.id) ? 'adminWorkbench.commission.alreadyExcluded' : 'adminWorkbench.commission.addExclusion') }}
            </button>
          </li>
        </ul>
        <p v-else-if="searched" class="py-2 text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.noData') }}</p>
      </div>

      <section class="min-w-0 border-t border-gray-100 pt-5 dark:border-dark-700">
        <div class="mb-2.5 flex items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('adminWorkbench.commission.excludedCustomers') }}</h3>
          <span v-if="total" class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500 dark:bg-dark-700 dark:text-dark-300">{{ total }}</span>
        </div>
        <div v-if="loading" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="listError" class="space-y-2 py-3">
          <p class="text-sm text-red-600 dark:text-red-400" role="alert">{{ listError }}</p>
          <button type="button" class="btn btn-secondary min-h-11" @click="loadList">{{ t('common.refresh') }}</button>
        </div>
        <p v-else-if="items.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.emptyExclusions') }}</p>
        <ul v-else class="max-h-[360px] overflow-y-auto rounded-xl border border-gray-200 bg-white pr-1 dark:border-dark-700 dark:bg-dark-800">
          <li
            v-for="item in items"
            :key="item.user_id"
            :data-test="`commission-exclusion-${item.user_id}`"
            class="flex min-w-0 items-center gap-3 border-b border-gray-100 px-3.5 py-3 last:border-b-0 dark:border-dark-700"
          >
            <div class="min-w-0 flex-1 space-y-1">
              <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ item.user_email }}</p>
              <p class="break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400">#{{ item.user_id }} · {{ t('admin.users.username') }}：{{ item.username?.trim() || '—' }}</p>
              <p class="break-words text-[11px] text-gray-400 [overflow-wrap:anywhere] dark:text-dark-500">{{ t('admin.users.notes') }}：{{ item.notes?.trim() || '—' }}</p>
              <p class="break-words text-[11px] text-gray-400 [overflow-wrap:anywhere] dark:text-dark-500">
                {{ t('adminWorkbench.commission.exclusionStartedAt') }} · <time :datetime="item.started_at">{{ formatDateTime(item.started_at) }}</time>
              </p>
            </div>
            <button
              type="button"
              :data-test="`commission-exclusion-remove-${item.user_id}`"
              class="btn btn-secondary min-h-9 shrink-0 rounded-lg px-3 text-xs"
              :disabled="mutating"
              @click="remove(item.user_id)"
            >
              {{ t('adminWorkbench.commission.removeExclusion') }}
            </button>
          </li>
        </ul>
        <Pagination
          v-if="!loading && !listError && total > pageSize"
          class="mt-2"
          variant="compact"
          :page="page"
          :page-size="pageSize"
          :total="total"
          :show-page-size-selector="false"
          @update:page="changePage"
        />
      </section>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary min-h-10 rounded-lg px-4" :disabled="mutating" @click="close">{{ t('common.close') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type SubAdminCommissionExclusion } from '@/api/admin'
import type { AdminUser } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'

const emit = defineEmits<{ (event: 'close' | 'changed'): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const items = ref<SubAdminCommissionExclusion[]>([])
const excludedUserIDs = ref<Set<number>>(new Set())
const page = ref(1)
const pageSize = 10
const total = ref(0)
const loading = ref(false)
const listError = ref('')
const mutating = ref(false)
const query = ref('')
const results = ref<AdminUser[]>([])
const searching = ref(false)
const searched = ref(false)
const searchError = ref('')
let disposed = false
let listRequestID = 0
let searchRequestID = 0
let searchTimer: ReturnType<typeof setTimeout> | undefined
let searchController: AbortController | undefined

function isExcluded(userID: number) {
  return excludedUserIDs.value.has(userID) || items.value.some((item) => item.user_id === userID)
}

function close() {
  if (!mutating.value) emit('close')
}

async function loadList() {
  const requestID = ++listRequestID
  loading.value = true
  listError.value = ''
  try {
    const response = await adminAPI.subAdminCommission.listExclusions({ page: page.value, page_size: pageSize })
    if (disposed || requestID !== listRequestID) return
    total.value = response.total
    const lastPage = Math.max(1, Math.ceil(response.total / pageSize))
    if (page.value > lastPage) {
      page.value = lastPage
      await loadList()
      return
    }
    items.value = response.items
    const known = new Set(excludedUserIDs.value)
    response.items.forEach((item) => known.add(item.user_id))
    excludedUserIDs.value = known
  } catch (error: unknown) {
    if (!disposed && requestID === listRequestID) listError.value = extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed'))
  } finally {
    if (requestID === listRequestID) loading.value = false
  }
}

function changePage(value: number) {
  if (mutating.value || value === page.value) return
  page.value = value
  void loadList()
}

function clearSearch() {
  searchRequestID += 1
  clearTimeout(searchTimer)
  searchController?.abort()
  searching.value = false
  searched.value = false
  results.value = []
  searchError.value = ''
}

async function searchUsers(keyword: string, requestID: number) {
  searching.value = true
  const controller = new AbortController()
  searchController = controller
  try {
    let found: AdminUser[] | undefined
    if (/^\d+$/.test(keyword)) {
      const id = Number(keyword)
      if (Number.isSafeInteger(id) && id > 0) {
        try {
          found = [await adminAPI.users.getById(id)]
        } catch (error: unknown) {
          if (typeof error !== 'object' || error === null || !('status' in error) || error.status !== 404) throw error
        }
      }
    }
    if (!found) {
      if (disposed || requestID !== searchRequestID || controller.signal.aborted) return
      const response = await adminAPI.users.list(1, 10, { search: keyword }, { signal: controller.signal })
      found = response.items
    }
    if (!disposed && requestID === searchRequestID) {
      const foundIDs = new Set(found.map((user) => user.id))
      const known = new Set([...excludedUserIDs.value].filter((id) => !foundIDs.has(id)))
      const excluded = foundIDs.size
        ? await adminAPI.subAdminCommission.getExcludedUserIDs([...foundIDs], { signal: controller.signal })
        : []
      excluded.forEach((id) => known.add(id))
      if (disposed || requestID !== searchRequestID || controller.signal.aborted) return
      excludedUserIDs.value = known
      results.value = found
    }
  } catch (error: unknown) {
    if (!disposed && requestID === searchRequestID) searchError.value = extractApiErrorMessage(error, t('adminWorkbench.commission.exclusionSearchFailed'))
  } finally {
    if (!disposed && requestID === searchRequestID) {
      searching.value = false
      searched.value = true
    }
  }
}

watch(query, (value) => {
  clearSearch()
  const keyword = value.trim()
  if (!keyword) return
  const requestID = searchRequestID
  searchTimer = setTimeout(() => { void searchUsers(keyword, requestID) }, 300)
})

async function mutate(action: () => Promise<unknown>) {
  if (mutating.value) return
  mutating.value = true
  try {
    await action()
    if (disposed) return
    query.value = ''
    clearSearch()
    emit('changed')
    appStore.showSuccess(t('adminWorkbench.commission.exclusionsSaved'))
    await loadList()
  } catch (error: unknown) {
    if (!disposed) appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.commission.saveFailed')))
  } finally {
    mutating.value = false
  }
}

function add(userID: number) {
  return mutate(async () => {
    await adminAPI.subAdminCommission.addExclusion({ user_id: userID })
    excludedUserIDs.value = new Set(excludedUserIDs.value).add(userID)
  })
}

function remove(userID: number) {
  return mutate(async () => {
    await adminAPI.subAdminCommission.removeExclusion(userID)
    const known = new Set(excludedUserIDs.value)
    known.delete(userID)
    excludedUserIDs.value = known
  })
}

onMounted(() => { void loadList() })
onBeforeUnmount(() => {
  disposed = true
  listRequestID += 1
  clearSearch()
})
</script>
