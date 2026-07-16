<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <section
          data-testid="affiliate-promotion-console"
          class="affiliate-console relative overflow-hidden rounded-lg border border-cyan-200/80 bg-white p-5 shadow-sm dark:border-cyan-900/50 dark:bg-dark-900/80"
        >
          <div class="affiliate-console__grid" aria-hidden="true"></div>
          <div class="relative z-[1] grid gap-5 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.35fr)] lg:items-end">
            <div class="min-w-0">
              <p class="flex items-center gap-2 text-xs font-semibold uppercase text-cyan-700 dark:text-cyan-300">
                <Icon name="sparkles" size="sm" />
                {{ t('affiliate.campaign.eyebrow') }}
              </p>
              <h1 class="mt-2 text-2xl font-semibold leading-tight text-gray-950 dark:text-white sm:text-3xl">
                {{ t('affiliate.campaign.title') }}
              </h1>
              <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-600 dark:text-dark-300">
                {{ t('affiliate.campaign.subtitle') }}
              </p>
              <div class="mt-4 flex min-w-0 flex-wrap items-center gap-2">
                <span class="inline-flex items-center gap-1.5 rounded-md border border-cyan-200 bg-cyan-50 px-2.5 py-1 text-xs font-medium text-cyan-700 dark:border-cyan-800 dark:bg-cyan-950/40 dark:text-cyan-300">
                  <Icon name="badge" size="xs" />
                  {{ currentTierLabel }}
                </span>
                <span class="min-w-0 rounded-md border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 dark:border-blue-900/70 dark:bg-blue-950/40 dark:text-blue-300">
                  {{ nextTierCallout }}
                </span>
              </div>
            </div>

            <div class="grid gap-3 sm:grid-cols-3">
              <article
                v-for="(step, index) in rewardSteps"
                :key="step.title"
                data-testid="affiliate-reward-step"
                class="affiliate-step min-w-0 rounded-lg border border-cyan-200/70 bg-white/80 p-4 dark:border-cyan-900/50 dark:bg-dark-950/50"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-cyan-500 text-sm font-semibold text-white shadow-sm shadow-cyan-500/30">
                    {{ index + 1 }}
                  </span>
                  <Icon :name="step.icon" size="sm" class="text-cyan-600 dark:text-cyan-300" />
                </div>
                <h2 class="mt-3 text-sm font-semibold text-gray-900 dark:text-white">
                  {{ step.title }}
                </h2>
                <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
                  {{ step.description }}
                </p>
              </article>
            </div>
          </div>
        </section>

        <div class="grid gap-5 lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.35fr)]">
          <section
            data-testid="affiliate-invite-tools"
            class="card affiliate-panel affiliate-tools p-6"
          >
            <div class="flex items-start justify-between gap-4">
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.campaign.toolsTitle') }}</h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.campaign.toolsDescription') }}</p>
              </div>
              <Icon name="link" size="lg" class="shrink-0 text-cyan-500" />
            </div>

            <div class="mt-5 space-y-4">
              <div class="min-w-0 space-y-2">
                <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
                <div class="affiliate-copy-row">
                  <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ detail.aff_code }}</code>
                  <button class="btn btn-secondary btn-sm" @click="copyCode">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.copyCode') }}</span>
                  </button>
                </div>
              </div>

              <div class="min-w-0 space-y-2">
                <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
                <div class="affiliate-copy-row">
                  <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                  <button class="btn btn-secondary btn-sm" @click="copyInviteLink">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.copyLink') }}</span>
                  </button>
                </div>
              </div>

              <div class="affiliate-pitch-card rounded-lg border border-cyan-200/80 bg-cyan-50/70 p-3 dark:border-cyan-900/50 dark:bg-cyan-950/20">
                <p class="text-xs font-medium text-cyan-700 dark:text-cyan-300">{{ t('affiliate.campaign.copyPitch') }}</p>
                <p class="mt-1 break-words text-sm leading-6 text-gray-700 dark:text-gray-200">
                  {{ promotionPitch }}
                </p>
                <button class="btn btn-secondary btn-sm mt-3 w-full justify-center" @click="copyPromotionPitch">
                  <Icon name="clipboard" size="sm" />
                  <span>{{ t('affiliate.campaign.copyPitch') }}</span>
                </button>
              </div>
            </div>
          </section>

          <section
            data-testid="affiliate-progress-center"
            class="card affiliate-panel affiliate-progress-panel p-6"
          >
            <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.campaign.progressTitle') }}</h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.campaign.progressDescription') }}</p>
              </div>
              <div class="rounded-md border border-cyan-200 bg-cyan-50 px-3 py-2 text-sm font-semibold text-cyan-700 dark:border-cyan-900/60 dark:bg-cyan-950/30 dark:text-cyan-300">
                {{ t('affiliate.campaign.qualifiedRatio') }} {{ qualifiedRatio }}
              </div>
            </div>

            <div class="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div
                data-stat="rate"
                :data-featured="featuredStat === 'rate' ? 'true' : undefined"
                class="affiliate-stat p-5"
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
                class="affiliate-stat p-5"
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
                class="affiliate-stat p-5"
              >
                <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
                <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
                  {{ formatCurrency(detail.aff_quota) }}
                </p>
              </div>
              <div
                data-stat="history"
                :data-featured="featuredStat === 'history' ? 'true' : undefined"
                class="affiliate-stat p-5"
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
          </section>
        </div>

        <AffiliateTierIdentity
          :detail="detail"
          :next-tier="nextTier"
          :progress="tierProgress"
          :formatted-rate="formattedRebateRate"
        />

        <section
          v-if="rewardTasks.length > 0"
          data-testid="affiliate-milestone-rewards"
          class="card affiliate-panel affiliate-rewards p-6"
        >
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.rewards.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.rewards.description') }}</p>
            </div>
            <div class="rounded-md border border-cyan-200 bg-cyan-50 px-3 py-2 text-sm font-semibold text-cyan-700 dark:border-cyan-900/60 dark:bg-cyan-950/30 dark:text-cyan-300">
              {{ t('affiliate.tiers.qualifiedCount') }} {{ formatCount(detail.qualified_invitee_count) }}
            </div>
          </div>

          <div class="mt-5 grid gap-3 lg:grid-cols-3">
            <article
              v-for="reward in rewardTasks"
              :key="reward.id"
              data-testid="affiliate-reward-task"
              class="affiliate-reward-card min-w-0"
              :data-state="reward.claimed ? 'claimed' : reward.claimable ? 'claimable' : 'locked'"
            >
              <div class="flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="break-words text-sm font-semibold text-gray-900 dark:text-white">{{ reward.name }}</p>
                  <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-400">
                    {{ reward.description || rewardRequirementText(reward) }}
                  </p>
                </div>
                <span class="affiliate-reward-icon" :data-ready="reward.claimable || reward.claimed ? 'true' : undefined">
                  <Icon :name="reward.claimed ? 'checkCircle' : reward.claimable ? 'gift' : 'lock'" size="sm" />
                </span>
              </div>

              <div class="mt-4 flex flex-wrap items-center gap-2">
                <span class="rounded-md bg-cyan-50 px-2.5 py-1 text-xs font-medium text-cyan-700 dark:bg-cyan-950/40 dark:text-cyan-300">
                  {{ rewardRequirementText(reward) }}
                </span>
                <span class="rounded-md bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 dark:bg-blue-950/40 dark:text-blue-300">
                  {{ rewardBenefitText(reward) }}
                </span>
              </div>

              <div class="mt-4">
                <button
                  v-if="reward.claimable && !reward.claimed"
                  class="btn btn-primary btn-sm w-full justify-center"
                  :disabled="claimingRewardId === reward.id"
                  @click="claimReward(reward.id)"
                >
                  <Icon v-if="claimingRewardId === reward.id" name="refresh" size="sm" class="animate-spin" />
                  <Icon v-else name="gift" size="sm" />
                  <span>{{ claimingRewardId === reward.id ? t('affiliate.rewards.claiming') : t('affiliate.rewards.claim') }}</span>
                </button>

                <div v-else-if="reward.claimed" class="affiliate-copy-row">
                  <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ reward.code || '-' }}</code>
                  <button class="btn btn-secondary btn-sm" :disabled="!reward.code" @click="copyRewardCode(reward.code)">
                    <Icon name="copy" size="sm" />
                    <span>{{ t('affiliate.rewards.copyCode') }}</span>
                  </button>
                </div>

                <p v-else class="rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-center text-xs font-medium text-gray-500 dark:border-dark-700 dark:bg-dark-900/70 dark:text-dark-400">
                  {{ t('affiliate.rewards.remaining', { count: formatCount(reward.remaining_invitees) }) }}
                </p>
              </div>
            </article>
          </div>
        </section>

        <div class="card affiliate-panel p-6">
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

        <div class="card affiliate-panel p-6">
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

        <div
          data-testid="affiliate-mobile-cta"
          class="sticky bottom-3 z-20 md:hidden"
        >
          <button
            class="flex w-full items-center justify-center gap-2 rounded-lg border border-cyan-300 bg-cyan-600 px-4 py-3 text-sm font-semibold text-white shadow-lg shadow-cyan-900/20 dark:border-cyan-500/70 dark:bg-cyan-500 dark:text-cyan-950"
            @click="copyInviteLink"
          >
            <Icon name="link" size="sm" />
            <span>{{ t('affiliate.campaign.mobileCta') }}</span>
          </button>
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
import type { AffiliateRewardProgress, UserAffiliateDetail } from '@/types'
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
const claimingRewardId = ref<number | null>(null)
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
const currentTierLabel = computed(() => t(getAffiliateTierPresentation(detail.value?.automatic_level).labelKey))
const nextTierCallout = computed(() => {
  if (!nextTier.value) return t('affiliate.campaign.maxTier')
  return t('affiliate.campaign.nextTier', {
    count: formatCount(detail.value?.remaining_qualified_invitees ?? 0),
    level: t(getAffiliateTierPresentation(nextTier.value.level).labelKey)
  })
})
const qualifiedRatio = computed(() => {
  const invited = normalizeCount(detail.value?.aff_count ?? 0)
  if (invited <= 0) return '0%'
  const qualified = normalizeCount(detail.value?.qualified_invitee_count ?? 0)
  return `${Math.min(100, Math.round((qualified / invited) * 100))}%`
})
const rewardSteps = computed(() => [
  {
    icon: 'userPlus' as const,
    title: t('affiliate.campaign.stepRegisterTitle'),
    description: t('affiliate.campaign.stepRegisterDescription')
  },
  {
    icon: 'creditCard' as const,
    title: t('affiliate.campaign.stepRechargeTitle', {
      amount: formatCurrency(detail.value?.qualification_amount ?? 0)
    }),
    description: t('affiliate.campaign.stepRechargeDescription')
  },
  {
    icon: 'gift' as const,
    title: t('affiliate.campaign.stepRewardTitle'),
    description: t('affiliate.campaign.stepRewardDescription')
  }
])
const promotionPitch = computed(() => t('affiliate.campaign.pitchTemplate', {
  link: inviteLink.value,
  amount: formatCurrency(detail.value?.qualification_amount ?? 0)
}))
const rewardTasks = computed(() => detail.value?.rewards ?? [])

function formatCount(value: number): string {
  return normalizeCount(value).toLocaleString()
}

function normalizeCount(value: number): number {
  return Number.isFinite(value) ? Math.floor(Math.max(0, value)) : 0
}

function rewardRequirementText(reward: AffiliateRewardProgress): string {
  return t('affiliate.rewards.requirement', {
    count: formatCount(reward.required_qualified_invitees)
  })
}

function rewardBenefitText(reward: AffiliateRewardProgress): string {
  if (reward.reward_type === 'subscription') {
    return t('affiliate.rewards.subscriptionBenefit', {
      group: rewardGroupLabel(reward),
      days: formatCount(reward.validity_days)
    })
  }
  return t('affiliate.rewards.balanceBenefit', {
    amount: formatCurrency(reward.balance_value)
  })
}

function rewardGroupLabel(reward: AffiliateRewardProgress): string {
  const name = reward.group_name?.trim()
  if (name) return name
  if (reward.group_id) return t('affiliate.rewards.groupFallback', { id: reward.group_id })
  return t('affiliate.rewards.subscriptionGeneric')
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

async function copyPromotionPitch(): Promise<void> {
  if (!promotionPitch.value) return
  await copyToClipboard(promotionPitch.value, t('affiliate.campaign.pitchCopied'))
}

async function copyRewardCode(code?: string): Promise<void> {
  if (!code) return
  await copyToClipboard(code, t('affiliate.rewards.codeCopied'))
}

async function claimReward(ruleId: number): Promise<void> {
  if (claimingRewardId.value !== null) return
  claimingRewardId.value = ruleId
  try {
    const result = await userAPI.claimAffiliateReward(ruleId)
    appStore.showSuccess(t('affiliate.rewards.claimSuccess', { code: result.code }))
    await loadAffiliateDetail(true)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.rewards.claimFailed')))
  } finally {
    claimingRewardId.value = null
  }
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
.affiliate-console {
  isolation: isolate;
}

.affiliate-console__grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(120deg, rgb(6 182 212 / 0.12), transparent 42%),
    linear-gradient(rgb(14 165 233 / 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgb(14 165 233 / 0.07) 1px, transparent 1px);
  background-size: auto, 32px 32px, 32px 32px;
  mask-image: linear-gradient(90deg, black, transparent 86%);
}

.affiliate-step {
  box-shadow:
    inset 0 0 0 1px rgb(255 255 255 / 0.72),
    0 10px 24px -22px rgb(8 145 178 / 0.55);
}

.affiliate-tools,
.affiliate-progress-panel,
.affiliate-rewards,
.affiliate-panel {
  overflow: hidden;
  border-color: rgb(165 243 252 / 0.65);
}

.affiliate-tools {
  background:
    radial-gradient(circle at top right, rgb(34 211 238 / 0.14), transparent 34%),
    rgb(255 255 255);
}

.affiliate-progress-panel {
  background:
    linear-gradient(135deg, rgb(240 249 255 / 0.92), rgb(255 255 255)),
    rgb(255 255 255);
}

.affiliate-copy-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: rgb(249 250 251);
  padding: 0.5rem 0.75rem;
}

.affiliate-stat {
  min-width: 0;
  border: 1px solid rgb(219 234 254 / 0.86);
  border-radius: 8px;
  background: rgb(255 255 255 / 0.88);
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.66);
}

.affiliate-stat--featured {
  border-color: rgb(34 211 238 / 0.6);
  background: linear-gradient(135deg, rgb(239 246 255 / 0.75), rgb(236 254 255 / 0.55));
  box-shadow: inset 0 0 0 1px rgb(14 165 233 / 0.08), 0 5px 16px rgb(14 165 233 / 0.08);
}

.affiliate-reward-card {
  border: 1px solid rgb(207 250 254 / 0.9);
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgb(240 253 250 / 0.76), rgb(255 255 255)),
    rgb(255 255 255);
  padding: 1rem;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.72);
}

.affiliate-reward-card[data-state="claimable"] {
  border-color: rgb(34 211 238 / 0.72);
  box-shadow: inset 0 0 0 1px rgb(14 165 233 / 0.1), 0 10px 24px -22px rgb(8 145 178 / 0.75);
}

.affiliate-reward-icon {
  display: grid;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  place-items: center;
  border-radius: 8px;
  background: rgb(241 245 249);
  color: rgb(100 116 139);
}

.affiliate-reward-icon[data-ready="true"] {
  background: rgb(207 250 254);
  color: rgb(8 145 178);
}

:global(.dark .affiliate-tools),
:global(.dark .affiliate-progress-panel),
:global(.dark .affiliate-rewards),
:global(.dark .affiliate-panel) {
  border-color: rgb(34 211 238 / 0.28) !important;
  color: rgb(226 232 240) !important;
  box-shadow:
    inset 0 0 0 1px rgb(255 255 255 / 0.035),
    0 16px 34px rgb(0 0 0 / 0.22) !important;
}

:global(.dark .affiliate-panel) {
  background:
    linear-gradient(145deg, rgb(15 23 42 / 0.96), rgb(2 6 23 / 0.88)),
    rgb(15 23 42) !important;
}

:global(.dark .affiliate-console) {
  background:
    linear-gradient(135deg, rgb(8 47 73 / 0.46), transparent 44%),
    rgb(15 23 42 / 0.94);
  border-color: rgb(34 211 238 / 0.28);
  box-shadow:
    inset 0 0 0 1px rgb(255 255 255 / 0.035),
    0 18px 38px rgb(0 0 0 / 0.24);
}

:global(.dark .affiliate-console__grid) {
  background:
    linear-gradient(120deg, rgb(34 211 238 / 0.12), transparent 44%),
    linear-gradient(rgb(103 232 249 / 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgb(103 232 249 / 0.055) 1px, transparent 1px);
  opacity: 0.82;
}

:global(.dark .affiliate-step) {
  background:
    linear-gradient(145deg, rgb(15 23 42 / 0.82), rgb(8 47 73 / 0.48)),
    rgb(15 23 42);
  border-color: rgb(34 211 238 / 0.22);
  box-shadow:
    inset 0 0 0 1px rgb(255 255 255 / 0.035),
    0 10px 24px -22px rgb(6 182 212 / 0.72);
}

:global(.dark .affiliate-tools) {
  background:
    radial-gradient(circle at top right, rgb(34 211 238 / 0.16), transparent 38%),
    linear-gradient(180deg, rgb(15 23 42 / 0.96), rgb(2 6 23 / 0.86)),
    rgb(15 23 42) !important;
}

:global(.dark .affiliate-progress-panel) {
  background:
    linear-gradient(135deg, rgb(8 47 73 / 0.42), rgb(15 23 42 / 0.94)),
    rgb(15 23 42) !important;
}

:global(.dark .affiliate-rewards) {
  background:
    linear-gradient(135deg, rgb(15 23 42 / 0.96), rgb(30 41 59 / 0.72)),
    rgb(15 23 42) !important;
}

:global(.dark .affiliate-pitch-card) {
  border-color: rgb(34 211 238 / 0.3) !important;
  background:
    linear-gradient(135deg, rgb(8 47 73 / 0.5), rgb(15 23 42 / 0.86)),
    rgb(15 23 42) !important;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.035);
}

:global(.dark .affiliate-copy-row) {
  border-color: rgb(51 65 85 / 0.95) !important;
  background: rgb(2 6 23 / 0.48) !important;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.025);
}

:global(.dark .affiliate-stat) {
  border-color: rgb(34 211 238 / 0.2) !important;
  background:
    linear-gradient(180deg, rgb(15 23 42 / 0.88), rgb(2 6 23 / 0.48)),
    rgb(15 23 42) !important;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.03);
}

:global(.dark .affiliate-stat--featured) {
  border-color: rgb(34 211 238 / 0.58) !important;
  background:
    linear-gradient(135deg, rgb(8 47 73 / 0.78), rgb(15 23 42 / 0.84)),
    rgb(15 23 42) !important;
  box-shadow:
    inset 0 0 0 1px rgb(34 211 238 / 0.08),
    0 8px 22px rgb(6 182 212 / 0.08);
}

:global(.dark .affiliate-reward-card) {
  border-color: rgb(34 211 238 / 0.24) !important;
  background:
    linear-gradient(135deg, rgb(8 47 73 / 0.42), rgb(15 23 42 / 0.9)),
    rgb(15 23 42) !important;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.03);
}

:global(.dark .affiliate-reward-card[data-state="claimable"]) {
  border-color: rgb(34 211 238 / 0.7);
  box-shadow: inset 0 0 0 1px rgb(34 211 238 / 0.1), 0 10px 24px -22px rgb(6 182 212 / 0.8);
}

:global(.dark .affiliate-reward-icon) {
  background: rgb(30 41 59);
  color: rgb(148 163 184);
}

:global(.dark .affiliate-reward-icon[data-ready="true"]) {
  background: rgb(8 145 178 / 0.28);
  color: rgb(103 232 249);
}

@media (max-width: 640px) {
  .affiliate-copy-row {
    align-items: stretch;
    flex-direction: column;
  }

  .affiliate-copy-row > button {
    justify-content: center;
    width: 100%;
  }
}
</style>
