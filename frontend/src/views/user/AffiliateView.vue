<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div
            data-stat="rate"
            :data-featured="featuredStat === 'rate' ? 'true' : undefined"
            class="card affiliate-stat p-5"
            :class="{ 'affiliate-stat--featured': featuredStat === 'rate' }"
          >
            <span v-if="featuredStat === 'rate'" class="sr-only">
              {{ t('affiliate.tiers.identity.featuredMetric') }}
            </span>
            <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="dollar" size="sm" class="text-primary-500" />
              {{ t('affiliate.stats.rebateRate') }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formattedRebateRate }}<span class="ml-0.5 text-base font-medium">%</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('affiliate.stats.rebateRateHint') }}
            </p>
          </div>
          <div
            data-stat="acquisition"
            :data-metric="acquisitionMetric"
            :data-featured="featuredStat === 'acquisition' ? 'true' : undefined"
            class="card affiliate-stat p-5"
            :class="{ 'affiliate-stat--featured': featuredStat === 'acquisition' }"
          >
            <span v-if="featuredStat === 'acquisition'" class="sr-only">
              {{ t('affiliate.tiers.identity.featuredMetric') }}
            </span>
            <div data-acquisition="primary">
              <p class="text-sm text-gray-500 dark:text-dark-400">
                {{ t(acquisitionMetric === 'qualified' ? 'affiliate.tiers.qualifiedCount' : 'affiliate.stats.invitedUsers') }}
              </p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ formatCount(acquisitionMetric === 'qualified' ? detail.qualified_invitee_count : detail.aff_count) }}
              </p>
            </div>
            <p
              data-acquisition="secondary"
              class="mt-1 flex flex-wrap items-baseline gap-x-1.5 text-xs text-gray-400 dark:text-dark-500"
            >
              <span>
                {{ t(acquisitionMetric === 'qualified' ? 'affiliate.stats.invitedUsers' : 'affiliate.tiers.qualifiedCount') }}
              </span>
              <strong class="font-medium text-gray-500 dark:text-dark-400">
                {{ formatCount(acquisitionMetric === 'qualified' ? detail.aff_count : detail.qualified_invitee_count) }}
              </strong>
            </p>
          </div>
          <div
            data-stat="available"
            class="card affiliate-stat p-5"
          >
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
          </div>
          <div
            data-stat="history"
            :data-featured="featuredStat === 'history' ? 'true' : undefined"
            class="card affiliate-stat p-5"
            :class="{ 'affiliate-stat--featured': featuredStat === 'history' }"
          >
            <span v-if="featuredStat === 'history'" class="sr-only">
              {{ t('affiliate.tiers.identity.featuredMetric') }}
            </span>
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <AffiliateTierIdentity
          :detail="detail"
          :next-tier="nextTier"
          :progress="tierProgress"
          :formatted-rate="formattedRebateRate"
        />

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.description') }}</p>

          <div class="mt-5 grid grid-cols-1 gap-4 md:grid-cols-2">
            <div class="min-w-0 space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="min-w-0 space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-lg border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
            <p class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('affiliate.tips.title') }}</p>
            <ul class="mt-2 space-y-1 text-sm text-primary-700 dark:text-primary-300">
              <li>1. {{ t('affiliate.tips.line1') }}</li>
              <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>3. {{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
            </ul>
          </div>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="transferring || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="dollar" size="sm" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>
          <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('affiliate.transfer.empty') }}
          </p>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invitees.title') }}</h3>
          <div v-if="detail.invitees.length === 0" class="mt-4 rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('affiliate.invitees.empty') }}
          </div>
          <template v-else>
            <div data-testid="invitees-desktop" class="mt-4 hidden md:block">
            <table class="w-full table-fixed text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.email') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.paymentProgress') }}</th>
                  <th class="px-3 py-2 font-medium text-right">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in detail.invitees"
                  :key="item.user_id"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                >
                  <td class="break-all px-3 py-3 text-gray-900 dark:text-white">{{ item.email || '-' }}</td>
                  <td class="break-all px-3 py-3 text-gray-700 dark:text-gray-300">{{ item.username || '-' }}</td>
                  <td class="px-3 py-3">
                    <p class="font-medium text-gray-800 dark:text-gray-200">{{ formatCurrency(item.qualifying_payment_amount) }} / {{ formatCurrency(detail.qualification_amount) }}</p>
                    <p :class="item.qualified ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'" class="mt-0.5 text-xs">
                      {{ item.qualified ? t('affiliate.invitees.qualified') : t('affiliate.invitees.inProgress') }}
                    </p>
                  </td>
                  <td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate) }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
            </div>

            <div data-testid="invitees-mobile" class="mt-4 divide-y divide-gray-100 border-y border-gray-100 md:hidden dark:divide-dark-800 dark:border-dark-800">
              <article v-for="item in detail.invitees" :key="item.user_id" class="min-w-0 py-4 first:pt-2 last:pb-2">
                <div class="flex min-w-0 items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="break-all text-sm font-medium text-gray-900 dark:text-white">{{ item.email || '-' }}</p>
                    <p class="mt-0.5 break-all text-xs text-gray-500 dark:text-dark-400">{{ item.username || '-' }}</p>
                  </div>
                  <span :class="item.qualified ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'" class="shrink-0 rounded-md px-2 py-1 text-xs font-medium">
                    {{ item.qualified ? t('affiliate.invitees.qualified') : t('affiliate.invitees.inProgress') }}
                  </span>
                </div>
                <div class="mt-3 flex min-w-0 flex-wrap items-end justify-between gap-x-3 gap-y-2 text-xs">
                  <div class="min-w-0">
                    <p class="text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.paymentProgress') }}</p>
                    <p class="mt-0.5 font-medium text-gray-800 dark:text-gray-200">{{ formatCurrency(item.qualifying_payment_amount) }} / {{ formatCurrency(detail.qualification_amount) }}</p>
                  </div>
                  <div class="text-right">
                    <p class="text-gray-500 dark:text-dark-400">{{ t('affiliate.invitees.columns.rebate') }}</p>
                    <p class="mt-0.5 font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate) }}</p>
                  </div>
                </div>
                <p class="mt-2 text-xs text-gray-400 dark:text-dark-500">{{ formatDateTime(item.created_at) || '-' }}</p>
              </article>
            </div>
          </template>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AffiliateTierIdentity from '@/components/affiliate/AffiliateTierIdentity.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { getAffiliateTierPresentation } from '@/config/affiliateTierPresentation'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

const nextTier = computed(() => {
  if (!detail.value?.next_level_invitee_threshold) return null
  return detail.value.tiers.find((tier) => tier.min_qualified_invitees === detail.value?.next_level_invitee_threshold) ?? null
})

const tierProgress = computed(() => {
  if (!detail.value || !nextTier.value) return 100
  const currentTier = detail.value.tiers.find((tier) => tier.level === detail.value?.automatic_level)
  const floor = currentTier?.min_qualified_invitees ?? 0
  const span = nextTier.value.min_qualified_invitees - floor
  if (span <= 0) return 100
  return Math.min(100, Math.max(0, ((detail.value.qualified_invitee_count - floor) / span) * 100))
})

const featuredMetric = computed(() => {
  return getAffiliateTierPresentation(detail.value?.automatic_level).featuredMetric
})

const acquisitionMetric = computed(() => featuredMetric.value === 'qualified' ? 'qualified' : 'invited')
const featuredStat = computed(() => {
  return featuredMetric.value === 'invited' || featuredMetric.value === 'qualified'
    ? 'acquisition'
    : featuredMetric.value
})

function formatCount(value: number): string {
  const normalized = Number.isFinite(value) ? Math.floor(Math.max(0, value)) : 0
  return normalized.toLocaleString()
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>

<style scoped>
.affiliate-stat--featured {
  border-color: rgb(34 211 238 / 0.6);
  background: linear-gradient(135deg, rgb(239 246 255 / 0.75), rgb(236 254 255 / 0.55));
  box-shadow: inset 0 0 0 1px rgb(14 165 233 / 0.08), 0 5px 16px rgb(14 165 233 / 0.08);
}

:global(.dark) .affiliate-stat--featured {
  border-color: rgb(14 116 144 / 0.7);
  background: linear-gradient(135deg, rgb(23 37 84 / 0.24), rgb(8 51 68 / 0.2));
  box-shadow: inset 0 0 0 1px rgb(34 211 238 / 0.08), 0 5px 16px rgb(6 182 212 / 0.06);
}
</style>
