<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6 px-4 py-6 sm:px-6 lg:px-8">
      <header class="flex flex-col gap-4 border-b border-gray-200 pb-5 dark:border-dark-700 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">
            {{ t('adminWorkbench.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('adminWorkbench.description') }}
          </p>
        </div>
        <div class="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-right dark:border-blue-500/20 dark:bg-blue-500/10">
          <p class="text-xs font-medium text-blue-600 dark:text-blue-300">
            {{ t('adminWorkbench.currentBalance') }}
          </p>
          <p class="mt-1 text-2xl font-semibold tabular-nums text-blue-950 dark:text-blue-50">
            ${{ availableBalance.toFixed(2) }}
          </p>
        </div>
      </header>

      <section class="grid gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <form
          data-test="workbench-transfer-form"
          class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
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

        <section class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-4 flex items-center justify-between gap-3">
            <div>
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

          <div v-if="generatedResults.length === 0" class="flex min-h-44 items-center justify-center rounded-lg bg-gray-50 text-sm text-gray-500 dark:bg-dark-800/70 dark:text-dark-400">
            {{ t('adminWorkbench.balanceTransfer.noGeneratedNow') }}
          </div>
          <div v-else class="max-h-64 space-y-2 overflow-auto">
            <div
              v-for="item in generatedResults"
              :key="item.id"
              class="rounded-lg border border-blue-100 bg-blue-50/70 px-3 py-2 font-mono text-sm text-blue-950 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-100"
            >
              {{ item.code }}
            </div>
          </div>
        </section>
      </section>

      <section class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-950 dark:text-white">
              {{ t('adminWorkbench.balanceTransfer.generatedList') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('adminWorkbench.balanceTransfer.generatedListHint') }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="loadingGenerated" @click="fetchGeneratedCodes">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingGenerated }" />
            <span>{{ t('common.refresh') }}</span>
          </button>
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
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="break-all font-mono text-sm font-semibold text-gray-950 dark:text-white">{{ item.code }}</span>
                <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">
                  ${{ item.value.toFixed(2) }}
                </span>
                <span v-if="item.single_use_per_user" class="rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
                  {{ t('adminWorkbench.balanceTransfer.singleUseBadge') }}
                </span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ formatDateTime(item.created_at) }}
                <span v-if="item.expires_at"> · {{ t('adminWorkbench.balanceTransfer.expiresAt') }} {{ formatDateTime(item.expires_at) }}</span>
              </p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { redeemAPI, type GeneratedRedeemCode } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

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
const deletingIds = ref<number[]>([])
const errorMessage = ref('')

const generatedPagination = reactive({
  page: 1,
  page_size: 10,
  total: 0
})

const totalValue = computed(() => {
  const amount = Number(form.amount)
  const count = Number(form.count)
  if (!Number.isFinite(amount) || !Number.isFinite(count)) {
    return 0
  }
  return Math.max(0, amount) * Math.max(0, count)
})

async function fetchGeneratedCodes() {
  loadingGenerated.value = true
  try {
    const response = await redeemAPI.getGenerated({
      page: generatedPagination.page,
      page_size: generatedPagination.page_size
    })
    generatedCodes.value = response.items
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
  return item.used_by == null && (item.status === 'unused' || item.status === 'expired')
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
    appStore.showSuccess(t('adminWorkbench.balanceTransfer.deleted'))
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.balanceTransfer.failedToDelete')))
  } finally {
    deletingIds.value = deletingIds.value.filter((id) => id !== item.id)
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

onMounted(() => {
  void fetchGeneratedCodes()
})
</script>
