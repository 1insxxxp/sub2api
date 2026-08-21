<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-6">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('manager.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('manager.description') }}</p>
        </div>
        <div class="rounded-lg border border-primary-100 bg-primary-50 px-4 py-3 text-right dark:border-primary-900/40 dark:bg-primary-950/30">
          <p class="text-xs font-medium text-primary-700 dark:text-primary-300">{{ t('manager.currentBalance') }}</p>
          <p class="mt-1 text-xl font-semibold text-primary-900 dark:text-primary-100">${{ currentBalance.toFixed(2) }}</p>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,420px)_1fr]">
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <div class="flex items-center gap-3">
              <span class="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
                <Icon name="gift" size="md" />
              </span>
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('manager.balanceToCodes') }}</h2>
                <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('manager.balanceToCodesDesc') }}</p>
              </div>
            </div>
          </div>

          <form class="space-y-5 p-6" @submit.prevent="handleGenerate">
            <div>
              <label class="input-label">{{ t('manager.singleValue') }}</label>
              <input
                v-model.number="form.value"
                type="number"
                min="0.01"
                step="0.01"
                class="input"
              />
            </div>
            <div>
              <label class="input-label">{{ t('manager.count') }}</label>
              <input
                v-model.number="form.count"
                type="number"
                min="1"
                max="100"
                step="1"
                class="input"
              />
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('manager.totalValue') }}</p>
                <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">${{ totalValue.toFixed(2) }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('manager.remainingBalance') }}</p>
                <p
                  class="mt-1 text-lg font-semibold"
                  :class="remainingBalance < 0 ? 'text-red-600 dark:text-red-400' : 'text-gray-900 dark:text-white'"
                >
                  ${{ remainingBalance.toFixed(2) }}
                </p>
              </div>
            </div>

            <button type="submit" class="btn btn-primary w-full" :disabled="!canSubmit || submitting">
              <Icon name="refresh" size="sm" class="mr-2" :class="submitting ? 'animate-spin' : ''" />
              {{ submitting ? t('manager.generating') : t('manager.generate') }}
            </button>
          </form>
        </div>

        <div class="card min-h-[360px]">
          <div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('manager.generatedCodes') }}</h2>
            <button
              v-if="generatedCodes.length"
              type="button"
              class="btn btn-secondary"
              @click="copyAllCodes"
            >
              <Icon name="copy" size="sm" class="mr-2" />
              {{ t('manager.copyAll') }}
            </button>
          </div>

          <div class="p-6">
            <div v-if="generatedCodes.length" class="space-y-3">
              <div
                v-for="code in generatedCodes"
                :key="code.code"
                class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800"
              >
                <div class="min-w-0">
                  <code class="block truncate font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ code.code }}</code>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">${{ code.value.toFixed(2) }}</p>
                </div>
                <button
                  type="button"
                  class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-white hover:text-primary-600 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-primary-300"
                  :title="t('keys.copyToClipboard')"
                  @click="copyCode(code.code)"
                >
                  <Icon name="copy" size="sm" />
                </button>
              </div>
            </div>

            <div v-else class="flex min-h-[240px] flex-col items-center justify-center text-center">
              <span class="flex h-14 w-14 items-center justify-center rounded-xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-dark-500">
                <Icon name="inbox" size="lg" />
              </span>
              <p class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('manager.noCodes') }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { redeemAPI } from '@/api'
import { useAppStore, useAuthStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import type { RedeemCode } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const form = reactive({
  value: 10,
  count: 1
})
const submitting = ref(false)
const generatedCodes = ref<RedeemCode[]>([])

const currentBalance = computed(() => authStore.user?.balance ?? 0)
const totalValue = computed(() => {
  const value = Number(form.value) || 0
  const count = Number(form.count) || 0
  return Number((value * count).toFixed(2))
})
const remainingBalance = computed(() => Number((currentBalance.value - totalValue.value).toFixed(2)))
const canSubmit = computed(() => {
  return form.value > 0 && form.count >= 1 && form.count <= 100 && totalValue.value <= currentBalance.value
})
const generatedCodesText = computed(() => generatedCodes.value.map((code) => code.code).join('\n'))

async function handleGenerate() {
  if (form.value <= 0) {
    appStore.showError(t('manager.amountRequired'))
    return
  }
  if (form.count < 1 || form.count > 100) {
    appStore.showError(t('manager.countRequired'))
    return
  }
  if (totalValue.value > currentBalance.value) {
    appStore.showError(t('manager.insufficientBalance'))
    return
  }

  submitting.value = true
  try {
    const result = await redeemAPI.convertBalanceToRedeemCodes(form.value, form.count)
    generatedCodes.value = result.codes
    await authStore.refreshUser()
    appStore.showSuccess(t('manager.generateSuccess'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('manager.generateFailed'))
  } finally {
    submitting.value = false
  }
}

function copyCode(code: string) {
  copyToClipboard(code, t('manager.copied'))
}

function copyAllCodes() {
  copyToClipboard(generatedCodesText.value, t('manager.copied'))
}
</script>
