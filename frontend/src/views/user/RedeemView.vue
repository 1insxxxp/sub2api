<template>
  <AppLayout>
    <div class="redeem-shell mx-auto w-full max-w-4xl space-y-5 sm:space-y-6">
      <!-- Current Balance Card -->
      <section class="redeem-balance-card">
        <div class="redeem-balance-orbit"></div>
        <div class="relative z-10 flex flex-col gap-5 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <div class="redeem-balance-icon">
              <Icon name="creditCard" size="xl" class="text-white" />
            </div>
            <p class="redeem-eyebrow">{{ t('redeem.currentBalance') }}</p>
            <p class="redeem-balance-value">
              ${{ user?.balance?.toFixed(2) || '0.00' }}
            </p>
          </div>
          <div class="redeem-balance-chip">
            <span>{{ t('redeem.concurrency') }}</span>
            <strong>{{ user?.concurrency || 0 }}</strong>
            <small>{{ t('redeem.requests') }}</small>
          </div>
        </div>
      </section>

      <!-- Redeem Form -->
      <section class="brand-surface redeem-code-panel">
        <div class="p-5 sm:p-6">
          <form @submit.prevent="handleRedeem" class="redeem-form">
            <div class="redeem-panel-header">
              <div class="brand-floating-icon redeem-panel-icon">
                <Icon name="gift" size="md" />
              </div>
              <div class="min-w-0">
                <h2 class="text-base font-semibold text-slate-950 dark:text-white">
                  {{ t('redeem.redeemCodeLabel') }}
                </h2>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                  {{ t('redeem.redeemCodeHint') }}
                </p>
              </div>
            </div>
            <div>
              <label for="code" class="sr-only">
                {{ t('redeem.redeemCodeLabel') }}
              </label>
              <div class="redeem-input-wrap">
                <div class="redeem-input-icon">
                  <Icon name="gift" size="md" />
                </div>
                <input
                  id="code"
                  v-model="redeemCode"
                  type="text"
                  required
                  :placeholder="t('redeem.redeemCodePlaceholder')"
                  :disabled="submitting"
                  class="redeem-input"
                />
              </div>
            </div>

            <button
              type="submit"
              :disabled="!redeemCode || submitting"
              class="btn btn-primary redeem-submit w-full"
            >
              <svg
                v-if="submitting"
                class="-ml-1 mr-2 h-5 w-5 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              <Icon v-else name="checkCircle" size="md" class="mr-2" />
              {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
            </button>
          </form>
        </div>
      </section>

      <!-- Success Message -->
      <transition name="fade">
        <div
          v-if="redeemResult"
          class="brand-surface redeem-feedback-panel redeem-feedback-success"
        >
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-100 dark:bg-emerald-900/30"
              >
                <Icon name="checkCircle" size="md" class="text-emerald-600 dark:text-emerald-400" />
              </div>
              <div class="flex-1">
                <h3 class="text-sm font-semibold text-emerald-800 dark:text-emerald-300">
                  {{ t('redeem.redeemSuccess') }}
                </h3>
                <div class="mt-2 text-sm text-emerald-700 dark:text-emerald-400">
                  <p>{{ redeemResult.message }}</p>
                  <div class="mt-3 space-y-1">
                    <p v-if="redeemResult.type === 'balance'" class="font-medium">
                      {{ t('redeem.added') }}: ${{ redeemResult.value.toFixed(2) }}
                    </p>
                    <p v-else-if="redeemResult.type === 'concurrency'" class="font-medium">
                      {{ t('redeem.added') }}: {{ redeemResult.value }}
                      {{ t('redeem.concurrentRequests') }}
                    </p>
                    <p v-else-if="redeemResult.type === 'subscription'" class="font-medium">
                      {{ t('redeem.subscriptionAssigned') }}
                      <span v-if="redeemResult.group_name"> - {{ redeemResult.group_name }}</span>
                      <span v-if="redeemResult.validity_days">
                        ({{
                          t('redeem.subscriptionDays', { days: redeemResult.validity_days })
                        }})</span
                      >
                    </p>
                    <p v-if="redeemResult.new_balance !== undefined">
                      {{ t('redeem.newBalance') }}:
                      <span class="font-semibold">${{ redeemResult.new_balance.toFixed(2) }}</span>
                    </p>
                    <p v-if="redeemResult.new_concurrency !== undefined">
                      {{ t('redeem.newConcurrency') }}:
                      <span class="font-semibold"
                        >{{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}</span
                      >
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </transition>

      <!-- Error Message -->
      <transition name="fade">
        <div
          v-if="errorMessage"
          class="brand-surface redeem-feedback-panel redeem-feedback-error"
        >
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div
                class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-red-100 dark:bg-red-900/30"
              >
                <Icon
                  name="exclamationCircle"
                  size="md"
                  class="text-red-600 dark:text-red-400"
                />
              </div>
              <div class="flex-1">
                <h3 class="text-sm font-semibold text-red-800 dark:text-red-300">
                  {{ t('redeem.redeemFailed') }}
                </h3>
                <p class="mt-2 text-sm text-red-700 dark:text-red-400">
                  {{ errorMessage }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </transition>

      <!-- Information Card -->
      <section class="brand-surface brand-rail redeem-info-panel">
        <div class="p-5 pl-7 sm:p-6 sm:pl-8">
          <div class="flex items-start gap-4">
            <div class="redeem-info-icon">
              <Icon name="infoCircle" size="md" class="text-primary-600 dark:text-primary-400" />
            </div>
            <div class="flex-1">
              <h3 class="text-sm font-semibold text-primary-800 dark:text-primary-300">
                {{ t('redeem.aboutCodes') }}
              </h3>
              <ul
                class="mt-2 list-inside list-disc space-y-1 text-sm text-primary-700 dark:text-primary-400"
              >
                <li>{{ t('redeem.codeRule1') }}</li>
                <li>{{ t('redeem.codeRule2') }}</li>
                <li>
                  {{ t('redeem.codeRule3') }}
                  <span
                    v-if="contactInfo"
                    class="ml-1.5 inline-flex items-center rounded-md bg-primary-200/50 px-2 py-0.5 text-xs font-medium text-primary-800 dark:bg-primary-800/40 dark:text-primary-200"
                  >
                    {{ contactInfo }}
                  </span>
                </li>
                <li>{{ t('redeem.codeRule4') }}</li>
              </ul>
            </div>
          </div>
        </div>
      </section>

      <!-- Recent Activity -->
      <section class="brand-surface redeem-history-panel">
        <div class="redeem-history-header">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('redeem.recentActivity') }}
          </h2>
        </div>
        <div class="p-6">
          <!-- Loading State -->
          <div v-if="loadingHistory" class="flex items-center justify-center py-8">
            <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
          </div>

          <!-- History List -->
          <div v-else-if="history.length > 0" class="space-y-3">
            <div
              v-for="item in history"
              :key="item.id"
              class="brand-floating-card redeem-history-row"
            >
              <div class="flex items-center gap-4">
                <div
                  :class="[
                    'flex h-10 w-10 items-center justify-center rounded-xl',
                    isBalanceType(item.type)
                      ? item.value >= 0
                        ? 'bg-emerald-100 dark:bg-emerald-900/30'
                        : 'bg-red-100 dark:bg-red-900/30'
                      : isSubscriptionType(item.type)
                        ? 'bg-purple-100 dark:bg-purple-900/30'
                        : item.value >= 0
                          ? 'bg-blue-100 dark:bg-blue-900/30'
                          : 'bg-orange-100 dark:bg-orange-900/30'
                  ]"
                >
                  <!-- 余额类型图标 -->
                  <Icon
                    v-if="isBalanceType(item.type)"
                    name="dollar"
                    size="md"
                    :class="
                      item.value >= 0
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-red-600 dark:text-red-400'
                    "
                  />
                  <!-- 订阅类型图标 -->
                  <Icon
                    v-else-if="isSubscriptionType(item.type)"
                    name="badge"
                    size="md"
                    class="text-purple-600 dark:text-purple-400"
                  />
                  <!-- 并发类型图标 -->
                  <Icon
                    v-else
                    name="bolt"
                    size="md"
                    :class="
                      item.value >= 0
                        ? 'text-blue-600 dark:text-blue-400'
                        : 'text-orange-600 dark:text-orange-400'
                    "
                  />
                </div>
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ getHistoryItemTitle(item) }}
                  </p>
                  <p class="text-xs text-gray-500 dark:text-dark-400">
                    {{ formatDateTime(item.used_at) }}
                  </p>
                </div>
              </div>
              <div class="text-right">
                <p
                  :class="[
                    'text-sm font-semibold',
                    isBalanceType(item.type)
                      ? item.value >= 0
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-red-600 dark:text-red-400'
                      : isSubscriptionType(item.type)
                        ? 'text-purple-600 dark:text-purple-400'
                        : item.value >= 0
                          ? 'text-blue-600 dark:text-blue-400'
                          : 'text-orange-600 dark:text-orange-400'
                  ]"
                >
                  {{ formatHistoryValue(item) }}
                </p>
                <p
                  v-if="!isAdminAdjustment(item.type)"
                  class="font-mono text-xs text-gray-400 dark:text-dark-500"
                >
                  {{ item.code.slice(0, 8) }}...
                </p>
                <p v-else class="text-xs text-gray-400 dark:text-dark-500">
                  {{ t('redeem.adminAdjustment') }}
                </p>
                <!-- Display notes for admin adjustments -->
                <p
                  v-if="item.notes"
                  class="mt-1 text-xs text-gray-500 dark:text-dark-400 italic max-w-[200px] truncate"
                  :title="item.notes"
                >
                  {{ item.notes }}
                </p>
              </div>
            </div>
          </div>

          <!-- Empty State -->
          <div v-else class="empty-state py-8">
            <div
              class="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-800"
            >
              <Icon name="clock" size="xl" class="text-gray-400 dark:text-dark-500" />
            </div>
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('redeem.historyWillAppear') }}
            </p>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, authAPI, type RedeemHistoryItem } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_name?: string
  validity_days?: number
} | null>(null)
const errorMessage = ref('')

// History data
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const contactInfo = ref('')

// Helper functions for history display
const isBalanceType = (type: string) => {
  return type === 'balance' || type === 'admin_balance' || type === 'checkin_reward'
}

const isSubscriptionType = (type: string) => {
  return type === 'subscription'
}

const isAdminAdjustment = (type: string) => {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

const getHistoryItemTitle = (item: RedeemHistoryItem) => {
  if (item.type === 'balance') {
    return t('redeem.balanceAddedRedeem')
  } else if (item.type === 'admin_balance') {
    return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
  } else if (item.type === 'checkin_reward') {
    return t('redeem.checkinReward')
  } else if (item.type === 'concurrency') {
    return t('redeem.concurrencyAddedRedeem')
  } else if (item.type === 'admin_concurrency') {
    return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
  } else if (item.type === 'subscription') {
    return t('redeem.subscriptionAssigned')
  }
  return t('common.unknown')
}

const formatHistoryValue = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  } else if (isSubscriptionType(item.type)) {
    // 订阅类型显示有效天数和分组名称
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}${t('redeem.days')} - ${groupName}` : `${days}${t('redeem.days')}`
  } else {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}${item.value} ${t('redeem.requests')}`
  }
}

const fetchHistory = async () => {
  loadingHistory.value = true
  try {
    history.value = await redeemAPI.getHistory()
  } catch (error) {
    console.error('Failed to fetch history:', error)
  } finally {
    loadingHistory.value = false
  }
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    await authStore.refreshUser()

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh history
    await fetchHistory()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')

    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchHistory()
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})
</script>

<style scoped>
.redeem-shell {
  padding-bottom: 2rem;
}

.redeem-balance-card {
  position: relative;
  overflow: hidden;
  border-radius: 1.5rem;
  padding: 1.5rem;
  color: white;
  background:
    radial-gradient(circle at 14% 8%, rgba(255, 255, 255, 0.24), transparent 28%),
    radial-gradient(circle at 90% 0%, rgba(6, 182, 212, 0.42), transparent 32%),
    linear-gradient(135deg, var(--brand-700), var(--brand-500) 54%, var(--brand-cyan));
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.32) inset,
    0 24px 58px rgba(37, 99, 235, 0.24);
}

.redeem-balance-card::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(115deg, rgba(255, 255, 255, 0.18), transparent 32%),
    linear-gradient(180deg, transparent, rgba(15, 23, 42, 0.12));
}

.redeem-balance-orbit {
  position: absolute;
  right: -5rem;
  top: -5.5rem;
  width: 15rem;
  height: 15rem;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  background: rgba(255, 255, 255, 0.08);
}

.redeem-balance-icon {
  display: inline-flex;
  height: 4.25rem;
  width: 4.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 1.25rem;
  background: rgba(255, 255, 255, 0.18);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.32) inset;
  backdrop-filter: blur(12px);
}

.redeem-eyebrow {
  margin-top: 1.25rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.78);
}

.redeem-balance-value {
  margin-top: 0.35rem;
  font-size: clamp(2.35rem, 5vw, 4rem);
  font-weight: 800;
  line-height: 1;
  letter-spacing: 0;
  font-variant-numeric: tabular-nums;
}

.redeem-balance-chip {
  display: inline-flex;
  width: fit-content;
  align-items: baseline;
  gap: 0.45rem;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.24);
  padding: 0.7rem 0.9rem;
  background: rgba(255, 255, 255, 0.13);
  color: rgba(255, 255, 255, 0.78);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.2) inset;
  backdrop-filter: blur(14px);
}

.redeem-balance-chip strong {
  color: white;
  font-size: 1.25rem;
  font-variant-numeric: tabular-nums;
}

.redeem-balance-chip small {
  font-size: 0.75rem;
}

.redeem-code-panel,
.redeem-info-panel,
.redeem-history-panel,
.redeem-feedback-panel {
  border-radius: 1.25rem;
}

.redeem-form {
  display: grid;
  gap: 1.25rem;
}

.redeem-panel-header {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.redeem-panel-icon {
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
}

.redeem-input-wrap {
  position: relative;
}

.redeem-input-icon {
  pointer-events: none;
  position: absolute;
  bottom: 0;
  left: 0;
  top: 0;
  display: flex;
  align-items: center;
  padding-left: 1rem;
  color: var(--brand-500);
}

.redeem-input {
  width: 100%;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.88);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(239, 246, 255, 0.74)),
    white;
  padding: 0.95rem 1rem 0.95rem 3rem;
  font-size: 1rem;
  font-weight: 600;
  color: rgb(15, 23, 42);
  outline: none;
  transition:
    border-color 180ms ease,
    box-shadow 180ms ease,
    background-color 180ms ease;
}

.redeem-input::placeholder {
  color: rgb(148, 163, 184);
  font-weight: 500;
}

.redeem-input:focus {
  border-color: rgba(var(--brand-rgb), 0.58);
  box-shadow:
    0 0 0 3px rgba(var(--brand-rgb), 0.12),
    0 14px 34px rgba(37, 99, 235, 0.1);
}

.redeem-input:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.redeem-submit {
  min-height: 3.15rem;
  border-radius: 1rem;
}

.redeem-info-icon {
  display: flex;
  height: 2.75rem;
  width: 2.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.74);
  background: rgba(239, 246, 255, 0.86);
}

.redeem-history-header {
  border-bottom: 1px solid rgba(191, 219, 254, 0.48);
  padding: 1rem 1.5rem;
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.72), rgba(236, 254, 255, 0.5));
}

.redeem-history-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  transition:
    border-color 180ms ease,
    transform 180ms ease,
    box-shadow 180ms ease;
}

.redeem-history-row:hover {
  transform: translateY(-1px);
  border-color: rgba(var(--brand-rgb), 0.34);
  box-shadow: 0 12px 28px rgba(37, 99, 235, 0.08);
}

.redeem-feedback-success {
  border-color: rgba(16, 185, 129, 0.3);
  background:
    radial-gradient(circle at 0% 0%, rgba(16, 185, 129, 0.1), transparent 34%),
    linear-gradient(180deg, rgba(240, 253, 244, 0.98), rgba(255, 255, 255, 0.96));
}

.redeem-feedback-error {
  border-color: rgba(239, 68, 68, 0.28);
  background:
    radial-gradient(circle at 0% 0%, rgba(239, 68, 68, 0.1), transparent 34%),
    linear-gradient(180deg, rgba(254, 242, 242, 0.98), rgba(255, 255, 255, 0.96));
}

.dark .redeem-input {
  border-color: rgba(96, 165, 250, 0.22);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.94), rgba(8, 13, 28, 0.9)),
    rgba(15, 23, 42, 0.86);
  color: white;
}

.dark .redeem-input::placeholder {
  color: rgb(100, 116, 139);
}

.dark .redeem-info-icon {
  border-color: rgba(96, 165, 250, 0.18);
  background: rgba(37, 99, 235, 0.16);
}

.dark .redeem-history-header {
  border-color: rgba(96, 165, 250, 0.16);
  background: linear-gradient(135deg, rgba(30, 64, 175, 0.16), rgba(8, 145, 178, 0.08));
}

.dark .redeem-feedback-success {
  border-color: rgba(52, 211, 153, 0.24);
  background:
    radial-gradient(circle at 0% 0%, rgba(16, 185, 129, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(6, 78, 59, 0.2), rgba(2, 6, 23, 0.9));
}

.dark .redeem-feedback-error {
  border-color: rgba(248, 113, 113, 0.24);
  background:
    radial-gradient(circle at 0% 0%, rgba(239, 68, 68, 0.16), transparent 34%),
    linear-gradient(180deg, rgba(127, 29, 29, 0.2), rgba(2, 6, 23, 0.9));
}

@media (max-width: 640px) {
  .redeem-balance-card {
    border-radius: 1.25rem;
    padding: 1.25rem;
  }

  .redeem-history-row {
    align-items: flex-start;
  }
}

@media (prefers-reduced-motion: reduce) {
  .redeem-history-row,
  .redeem-input {
    transition: none;
  }

  .redeem-history-row:hover {
    transform: none;
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
