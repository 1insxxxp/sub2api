<template>
  <BaseDialog
    v-if="active"
    :show="true"
    :title="t('admin.groups.accountUserExclusions.title')"
    appearance="neutral"
    width="normal"
    :close-on-escape="!mutating"
    :show-close-button="!mutating"
    @close="close"
  >
    <div class="min-w-0 space-y-4">
      <div class="space-y-1">
        <p class="break-words text-sm font-semibold text-gray-900 [overflow-wrap:anywhere] dark:text-white">{{ group?.name }}</p>
        <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('admin.groups.accountUserExclusions.hint') }}</p>
      </div>

      <section class="min-w-0 space-y-2" :aria-label="t('admin.groups.accountUserExclusions.account')">
        <label class="block">
          <span class="input-label">{{ t('admin.groups.accountUserExclusions.account') }}</span>
          <input
            v-model="accountQuery"
            data-test="group-account-exclusions-account-search"
            type="search"
            autocomplete="off"
            class="input min-h-11"
            :disabled="mutating"
            :placeholder="t('admin.groups.accountUserExclusions.accountSearchPlaceholder')"
          />
        </label>
        <Select
          :model-value="selectedAccount?.id ?? null"
          :options="accountOptions"
          :placeholder="t('admin.groups.accountUserExclusions.selectAccount')"
          :aria-label="t('admin.groups.accountUserExclusions.account')"
          :disabled="mutating || accountsLoading"
          @update:model-value="selectAccount"
        />
        <p v-if="accountsLoading" class="text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
        <div v-else-if="accountError" class="flex flex-wrap items-center gap-2">
          <p class="flex-1 text-xs text-red-600 dark:text-red-400" role="alert">{{ accountError }}</p>
          <button type="button" class="btn btn-secondary min-h-11" :disabled="mutating" @click="loadAccounts">{{ t('common.refresh') }}</button>
        </div>
        <p v-else-if="!accounts.length" class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.groups.accountUserExclusions.noAccounts') }}</p>
        <Pagination
          v-if="!accountsLoading && !accountError && accountTotal > accountPageSize"
          variant="compact"
          :page="accountPage"
          :page-size="accountPageSize"
          :total="accountTotal"
          :show-page-size-selector="false"
          @update:page="changeAccountPage"
        />
      </section>

      <section class="min-w-0 space-y-2">
        <label class="block">
          <span class="input-label">{{ t('admin.groups.accountUserExclusions.user') }}</span>
          <input
            v-model="userQuery"
            data-test="group-account-exclusions-user-search"
            type="search"
            autocomplete="off"
            class="input min-h-11"
            :disabled="mutating"
            :placeholder="t('admin.groups.accountUserExclusions.userSearchPlaceholder')"
          />
        </label>
        <p v-if="searching" class="py-2 text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
        <p v-else-if="searchError" class="text-xs text-red-600 dark:text-red-400" role="alert">{{ searchError }}</p>
        <ul v-else-if="users.length" class="divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="user in users" :key="user.id" class="flex min-w-0 items-center gap-3 py-2">
            <div class="min-w-0 flex-1 space-y-1">
              <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ user.email }}</p>
              <p class="break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400">#{{ user.id }} · {{ t('admin.users.username') }}：{{ user.username?.trim() || '—' }}</p>
              <p class="whitespace-pre-wrap break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400">{{ t('admin.users.notes') }}：{{ user.notes?.trim() || '—' }}</p>
            </div>
            <button
              type="button"
              :data-test="`group-account-exclusion-add-${user.id}`"
              class="btn btn-secondary min-h-11 shrink-0 px-3"
              :disabled="mutating || !selectedAccount || isExcluded(user.id)"
              @click="add(user.id)"
            >
              {{ t(isExcluded(user.id) ? 'admin.groups.accountUserExclusions.alreadyExcluded' : 'common.add') }}
            </button>
          </li>
        </ul>
        <p v-else-if="searched" class="py-2 text-xs text-gray-500 dark:text-dark-400" role="status">{{ t('common.noData') }}</p>
      </section>

      <section class="min-w-0 border-t border-gray-100 pt-4 dark:border-dark-700">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.groups.accountUserExclusions.entries') }}</h3>
        <p v-if="loading" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('common.loading') }}</p>
        <div v-else-if="listError" class="space-y-2 py-3">
          <p class="text-sm text-red-600 dark:text-red-400" role="alert">{{ listError }}</p>
          <button data-test="group-account-exclusions-retry" type="button" class="btn btn-secondary min-h-11" :disabled="mutating" @click="loadList">{{ t('common.refresh') }}</button>
        </div>
        <p v-else-if="!items.length" class="py-6 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.groups.accountUserExclusions.empty') }}</p>
        <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="item in items" :key="item.id" :data-test="`group-account-exclusion-${item.id}`" class="flex min-w-0 items-center gap-3 py-3">
            <div class="min-w-0 flex-1 space-y-1">
              <p class="break-words text-xs font-medium text-gray-600 [overflow-wrap:anywhere] dark:text-dark-300">{{ item.account_name || `#${item.account_id}` }} · #{{ item.account_id }}</p>
              <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ item.user_email || `#${item.user_id}` }}</p>
              <p class="break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400">#{{ item.user_id }} · {{ t('admin.users.username') }}：{{ item.username?.trim() || '—' }}</p>
              <p class="whitespace-pre-wrap break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-dark-400">{{ t('admin.users.notes') }}：{{ item.notes?.trim() || '—' }}</p>
              <time :datetime="item.created_at" class="block text-xs text-gray-400 dark:text-dark-500">{{ formatDateTime(item.created_at) }}</time>
            </div>
            <button
              type="button"
              :data-test="`group-account-exclusion-remove-${item.id}`"
              class="btn btn-secondary min-h-11 shrink-0 px-3"
              :disabled="mutating"
              @click="remove(item.id)"
            >
              {{ t('admin.groups.accountUserExclusions.remove') }}
            </button>
          </li>
        </ul>
        <Pagination
          v-if="!loading && !listError && total > pageSize"
          data-test="group-account-exclusions-pagination"
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
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupAccountUserExclusion } from '@/api/admin/groups'
import type { AccountListItem, AdminGroup, AdminUser } from '@/types'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'

const props = defineProps<{ show: boolean; group: AdminGroup | null }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const active = computed(() => props.show && authStore.isAdmin && !!props.group)
const items = ref<GroupAccountUserExclusion[]>([])
const loading = ref(false)
const listError = ref('')
const page = ref(1)
const pageSize = 10
const total = ref(0)
const mutating = ref(false)
const accounts = ref<Pick<AccountListItem, 'id' | 'name'>[]>([])
const selectedAccount = ref<Pick<AccountListItem, 'id' | 'name'> | null>(null)
const accountQuery = ref('')
const accountsLoading = ref(false)
const accountError = ref('')
const accountPage = ref(1)
const accountPageSize = 20
const accountTotal = ref(0)
const userQuery = ref('')
const users = ref<AdminUser[]>([])
const searching = ref(false)
const searched = ref(false)
const searchError = ref('')
let epoch = 0
let disposed = false
let listVersion = 0
let accountVersion = 0
let searchVersion = 0
let listController: AbortController | undefined
let accountController: AbortController | undefined
let searchController: AbortController | undefined
let userTimer: ReturnType<typeof setTimeout> | undefined
let accountTimer: ReturnType<typeof setTimeout> | undefined

const accountOptions = computed(() => {
  const candidates = [...accounts.value]
  if (selectedAccount.value && !candidates.some(account => account.id === selectedAccount.value?.id)) candidates.unshift(selectedAccount.value)
  return candidates.map(account => ({ value: account.id, label: `${account.name} · #${account.id}` }))
})

function scope() {
  return active.value && props.group && !disposed ? { epoch, groupID: props.group.id } : null
}
function owns(request: NonNullable<ReturnType<typeof scope>>) {
  return !disposed && active.value && request.epoch === epoch && request.groupID === props.group?.id
}
function close() { if (!mutating.value) emit('close') }
function selectAccount(value: string | number | boolean | null) {
  if (!active.value || mutating.value || typeof value === 'boolean') return
  selectedAccount.value = accounts.value.find(account => account.id === Number(value)) ?? null
}
function isExcluded(userID: number) {
  return !!selectedAccount.value && items.value.some(item => item.user_id === userID && item.account_id === selectedAccount.value?.id)
}

async function loadList() {
  const request = scope()
  if (!request) return
  const version = ++listVersion
  listController?.abort()
  const controller = new AbortController()
  listController = controller
  loading.value = true
  listError.value = ''
  try {
    const response = await adminAPI.groups.listAccountUserExclusions(request.groupID, page.value, pageSize, { signal: controller.signal })
    if (!owns(request) || version !== listVersion) return
    total.value = response.total
    const lastPage = Math.max(1, Math.ceil(response.total / pageSize))
    if (page.value > lastPage) {
      page.value = lastPage
      await loadList()
      return
    }
    items.value = response.items
  } catch (error: unknown) {
    if (owns(request) && version === listVersion && !controller.signal.aborted) listError.value = extractApiErrorMessage(error, t('admin.groups.accountUserExclusions.loadFailed'))
  } finally {
    if (owns(request) && version === listVersion) loading.value = false
  }
}

async function loadAccounts() {
  const request = scope()
  if (!request) return
  const version = ++accountVersion
  accountController?.abort()
  const controller = new AbortController()
  accountController = controller
  accountsLoading.value = true
  accountError.value = ''
  try {
    const response = await adminAPI.accounts.list(accountPage.value, accountPageSize, {
      group: String(request.groupID), lite: 'true', search: accountQuery.value.trim() || undefined
    }, { signal: controller.signal })
    if (!owns(request) || version !== accountVersion) return
    accounts.value = response.items.map(({ id, name }) => ({ id, name }))
    accountTotal.value = response.total
  } catch (error: unknown) {
    if (owns(request) && version === accountVersion && !controller.signal.aborted) accountError.value = extractApiErrorMessage(error, t('admin.groups.accountUserExclusions.accountsFailed'))
  } finally {
    if (owns(request) && version === accountVersion) accountsLoading.value = false
  }
}

function clearUserSearch() {
  searchVersion += 1
  clearTimeout(userTimer)
  searchController?.abort()
  users.value = []
  searching.value = false
  searched.value = false
  searchError.value = ''
}

async function searchUsers(keyword: string, version: number) {
  const request = scope()
  if (!request || version !== searchVersion) return
  const controller = new AbortController()
  searchController = controller
  searching.value = true
  try {
    let found: AdminUser[] = []
    if (/^\d+$/.test(keyword)) {
      const id = Number(keyword)
      if (Number.isSafeInteger(id) && id > 0) found = [await adminAPI.users.getById(id)]
    } else {
      const response = await adminAPI.users.list(1, 10, { search: keyword }, { signal: controller.signal })
      found = response.items
    }
    if (owns(request) && version === searchVersion) users.value = found
  } catch (error: unknown) {
    if (owns(request) && version === searchVersion && !controller.signal.aborted) searchError.value = extractApiErrorMessage(error, t('admin.groups.accountUserExclusions.searchFailed'))
  } finally {
    if (owns(request) && version === searchVersion) { searching.value = false; searched.value = true }
  }
}

watch(userQuery, value => {
  clearUserSearch()
  const keyword = value.trim()
  if (!active.value || !keyword) return
  const version = searchVersion
  userTimer = setTimeout(() => { void searchUsers(keyword, version) }, 300)
})

watch(accountQuery, () => {
  clearTimeout(accountTimer)
  accountVersion += 1
  accountController?.abort()
  if (!active.value) return
  accountPage.value = 1
  accountTimer = setTimeout(() => { void loadAccounts() }, 300)
})

function changePage(value: number) {
  if (!active.value || mutating.value || loading.value || value === page.value) return
  page.value = value
  void loadList()
}
function changeAccountPage(value: number) {
  if (!active.value || mutating.value || accountsLoading.value || value === accountPage.value) return
  accountPage.value = value
  void loadAccounts()
}

async function mutate(action: (groupID: number) => Promise<unknown>) {
  const request = scope()
  if (!request || mutating.value) return
  mutating.value = true
  try {
    await action(request.groupID)
    if (!owns(request)) return
    userQuery.value = ''
    clearUserSearch()
    appStore.showSuccess(t('admin.groups.accountUserExclusions.saved'))
    await loadList()
  } catch (error: unknown) {
    if (owns(request)) appStore.showError(extractApiErrorMessage(error, t('admin.groups.accountUserExclusions.saveFailed')))
  } finally {
    if (owns(request)) mutating.value = false
  }
}
function add(userID: number) {
  const accountID = selectedAccount.value?.id
  if (!accountID || !users.value.some(user => user.id === userID) || isExcluded(userID)) return
  return mutate(groupID => adminAPI.groups.addAccountUserExclusion(groupID, { account_id: accountID, user_id: userID }))
}
function remove(exclusionID: number) {
  if (!items.value.some(item => item.id === exclusionID)) return
  return mutate(groupID => adminAPI.groups.removeAccountUserExclusion(groupID, exclusionID))
}

function reset() {
  epoch += 1
  listVersion += 1
  accountVersion += 1
  listController?.abort()
  accountController?.abort()
  clearTimeout(accountTimer)
  clearUserSearch()
  items.value = []
  accounts.value = []
  selectedAccount.value = null
  userQuery.value = ''
  accountQuery.value = ''
  total.value = 0
  accountTotal.value = 0
  page.value = 1
  accountPage.value = 1
  loading.value = false
  accountsLoading.value = false
  mutating.value = false
  listError.value = ''
  accountError.value = ''
}

watch([active, () => props.group?.id], () => {
  reset()
  if (active.value) { void loadList(); void loadAccounts() }
}, { immediate: true })
onBeforeUnmount(() => { disposed = true; reset() })
</script>
