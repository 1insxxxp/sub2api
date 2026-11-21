<template>
  <div :class="flat ? 'usage-list-filters px-3 py-2.5 md:px-4 md:py-3' : 'card p-6'">
    <!-- Toolbar: left filters (multi-line) + right actions -->
    <div class="admin-toolbar items-end" :class="{ 'usage-filter-toolbar': flat }">
      <!-- Left: filters (allowed to wrap to multiple rows) -->
      <div class="admin-toolbar-group flex-1 items-end gap-4" :class="{ 'usage-filter-fields': flat }">
        <!-- User Search -->
        <div ref="userSearchRef" class="usage-filter-user usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[240px]">
          <label class="input-label">{{ t('admin.usage.userFilter') }}</label>
          <input
            v-model="userKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchUserPlaceholder')"
            :aria-label="t('admin.usage.userFilter')"
            @input="debounceUserSearch"
            @focus="showUserDropdown = true"
          />
          <button
            v-if="filters.user_id"
            type="button"
            @click="clearUser"
            class="usage-clear-user absolute right-2 top-8 flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-500/10 dark:hover:text-primary-200"
            aria-label="Clear user filter"
          >
            ✕
          </button>
          <div
            v-if="showUserDropdown && (userResults.length > 0 || userKeyword)"
            class="admin-action-menu absolute z-50 mt-1 max-h-60 w-full overflow-y-auto"
          >
            <button
              v-for="u in userResults"
              :key="u.id"
              type="button"
              @click="selectUser(u)"
              class="admin-action-menu-item justify-between"
            >
              <span>{{ u.email }}<span v-if="u.deleted" class="ml-1 text-xs text-gray-400">（{{ t('admin.usage.userDeletedBadge') }}）</span></span>
              <span class="ml-2 text-xs text-gray-400">#{{ u.id }}</span>
            </button>
          </div>
        </div>

        <button
          v-if="flat && hasExtraFilters"
          ref="filterToggle"
          type="button"
          class="btn btn-secondary usage-filter-toggle"
          data-test="usage-filter-toggle"
          :aria-label="t('common.filter')"
          :aria-expanded="filtersExpanded"
          :aria-controls="mobile && showActions ? 'usage-extra-filters usage-filter-actions' : 'usage-extra-filters'"
          @click="filtersExpanded = !filtersExpanded"
        >
          <Icon name="filter" size="sm" />
          <span class="usage-filter-toggle-label">{{ t('common.filter') }}</span>
          <span v-if="activeExtraFilters" class="usage-filter-count">{{ activeExtraFilters }}</span>
        </button>
        <button
          v-if="flat && mobile && showActions"
          type="button"
          class="btn btn-secondary usage-filter-refresh"
          :aria-label="t('common.refresh')"
          @click="$emit('refresh')"
        >
          <Icon name="refresh" size="sm" />
        </button>

        <div
          v-show="showExtraFilters"
          :id="flat ? 'usage-extra-filters' : undefined"
          :class="flat && hasExtraFilters ? 'usage-extra-filters' : 'contents'"
          @keydown.esc="closeExtraFilters"
        >
          <!-- API Key Search -->
          <div v-if="mode === 'usage' || mode === 'errors'" ref="apiKeySearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[240px]">
            <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
            <input
              v-model="apiKeyKeyword"
              type="text"
              class="input pr-8"
              :placeholder="t('admin.usage.searchApiKeyPlaceholder')"
              @input="debounceApiKeySearch"
              @focus="onApiKeyFocus"
            />
            <button
              v-if="filters.api_key_id"
              type="button"
              @click="onClearApiKey"
              class="absolute right-2 top-8 flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-500/10 dark:hover:text-primary-200"
              aria-label="Clear API key filter"
            >
              ✕
            </button>
            <div
              v-if="showApiKeyDropdown && apiKeyResults.length > 0"
              class="admin-action-menu absolute z-50 mt-1 max-h-60 w-full overflow-y-auto"
            >
              <button
                v-for="k in apiKeyResults"
                :key="k.id"
                type="button"
                @click="selectApiKey(k)"
                class="admin-action-menu-item justify-between"
              >
                <span class="truncate">{{ k.name || `#${k.id}` }}</span>
                <span class="ml-2 text-xs text-gray-400">#{{ k.id }}</span>
              </button>
            </div>
          </div>

          <!-- Model Filter -->
          <div v-if="mode === 'usage' || mode === 'errors'" class="w-full sm:w-auto sm:min-w-[220px]">
            <label class="input-label">{{ t('usage.model') }}</label>
            <Select v-model="filters.model" :options="modelOptions" searchable @change="emitChange" />
          </div>

          <!-- Account Filter -->
          <div v-if="mode === 'usage' || mode === 'errors'" ref="accountSearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[220px]">
            <label class="input-label">{{ t('admin.usage.account') }}</label>
            <input
              v-model="accountKeyword"
              type="text"
              class="input pr-8"
              :placeholder="t('admin.usage.searchAccountPlaceholder')"
              @input="debounceAccountSearch"
              @focus="showAccountDropdown = true"
            />
            <button
              v-if="filters.account_id"
              type="button"
              @click="clearAccount"
              class="absolute right-2 top-8 flex h-8 w-8 items-center justify-center rounded-lg text-slate-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-500/10 dark:hover:text-primary-200"
              aria-label="Clear account filter"
            >
              ✕
            </button>
            <div
              v-if="showAccountDropdown && (accountResults.length > 0 || accountKeyword)"
              class="admin-action-menu absolute z-50 mt-1 max-h-60 w-full overflow-y-auto"
            >
              <button
                v-for="a in accountResults"
                :key="a.id"
                type="button"
                @click="selectAccount(a)"
                class="admin-action-menu-item justify-between"
              >
                <span class="truncate">{{ a.name }}</span>
                <span class="ml-2 text-xs text-gray-400">#{{ a.id }}</span>
              </button>
            </div>
          </div>

          <!-- Request Type Filter (usage only) -->
          <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('usage.type') }}</label>
            <Select v-model="filters.request_type" :options="requestTypeOptions" @change="emitChange" />
          </div>

          <!-- Native compaction is independent of the transport request type. -->
          <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('usage.compactionFilter') }}</label>
            <Select v-model="filters.native_compaction_v2" :options="compactionOptions" @change="emitChange" />
          </div>

          <!-- Billing Type Filter (usage only) -->
          <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[200px]">
            <label class="input-label">{{ t('admin.usage.billingType') }}</label>
            <Select v-model="filters.billing_type" :options="billingTypeOptions" @change="emitChange" />
          </div>

          <!-- Billing Mode Filter (usage only；用户排行的 user-breakdown 接口不支持该维度) -->
          <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[200px]">
            <label class="input-label">{{ t('admin.usage.billingMode') }}</label>
            <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="emitChange" />
          </div>

          <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[220px]">
            <label class="input-label">{{ t('admin.usage.upstreamModelAudit') }}</label>
            <Select v-model="filters.upstream_model_mismatch" :options="upstreamModelMismatchOptions" @change="emitChange" />
          </div>

          <!-- Error Phase Filter (errors only) -->
          <div v-if="mode === 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('admin.ops.errorLog.type') }}</label>
            <Select v-model="filters.error_phase" :options="errorPhaseOptions" @change="emitChange" />
          </div>

          <!-- Error Category Filter (errors only) -->
          <div v-if="mode === 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('usage.errors.category') }}</label>
            <Select v-model="filters.error_category" :options="errorCategoryOptions" @change="emitChange" />
          </div>

          <!-- Status Code Filter (errors only) -->
          <div v-if="mode === 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
            <label class="input-label">{{ t('admin.ops.errorLog.status') }}</label>
            <Select v-model="filters.status_code" :options="statusCodeOptions" @change="emitChange" />
          </div>

          <!-- Group Filter -->
          <div v-if="mode === 'usage' || mode === 'errors'" class="w-full sm:w-auto sm:min-w-[200px]">
            <label class="input-label">{{ t('admin.usage.group') }}</label>
            <Select v-model="filters.group_id" :options="groupOptions" searchable @change="emitChange" />
          </div>
        </div>
      </div>

      <!-- Right: actions -->
      <div
        v-if="showActions"
        v-show="!mobile || showExtraFilters"
        :id="flat ? 'usage-filter-actions' : undefined"
        class="admin-toolbar-group w-full justify-end sm:w-auto"
        :class="{ 'usage-filter-actions': flat }"
        @keydown.esc="closeExtraFilters"
      >
        <button v-if="!flat || !mobile" type="button" @click="$emit('refresh')" class="btn btn-secondary">
          {{ t('common.refresh') }}
        </button>
        <button type="button" @click="$emit('reset')" class="btn btn-secondary">
          {{ t('common.reset') }}
        </button>
        <slot name="after-reset" />
        <template v-if="mode === 'usage' && showRecordActions">
          <button type="button" @click="$emit('cleanup')" class="btn btn-danger">
            {{ t('admin.usage.cleanup.button') }}
          </button>
          <button type="button" @click="$emit('export')" :disabled="exporting" class="btn btn-primary">
            {{ t('usage.exportExcel') }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, toRef, watch, computed } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'
import type { SimpleApiKey, SimpleUser } from '@/api/admin/usage'

type ModelValue = Record<string, any>

interface Props {
  modelValue: ModelValue
  exporting: boolean
  startDate: string
  endDate: string
  showActions?: boolean
  showRecordActions?: boolean
  modelOptions?: string[]
  /**
   * errors 模式:隐藏用量专属字段/按钮,显示错误类型+状态码(错误请求 tab 用)
   * ranking 模式:同 usage 但隐藏计费模式筛选与清理/导出按钮(用户排行 tab 用)
   */
  mode?: 'usage' | 'errors' | 'ranking' | 'recharge'
  /** 嵌入统一卡片内使用：去掉自身卡片外观 */
  flat?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true,
  showRecordActions: true,
  mode: 'usage',
  flat: false
})
const emit = defineEmits([
  'update:modelValue',
  'change',
  'refresh',
  'reset',
  'export',
  'cleanup'
])

const { t } = useI18n()
const filters = toRef(props, 'modelValue')
const mobile = useMediaQuery('(max-width: 767px)')
const filtersExpanded = ref(false)
const filterToggle = ref<HTMLButtonElement | null>(null)
const hasExtraFilters = computed(() => props.mode === 'usage' || props.mode === 'errors')
const showExtraFilters = computed(() => !props.flat || !hasExtraFilters.value || filtersExpanded.value)
const activeExtraFilters = computed(() => {
  if (!hasExtraFilters.value) return 0
  const shared = ['api_key_id', 'model', 'account_id', 'group_id']
  const fields = props.mode === 'usage'
    ? [...shared, 'request_type', 'native_compaction_v2', 'billing_type', 'billing_mode', 'upstream_model_mismatch']
    : [...shared, 'error_phase', 'error_category', 'status_code']
  return fields.filter((field) => filters.value[field] != null && filters.value[field] !== '').length
})

function closeExtraFilters(event: KeyboardEvent) {
  if (event.defaultPrevented || !props.flat || !filtersExpanded.value) return
  event.preventDefault()
  filtersExpanded.value = false
  filterToggle.value?.focus()
}

const userSearchRef = ref<HTMLElement | null>(null)
const apiKeySearchRef = ref<HTMLElement | null>(null)
const accountSearchRef = ref<HTMLElement | null>(null)

const userKeyword = ref('')
const userResults = ref<SimpleUser[]>([])
const showUserDropdown = ref(false)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null
let userSearchSequence = 0

const apiKeyKeyword = ref('')
const apiKeyResults = ref<SimpleApiKey[]>([])
const showApiKeyDropdown = ref(false)
let apiKeySearchTimeout: ReturnType<typeof setTimeout> | null = null

interface SimpleAccount {
  id: number
  name: string
}
const accountKeyword = ref('')
const accountResults = ref<SimpleAccount[]>([])
const showAccountDropdown = ref(false)
let accountSearchTimeout: ReturnType<typeof setTimeout> | null = null

const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...(props.modelOptions ?? []).map((m) => ({ value: m, label: m })),
])
const groupOptions = ref<SelectOption[]>([{ value: null, label: t('admin.usage.allGroups') }])

const requestTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
  { value: 'cyber', label: t('usage.cyber') }
])

const compactionOptions = ref<SelectOption[]>([
  { value: null, label: t('usage.allCompactionTypes') },
  { value: true, label: t('usage.compactionOnly') }
])

const billingTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingTypes') },
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') }
])

// 错误类型对应后端 phase 参数(与错误表"类型"徽章同语义)
const errorPhaseOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'upstream', label: t('admin.ops.errorLog.typeUpstream') },
  { value: 'account_auth', label: t('admin.ops.errorLog.typeAccountAuth') },
  { value: 'request', label: t('admin.ops.errorLog.typeRequest') },
  { value: 'auth', label: t('admin.ops.errorLog.typeAuth') },
  { value: 'routing', label: t('admin.ops.errorLog.typeRouting') },
  { value: 'internal', label: t('admin.ops.errorLog.typeInternal') },
])

// 分类码同用户端 /usage 错误筛选;"other" 无法反查为过滤条件,刻意不列
const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'upstream', 'internal', 'cyber']

const errorCategoryOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allCategories') },
  ...errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) })),
])

const statusCodeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allStatuses') },
  ...COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })),
])

const billingModeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') }
])

const upstreamModelMismatchOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allUpstreamModelAudit') },
  { value: true, label: t('admin.usage.upstreamModelMismatchOnly') },
  { value: false, label: t('admin.usage.upstreamModelMatchedOnly') }
])

const emitChange = () => emit('change')

const clearPendingUserSearch = () => {
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
    userSearchTimeout = null
  }
  userSearchSequence += 1
}

const debounceUserSearch = () => {
  clearPendingUserSearch()
  const query = userKeyword.value.trim()
  if (!query) {
    userResults.value = []
    return
  }

  const sequence = userSearchSequence
  userSearchTimeout = setTimeout(async () => {
    userSearchTimeout = null
    try {
      const results = await adminAPI.usage.searchUsers(query)
      if (sequence === userSearchSequence) {
        userResults.value = results.sort((a, b) => Number(a.deleted) - Number(b.deleted))
      }
    } catch {
      if (sequence === userSearchSequence) {
        userResults.value = []
      }
    }
  }, 300)
}

const debounceApiKeySearch = () => {
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
  apiKeySearchTimeout = setTimeout(async () => {
    try {
      apiKeyResults.value = await adminAPI.usage.searchApiKeys(
        filters.value.user_id,
        apiKeyKeyword.value || ''
      )
    } catch {
      apiKeyResults.value = []
    }
  }, 300)
}

const selectUser = async (u: SimpleUser) => {
  clearPendingUserSearch()
  userKeyword.value = u.email
  showUserDropdown.value = false
  filters.value.user_id = u.id
  clearApiKey()

  // Auto-load API keys for this user
  try {
    apiKeyResults.value = await adminAPI.usage.searchApiKeys(u.id, '')
  } catch {
    apiKeyResults.value = []
  }

  emitChange()
}

const clearUser = () => {
  clearPendingUserSearch()
  userKeyword.value = ''
  userResults.value = []
  showUserDropdown.value = false
  filters.value.user_id = undefined
  clearApiKey()
  emitChange()
}

const selectApiKey = (k: SimpleApiKey) => {
  apiKeyKeyword.value = k.name || String(k.id)
  showApiKeyDropdown.value = false
  filters.value.api_key_id = k.id
  emitChange()
}

const clearApiKey = () => {
  apiKeyKeyword.value = ''
  apiKeyResults.value = []
  showApiKeyDropdown.value = false
  filters.value.api_key_id = undefined
}

const onClearApiKey = () => {
  clearApiKey()
  emitChange()
}

const debounceAccountSearch = () => {
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
  accountSearchTimeout = setTimeout(async () => {
    if (!accountKeyword.value) {
      accountResults.value = []
      return
    }
    try {
      const res = await adminAPI.accounts.list(1, 20, { search: accountKeyword.value })
      accountResults.value = res.items.map((a) => ({ id: a.id, name: a.name }))
    } catch {
      accountResults.value = []
    }
  }, 300)
}

const selectAccount = (a: SimpleAccount) => {
  accountKeyword.value = a.name
  showAccountDropdown.value = false
  filters.value.account_id = a.id
  emitChange()
}

const clearAccount = () => {
  accountKeyword.value = ''
  accountResults.value = []
  showAccountDropdown.value = false
  filters.value.account_id = undefined
  emitChange()
}

const onApiKeyFocus = () => {
  showApiKeyDropdown.value = true
  // Trigger search if no results yet
  if (apiKeyResults.value.length === 0) {
    debounceApiKeySearch()
  }
}

const onDocumentClick = (e: MouseEvent) => {
  const target = e.target as Node | null
  if (!target) return

  const clickedInsideUser = userSearchRef.value?.contains(target) ?? false
  const clickedInsideApiKey = apiKeySearchRef.value?.contains(target) ?? false
  const clickedInsideAccount = accountSearchRef.value?.contains(target) ?? false

  if (!clickedInsideUser) showUserDropdown.value = false
  if (!clickedInsideApiKey) showApiKeyDropdown.value = false
  if (!clickedInsideAccount) showAccountDropdown.value = false
}

watch(
  () => props.startDate,
  (value) => {
    filters.value.start_date = value
  },
  { immediate: true }
)

watch(
  () => props.endDate,
  (value) => {
    filters.value.end_date = value
  },
  { immediate: true }
)

watch(
  () => filters.value.user_id,
  (userId) => {
    if (!userId) {
      clearPendingUserSearch()
      userKeyword.value = ''
      userResults.value = []
    }
  }
)

watch(
  () => filters.value.api_key_id,
  (apiKeyId) => {
    if (!apiKeyId) {
      apiKeyKeyword.value = ''
      apiKeyResults.value = []
    }
  }
)

watch(
  () => filters.value.account_id,
  (accountId) => {
    if (!accountId) {
      accountKeyword.value = ''
      accountResults.value = []
    }
  }
)

onMounted(async () => {
  document.addEventListener('click', onDocumentClick)
  try {
    const gs = await adminAPI.groups.list(1, 1000)
    groupOptions.value.push(...gs.items.map((g: any) => ({ value: g.id, label: g.name })))
  } catch {
    // Ignore filter option loading errors (page still usable)
  }
})

onUnmounted(() => {
  clearPendingUserSearch()
  document.removeEventListener('click', onDocumentClick)
})

// 供外部(如用户排行下钻)在程序化设置 user_id 后回显选中的用户邮箱
const setUserKeyword = (email: string) => {
  clearPendingUserSearch()
  userKeyword.value = email
  userResults.value = []
  showUserDropdown.value = false
}

const getUserSearchRevision = () => userSearchSequence

defineExpose({ getUserSearchRevision, setUserKeyword })
</script>

<style scoped>
.usage-filter-count {
  padding: 0.125rem 0.375rem;
  border-radius: 4px;
  background: rgb(var(--brand-rgb) / 12%);
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
}

.usage-list-filters .usage-filter-toolbar { align-items: start; }
.usage-list-filters .usage-filter-fields { width: 100%; }
.usage-list-filters .usage-filter-user { flex: 0 1 20rem; min-width: 0; }
.usage-list-filters .usage-filter-user .input-label {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip-path: inset(50%);
}
.usage-list-filters .usage-clear-user { top: 0.25rem; right: 0.25rem; }
.usage-filter-toggle { flex-shrink: 0; gap: 0.375rem; padding-inline: 0.625rem; white-space: nowrap; }
.usage-filter-refresh { width: 2.5rem; padding-inline: 0; }
.usage-extra-filters {
  display: grid;
  flex-basis: 100%;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 11rem), 1fr));
  gap: 0.5rem;
  align-items: end;
  padding-top: 0.25rem;
}
.usage-extra-filters > div { width: 100%; min-width: 0; }

@media (max-width: 767px) {
  .usage-list-filters .usage-filter-toolbar { gap: 0.625rem; }
  .usage-list-filters .usage-filter-fields {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: end;
  }
  .usage-extra-filters {
    grid-column: 1 / -1;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .usage-list-filters .usage-filter-actions {
    width: 100%;
    flex-wrap: wrap;
    justify-content: flex-start;
  }
  .usage-filter-actions .btn { padding-inline: 0.625rem; font-size: 0.75rem; }
}

@media (max-width: 359px) {
  .usage-filter-toggle-label { display: none; }
}
</style>
