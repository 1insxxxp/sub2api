<template>
  <AppLayout>
    <div class="admin-workbench-page mx-auto min-w-0 max-w-6xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
      <header class="flex min-w-0 flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between">
        <div class="min-w-0 flex-1">
          <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">
            {{ t('adminWorkbench.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('adminWorkbench.description') }}
          </p>
        </div>
        <div class="w-full rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-left dark:border-blue-500/20 dark:bg-blue-500/10 sm:w-auto sm:text-right">
          <p class="text-xs font-medium text-blue-600 dark:text-blue-300">
            {{ t('adminWorkbench.currentBalance') }}
          </p>
          <p class="mt-1 text-2xl font-semibold tabular-nums text-blue-950 dark:text-blue-50">
            ${{ availableBalance.toFixed(2) }}
          </p>
        </div>
      </header>

      <nav
        data-test="admin-workbench-tabs"
        role="tablist"
        :aria-label="t('adminWorkbench.tabs.label')"
        class="flex min-w-0 gap-1 overflow-x-auto border-b border-gray-200 dark:border-dark-700"
      >
        <button
          v-for="tab in workbenchTabs"
          :id="`workbench-tab-${tab.id}`"
          :key="tab.id"
          type="button"
          role="tab"
          :data-test="`workbench-tab-${tab.id}`"
          :aria-controls="`workbench-panel-${tab.id}`"
          :aria-selected="activeTab === tab.id"
          class="-mb-px inline-flex shrink-0 items-center gap-2 border-b-2 px-4 py-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-dark-950"
          :class="activeTab === tab.id
            ? 'border-primary-600 text-primary-700 dark:border-primary-400 dark:text-primary-300'
            : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-800 dark:text-dark-400 dark:hover:border-dark-500 dark:hover:text-dark-200'"
          @click="activeTab = tab.id"
        >
          <Icon :name="tab.icon" size="sm" />
          <span>{{ tab.label }}</span>
        </button>
      </nav>

      <section
        v-if="activeTab === 'commission'"
        id="workbench-panel-commission"
        data-test="workbench-commission-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-commission"
        class="min-w-0"
      >
        <SubAdminCommissionPanel />
      </section>

      <section
        v-else-if="activeTab === 'affiliate-leaderboard'"
        id="workbench-panel-affiliate-leaderboard"
        data-test="workbench-affiliate-leaderboard-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-affiliate-leaderboard"
        class="min-w-0"
      >
        <AdminAffiliateLeaderboardPanel />
      </section>

      <div
        v-else
        id="workbench-panel-balance-transfer"
        data-test="workbench-balance-transfer-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-balance-transfer"
        class="min-w-0 space-y-6"
      >
        <section class="grid min-w-0 gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <form
            ref="transferFormRef"
            data-test="workbench-transfer-form"
            class="min-w-0 rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
            @submit.prevent="handleGenerate"
          >
            <div class="mb-5">
              <h2 class="text-base font-semibold text-gray-950 dark:text-white">
                {{ t('adminWorkbench.balanceTransfer.title') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.balanceTransfer.subtitle') }}
              </p>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">{{ t('adminWorkbench.balanceTransfer.amount') }}</span>
                <input
                  v-model="form.amount"
                  data-test="workbench-transfer-amount"
                  type="number"
                  min="0.01"
                  step="0.01"
                  class="input"
                  :placeholder="t('adminWorkbench.balanceTransfer.amountPlaceholder')"
                />
              </label>
              <label class="block">
                <span class="input-label">{{ t('adminWorkbench.balanceTransfer.count') }}</span>
                <input
                  v-model.number="form.count"
                  data-test="workbench-transfer-count"
                  type="number"
                  min="1"
                  max="100"
                  step="1"
                  class="input"
                />
              </label>
              <label class="block">
                <span class="input-label">{{ t('adminWorkbench.balanceTransfer.expiresInDays') }}</span>
                <input
                  v-model.number="form.expires_in_days"
                  data-test="workbench-transfer-expiry"
                  type="number"
                  min="1"
                  max="3650"
                  step="1"
                  class="input"
                />
              </label>
              <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('adminWorkbench.balanceTransfer.totalValue') }}
                </p>
                <p class="mt-1 text-lg font-semibold tabular-nums text-gray-950 dark:text-white">
                  ${{ totalValue.toFixed(2) }}
                </p>
              </div>
            </div>

            <label class="mt-4 block">
              <span class="input-label">{{ t('adminWorkbench.balanceTransfer.notes') }}</span>
              <textarea
                v-model="form.notes"
                data-test="workbench-transfer-notes"
                rows="3"
                class="input resize-y"
                :placeholder="t('adminWorkbench.balanceTransfer.notesPlaceholder')"
              ></textarea>
            </label>

            <label class="mt-4 flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700">
              <input
                v-model="form.single_use_per_user"
                data-test="workbench-transfer-single-use"
                type="checkbox"
                class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
              <span>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('adminWorkbench.balanceTransfer.singleUsePerUser') }}
                </span>
                <span class="mt-1 block text-xs leading-5 text-gray-500 dark:text-dark-400">
                  {{ t('adminWorkbench.balanceTransfer.singleUsePerUserHint') }}
                </span>
              </span>
            </label>

            <p v-if="errorMessage" class="mt-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-300">
              {{ errorMessage }}
            </p>

            <button
              type="submit"
              class="btn btn-primary mt-5 w-full justify-center"
              :disabled="generating"
            >
              <Icon v-if="generating" name="refresh" size="sm" class="animate-spin" />
              <span>{{ generating ? t('adminWorkbench.balanceTransfer.generating') : t('adminWorkbench.balanceTransfer.generate') }}</span>
            </button>
          </form>

          <section
            data-test="workbench-generated-now-card"
            class="admin-workbench-generated-now flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
            :style="generatedNowPanelStyle"
          >
            <div class="mb-4 flex shrink-0 items-center justify-between gap-3">
              <div class="min-w-0">
                <h2 class="text-base font-semibold text-gray-950 dark:text-white">
                  {{ t('adminWorkbench.balanceTransfer.generatedNow') }}
                </h2>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ t('adminWorkbench.balanceTransfer.generatedNowHint') }}
                </p>
              </div>
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="generatedResults.length === 0"
                @click="copyGeneratedResults"
              >
                <Icon name="copy" size="sm" />
                <span>{{ t('common.copy') }}</span>
              </button>
            </div>

            <div v-if="generatedResults.length === 0" class="flex min-h-44 flex-1 items-center justify-center rounded-lg bg-gray-50 text-sm text-gray-500 dark:bg-dark-800/70 dark:text-dark-400">
              {{ t('adminWorkbench.balanceTransfer.noGeneratedNow') }}
            </div>
            <div v-else data-test="workbench-generated-results" class="admin-workbench-generated-results min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
              <div
                v-for="item in generatedResults"
                :key="item.id"
                data-test="workbench-generated-code"
                class="rounded-lg border border-blue-100 bg-blue-50/70 px-3 py-2 text-sm text-blue-950 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-100"
              >
                <div class="break-all font-mono">{{ item.code }}</div>
                <p v-if="item.notes" class="mt-1 break-words text-xs leading-5 text-blue-700 dark:text-blue-200">
                  {{ t('adminWorkbench.balanceTransfer.notes') }}: {{ item.notes }}
                </p>
              </div>
            </div>
          </section>
        </section>

        <section class="min-w-0 rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-950 dark:text-white">
                {{ t('adminWorkbench.balanceTransfer.generatedList') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.balanceTransfer.generatedListHint') }}
              </p>
              <p v-if="selectedGeneratedCodeIds.length > 0" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('adminWorkbench.balanceTransfer.selectedCount', { count: selectedGeneratedCodeIds.length }) }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <label
                v-if="deletableGeneratedCodes.length > 0"
                class="inline-flex h-10 items-center gap-2 rounded-lg border border-gray-200 px-3 text-sm text-gray-700 dark:border-dark-700 dark:text-dark-300"
              >
                <input
                  data-test="select-all-generated-codes"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="allDeletableGeneratedCodesSelected"
                  :disabled="batchDeletingGeneratedCodes"
                  @change="handleSelectAllGeneratedCodes"
                />
                <span>{{ t('adminWorkbench.balanceTransfer.selectAll') }}</span>
              </label>
              <button
                type="button"
                data-test="delete-selected-generated-codes"
                class="btn btn-secondary text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                :disabled="selectedGeneratedCodeIds.length === 0 || batchDeletingGeneratedCodes"
                @click="handleDeleteSelectedGeneratedCodes"
              >
                <Icon
                  :name="batchDeletingGeneratedCodes ? 'refresh' : 'trash'"
                  size="sm"
                  :class="{ 'animate-spin': batchDeletingGeneratedCodes }"
                />
                <span>{{ t('adminWorkbench.balanceTransfer.batchDelete') }}</span>
              </button>
              <button type="button" class="btn btn-secondary" :disabled="loadingGenerated" @click="fetchGeneratedCodes">
                <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingGenerated }" />
                <span>{{ t('common.refresh') }}</span>
              </button>
            </div>
          </div>

          <div v-if="loadingGenerated" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="generatedCodes.length === 0" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('adminWorkbench.balanceTransfer.empty') }}
          </div>
          <div v-else class="space-y-3">
            <article
              v-for="item in generatedCodes"
              :key="item.id"
              class="flex flex-col gap-3 rounded-lg border border-gray-200 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="flex min-w-0 flex-1 gap-3">
                <label v-if="canDeleteGeneratedCode(item)" class="mt-1 shrink-0">
                  <input
                    v-model="selectedGeneratedCodeIds"
                    type="checkbox"
                    class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    :value="item.id"
                    :data-test="`select-generated-code-${item.id}`"
                    :disabled="deletingIds.includes(item.id) || batchDeletingGeneratedCodes"
                  />
                </label>
                <span v-else class="mt-1 h-4 w-4 shrink-0" aria-hidden="true"></span>
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="break-all font-mono text-sm font-semibold text-gray-950 dark:text-white">{{ item.code }}</span>
                    <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">
                      ${{ item.value.toFixed(2) }}
                    </span>
                    <span :class="getGeneratedStatusClass(item.status)">
                      {{ getGeneratedStatusLabel(item.status) }}
                    </span>
                    <span v-if="item.single_use_per_user" class="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
                      {{ t('adminWorkbench.balanceTransfer.singleUseBadge') }}
                    </span>
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ formatDateTime(item.created_at) }}
                    <span v-if="item.expires_at"> · {{ t('adminWorkbench.balanceTransfer.expiresAt') }} {{ formatDateTime(item.expires_at) }}</span>
                    <span v-if="item.used_at"> · {{ t('adminWorkbench.balanceTransfer.usedAt') }} {{ formatDateTime(item.used_at) }}</span>
                  </p>
                  <p v-if="item.notes" class="mt-2 break-words rounded-md bg-gray-50 px-2 py-1 text-xs leading-5 text-gray-600 dark:bg-dark-800/70 dark:text-dark-300">
                    {{ t('adminWorkbench.balanceTransfer.notes') }}: {{ item.notes }}
                  </p>
                </div>
              </div>
              <div class="flex w-full shrink-0 items-center justify-end gap-2 sm:w-auto">
                <button type="button" class="btn btn-secondary px-3" @click="copyCode(item.code)">
                  <Icon name="copy" size="sm" />
                </button>
                <button
                  v-if="canDeleteGeneratedCode(item)"
                  type="button"
                  class="btn btn-danger px-3"
                  :disabled="deletingIds.includes(item.id)"
                  @click="handleDeleteGeneratedCode(item)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </article>
          </div>

          <Pagination
            v-if="generatedPagination.total > generatedPagination.page_size"
            class="mt-5"
            :page="generatedPagination.page"
            :page-size="generatedPagination.page_size"
            :total="generatedPagination.total"
            :page-size-options="[10, 20, 50]"
            @update:page="handleGeneratedPageChange"
            @update:page-size="handleGeneratedPageSizeChange"
          />
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { redeemAPI, type GeneratedRedeemCode } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import SubAdminCommissionPanel from '@/components/admin/workbench/SubAdminCommissionPanel.vue'
import AdminAffiliateLeaderboardPanel from '@/components/admin/workbench/AdminAffiliateLeaderboardPanel.vue'
import { formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

type WorkbenchTab = 'balance-transfer' | 'commission' | 'affiliate-leaderboard'

const activeTab = ref<WorkbenchTab>('balance-transfer')
const workbenchTabs = computed<Array<{ id: WorkbenchTab; label: string; icon: 'gift' | 'chartBar' | 'users' }>>(() => [
  {
    id: 'balance-transfer',
    label: t('adminWorkbench.tabs.balanceTransfer'),
    icon: 'gift'
  },
  {
    id: 'commission',
    label: t('adminWorkbench.tabs.commission'),
    icon: 'chartBar'
  },
  {
    id: 'affiliate-leaderboard',
    label: t('adminWorkbench.tabs.affiliateLeaderboard'),
    icon: 'users'
  }
])

const availableBalance = computed(() => authStore.user?.balance ?? 0)
const form = reactive({
  amount: '',
  count: 1,
  expires_in_days: 30,
  notes: '',
  single_use_per_user: false
})

const generatedResults = ref<GeneratedRedeemCode[]>([])
const generatedCodes = ref<GeneratedRedeemCode[]>([])
const loadingGenerated = ref(false)
const generating = ref(false)
const batchDeletingGeneratedCodes = ref(false)
const deletingIds = ref<number[]>([])
const selectedGeneratedCodeIds = ref<number[]>([])
const errorMessage = ref('')

const generatedPagination = reactive({
  page: 1,
  page_size: 10,
  total: 0
})
const transferFormRef = ref<HTMLElement | null>(null)
const generatedNowHeight = ref<number | null>(null)
let transferFormResizeObserver: ResizeObserver | null = null

const generatedNowPanelStyle = computed(() => {
  if (generatedNowHeight.value == null) {
    return undefined
  }
  return { height: `${generatedNowHeight.value}px` }
})

const totalValue = computed(() => {
  const amount = Number(form.amount)
  const count = Number(form.count)
  if (!Number.isFinite(amount) || !Number.isFinite(count)) {
    return 0
  }
  return Math.max(0, amount) * Math.max(0, count)
})

const deletableGeneratedCodes = computed(() => generatedCodes.value.filter(canDeleteGeneratedCode))

const allDeletableGeneratedCodesSelected = computed(() => {
  if (deletableGeneratedCodes.value.length === 0) {
    return false
  }
  const selected = new Set(selectedGeneratedCodeIds.value)
  return deletableGeneratedCodes.value.every((item) => selected.has(item.id))
})

async function fetchGeneratedCodes() {
  loadingGenerated.value = true
  try {
    const response = await redeemAPI.getGenerated({
      page: generatedPagination.page,
      page_size: generatedPagination.page_size
    })
    generatedCodes.value = response.items
    const visibleDeletableIds = new Set(response.items.filter(canDeleteGeneratedCode).map((item) => item.id))
    selectedGeneratedCodeIds.value = selectedGeneratedCodeIds.value.filter((id) => visibleDeletableIds.has(id))
    generatedPagination.total = response.total
    generatedPagination.page = response.page
    generatedPagination.page_size = response.page_size
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.balanceTransfer.failedToLoad')))
  } finally {
    loadingGenerated.value = false
  }
}

function validateForm(): boolean {
  const amount = Number(form.amount)
  const count = Number(form.count)
  const expiresInDays = Number(form.expires_in_days)
  errorMessage.value = ''

  if (!Number.isFinite(amount) || amount <= 0) {
    errorMessage.value = t('adminWorkbench.balanceTransfer.invalidAmount')
    appStore.showError(errorMessage.value)
    return false
  }
  if (!Number.isInteger(count) || count < 1 || count > 100) {
    errorMessage.value = t('adminWorkbench.balanceTransfer.invalidCount')
    appStore.showError(errorMessage.value)
    return false
  }
  if (amount * count > availableBalance.value) {
    errorMessage.value = t('adminWorkbench.balanceTransfer.insufficientBalance')
    appStore.showError(errorMessage.value)
    return false
  }
  if (!Number.isInteger(expiresInDays) || expiresInDays < 1 || expiresInDays > 3650) {
    errorMessage.value = t('adminWorkbench.balanceTransfer.invalidExpiry')
    appStore.showError(errorMessage.value)
    return false
  }
  return true
}

async function handleGenerate() {
  if (generating.value || !validateForm()) {
    return
  }

  const amount = Number(form.amount)
  const count = Number(form.count)
  const expiresInDays = Number(form.expires_in_days)

  generating.value = true
  try {
    const codes = await redeemAPI.generateBalanceTransferCodes({
      amount,
      count,
      expires_in_days: expiresInDays,
      notes: form.notes.trim(),
      single_use_per_user: form.single_use_per_user
    })
    generatedResults.value = codes
    form.amount = ''
    form.notes = ''
    await authStore.refreshUser()
    generatedPagination.page = 1
    await fetchGeneratedCodes()
    appStore.showSuccess(t('adminWorkbench.balanceTransfer.generated'))
  } catch (error: any) {
    errorMessage.value = extractApiErrorMessage(error, t('adminWorkbench.balanceTransfer.failedToGenerate'))
    appStore.showError(errorMessage.value)
  } finally {
    generating.value = false
  }
}

function canDeleteGeneratedCode(item: GeneratedRedeemCode): boolean {
  const source = item.source?.trim()
  return item.type === 'balance' && (!source || source === 'user_balance_transfer')
}

function getGeneratedStatusLabel(status: GeneratedRedeemCode['status'] | string): string {
  const labels: Record<string, string> = {
    unused: t('adminWorkbench.balanceTransfer.status.unused'),
    used: t('adminWorkbench.balanceTransfer.status.used'),
    expired: t('adminWorkbench.balanceTransfer.status.expired'),
    disabled: t('adminWorkbench.balanceTransfer.status.disabled'),
    active: t('adminWorkbench.balanceTransfer.status.active')
  }
  return labels[status] || status
}

function getGeneratedStatusClass(status: GeneratedRedeemCode['status'] | string): string {
  const base = 'rounded-full px-2 py-0.5 text-xs font-medium'
  if (status === 'unused' || status === 'active') {
    return `${base} bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300`
  }
  if (status === 'used') {
    return `${base} bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300`
  }
  if (status === 'expired') {
    return `${base} bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300`
  }
  return `${base} bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300`
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    appStore.showSuccess(t('common.copied'))
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}

function copyCode(code: string) {
  void copyText(code)
}

function copyGeneratedResults() {
  if (generatedResults.value.length === 0) {
    return
  }
  void copyText(generatedResults.value.map((item) => item.code).join('\n'))
}

async function handleDeleteGeneratedCode(item: GeneratedRedeemCode) {
  if (!canDeleteGeneratedCode(item) || deletingIds.value.includes(item.id)) {
    return
  }
  if (!window.confirm(t('adminWorkbench.balanceTransfer.deleteConfirm'))) {
    return
  }
  deletingIds.value = [...deletingIds.value, item.id]
  try {
    await redeemAPI.deleteGenerated(item.id)
    await authStore.refreshUser()
    await fetchGeneratedCodes()
    selectedGeneratedCodeIds.value = selectedGeneratedCodeIds.value.filter((id) => id !== item.id)
    appStore.showSuccess(t('adminWorkbench.balanceTransfer.deleted'))
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.balanceTransfer.failedToDelete')))
  } finally {
    deletingIds.value = deletingIds.value.filter((id) => id !== item.id)
  }
}

function handleSelectAllGeneratedCodes(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  selectedGeneratedCodeIds.value = checked ? deletableGeneratedCodes.value.map((item) => item.id) : []
}

async function handleDeleteSelectedGeneratedCodes() {
  const deletableIds = new Set(deletableGeneratedCodes.value.map((item) => item.id))
  const ids = selectedGeneratedCodeIds.value.filter((id) => deletableIds.has(id))
  if (ids.length === 0 || batchDeletingGeneratedCodes.value) {
    return
  }
  if (!window.confirm(t('adminWorkbench.balanceTransfer.batchDeleteConfirm'))) {
    return
  }

  batchDeletingGeneratedCodes.value = true
  try {
    await redeemAPI.deleteGeneratedBatch(ids)
    selectedGeneratedCodeIds.value = []
    await authStore.refreshUser()
    generatedPagination.page = 1
    await fetchGeneratedCodes()
    appStore.showSuccess(t('adminWorkbench.balanceTransfer.batchDeleted'))
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.balanceTransfer.failedToBatchDelete')))
  } finally {
    batchDeletingGeneratedCodes.value = false
  }
}

function handleGeneratedPageChange(page: number) {
  generatedPagination.page = page
  void fetchGeneratedCodes()
}

function handleGeneratedPageSizeChange(pageSize: number) {
  generatedPagination.page_size = pageSize
  generatedPagination.page = 1
  void fetchGeneratedCodes()
}

function updateGeneratedNowHeight() {
  if (window.innerWidth < 1024) {
    generatedNowHeight.value = null
    return
  }
  const height = transferFormRef.value?.getBoundingClientRect().height ?? 0
  generatedNowHeight.value = height > 0 ? Math.ceil(height) : null
}

onMounted(() => {
  void fetchGeneratedCodes()
  updateGeneratedNowHeight()
  if (typeof ResizeObserver !== 'undefined' && transferFormRef.value) {
    transferFormResizeObserver = new ResizeObserver(updateGeneratedNowHeight)
    transferFormResizeObserver.observe(transferFormRef.value)
  }
  window.addEventListener('resize', updateGeneratedNowHeight)
})

onBeforeUnmount(() => {
  transferFormResizeObserver?.disconnect()
  window.removeEventListener('resize', updateGeneratedNowHeight)
})
</script>
