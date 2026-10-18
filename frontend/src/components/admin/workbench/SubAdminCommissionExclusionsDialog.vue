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
    <div class="min-w-0 space-y-4">
      <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
        {{ t('adminWorkbench.commission.exclusionsHint') }}
      </p>
      <div class="min-w-0 space-y-2">
        <label class="block">
          <span class="input-label">{{ t('adminWorkbench.commission.exclusionSearch') }}</span>
          <input
            v-model="query"
            data-test="commission-exclusions-search"
            class="input"
            type="search"
            autocomplete="off"
            :disabled="mutating"
            :placeholder="t('adminWorkbench.commission.exclusionSearchPlaceholder')"
          />
        </label>
        <p v-if="searching" class="py-2 text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
        <p v-else-if="searchError" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ searchError }}</p>
        <ul v-else-if="results.length" class="divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="user in results" :key="user.id" class="flex min-w-0 items-center gap-3 py-2">
            <div class="min-w-0 flex-1">
              <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
              <p class="break-words text-xs text-gray-500 dark:text-dark-400">#{{ user.id }}<template v-if="user.username"> · {{ user.username }}</template></p>
            </div>
            <button
              type="button"
              :data-test="`commission-exclusion-add-${user.id}`"
              class="btn btn-secondary min-h-11 shrink-0 px-3"
              :disabled="mutating || isExcluded(user.id)"
              @click="add(user.id)"
            >
              {{ t(isExcluded(user.id) ? 'adminWorkbench.commission.alreadyExcluded' : 'adminWorkbench.commission.addExclusion') }}
            </button>
          </li>
        </ul>
        <p v-else-if="searched" class="py-2 text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.noData') }}</p>
      </div>

      <section class="min-w-0 border-t border-gray-100 pt-4 dark:border-dark-700">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('adminWorkbench.commission.excludedCustomers') }}</h3>
        <div v-if="loading" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="listError" class="space-y-2 py-3">
          <p class="text-sm text-red-600 dark:text-red-400" role="alert">{{ listError }}</p>
          <button type="button" class="btn btn-secondary min-h-11" @click="loadList">{{ t('common.refresh') }}</button>
        </div>
        <p v-else-if="items.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.commission.emptyExclusions') }}</p>
        <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <li
            v-for="item in items"
            :key="item.user_id"
            :data-test="`commission-exclusion-${item.user_id}`"
            class="flex min-w-0 items-center gap-3 py-3"
          >
            <div class="min-w-0 flex-1 space-y-1">
              <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ item.user_email }}</p>
              <p class="break-words text-xs text-gray-500 dark:text-dark-400">#{{ item.user_id }}<template v-if="item.username"> · {{ item.username }}</template></p>
              <p class="break-words text-xs text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.commission.exclusionStartedAt') }}：<time :datetime="item.started_at">{{ formatDateTime(item.started_at) }}</time>
              </p>
            </div>
            <button
              type="button"
              :data-test="`commission-exclusion-remove-${item.user_id}`"
              class="btn btn-secondary min-h-11 shrink-0 px-3"
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
      <button type="button" class="btn btn-secondary min-h-11" :disabled="mutating" @click="close">{{ t('common.close') }}</button>
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
  return items.value.some((item) => item.user_id === userID)
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
  searchController = new AbortController()
  try {
    let found: AdminUser[] = []
    if (/^\d+$/.test(keyword)) {
      const id = Number(keyword)
      if (Number.isSafeInteger(id) && id > 0) found = [await adminAPI.users.getById(id)]
    } else {
      const response = await adminAPI.users.list(1, 10, { search: keyword }, { signal: searchController.signal })
      found = response.items
    }
    if (!disposed && requestID === searchRequestID) results.value = found
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
  return mutate(() => adminAPI.subAdminCommission.addExclusion({ user_id: userID }))
}

function remove(userID: number) {
  return mutate(() => adminAPI.subAdminCommission.removeExclusion(userID))
}

onMounted(() => { void loadList() })
onBeforeUnmount(() => {
  disposed = true
  listRequestID += 1
  clearSearch()
})
</script>
