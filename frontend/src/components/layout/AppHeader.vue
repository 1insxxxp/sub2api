<template>
  <header class="app-header-shell theme-crisp sticky top-0 z-30">
    <div class="app-header-toolbar">
      <!-- Left: Mobile Menu Toggle + Page Title -->
      <div class="app-header-title-group">
        <button
          @click="toggleMobileSidebar"
          class="btn-ghost btn-icon lg:hidden"
          aria-label="Toggle Menu"
        >
          <Icon name="menu" size="md" />
        </button>

        <div class="hidden min-w-0 lg:block">
          <h1 class="truncate text-base font-bold text-slate-950 dark:text-white">
            {{ pageTitle }}
          </h1>
          <p v-if="pageDescription" class="mt-0.5 max-w-xl truncate text-xs text-slate-500 dark:text-dark-400">
            {{ pageDescription }}
          </p>
        </div>
      </div>

      <!-- Right: Announcements + Docs + Language + Subscriptions + Balance + User Dropdown -->
      <div class="app-header-actions">
        <!-- Announcement Bell -->
        <AnnouncementBell v-if="user" />

        <!-- Docs Link -->
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-semibold text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-950 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white sm:flex"
        >
          <Icon name="book" size="sm" />
          <span class="hidden sm:inline">{{ t('nav.docs') }}</span>
        </a>

        <!-- Language Switcher -->
        <LocaleSwitcher />

        <!-- Subscription Progress (for users with active subscriptions) -->
        <SubscriptionProgressMini v-if="user" />

        <!-- Balance Display -->
        <div
          v-if="user"
          data-test="header-balance-pill"
          class="app-header-balance-pill hidden sm:flex"
        >
          <svg
            class="h-4 w-4 text-blue-600 dark:text-blue-300"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"
            />
          </svg>
          <span class="text-sm font-semibold text-blue-700 dark:text-blue-200">
            ${{ user.balance?.toFixed(2) || '0.00' }}
          </span>
        </div>

        <!-- Daily Check-in -->
        <div
          v-if="showCheckinButton"
          class="group/checkin relative hidden sm:inline-flex"
        >
          <button
            type="button"
            data-test="daily-checkin-button"
            :disabled="checkinButtonDisabled"
            :title="checkinButtonTitle"
            class="inline-flex items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-3 py-1.5 text-sm font-semibold text-amber-700 shadow-sm shadow-amber-900/5 transition-colors hover:bg-amber-100 disabled:cursor-not-allowed disabled:border-slate-200 disabled:bg-slate-100 disabled:text-slate-400 dark:border-amber-500/30 dark:bg-amber-900/20 dark:text-amber-200 dark:hover:bg-amber-900/30 dark:disabled:border-dark-700 dark:disabled:bg-dark-800 dark:disabled:text-dark-400"
            @click="handleCheckin"
          >
            <Icon
              :name="checkinStatus?.checked_in ? 'check' : 'gift'"
              size="sm"
              :class="checkinSubmitting ? 'animate-pulse' : ''"
            />
            <span>{{ checkinButtonLabel }}</span>
          </button>

          <div
            class="brand-floating-panel pointer-events-none absolute right-0 top-full z-50 mt-3 w-[25rem] max-w-[calc(100vw-2rem)] translate-y-1 overflow-hidden text-left opacity-0 transition-all duration-150 group-hover/checkin:pointer-events-auto group-hover/checkin:translate-y-0 group-hover/checkin:opacity-100 group-focus-within/checkin:pointer-events-auto group-focus-within/checkin:translate-y-0 group-focus-within/checkin:opacity-100"
            data-test="daily-checkin-popover"
          >
            <div class="brand-floating-header px-6 py-5">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="mb-3 flex items-center gap-3">
                    <div class="brand-floating-icon h-10 w-10 rounded-2xl">
                      <Icon :name="checkinStatus?.checked_in ? 'check' : 'gift'" size="sm" />
                    </div>
                    <div class="min-w-0">
                      <h3 class="text-sm font-semibold text-slate-950 dark:text-white">
                        {{ t('checkin.cardTitle') }}
                      </h3>
                      <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                        {{ checkinStatus?.checked_in ? t('checkin.cardCheckedHint') : t('checkin.cardHint') }}
                      </p>
                    </div>
                  </div>
                </div>
                <span
                  v-if="checkinStatus?.checked_in && displayRewardAmount > 0"
                  class="brand-floating-chip shrink-0 bg-emerald-50/80 text-emerald-700 dark:bg-emerald-500/14 dark:text-emerald-200"
                >
                  +{{ formatUsd(displayRewardAmount) }}
                </span>
              </div>
            </div>

            <div class="p-4">
              <div class="grid grid-cols-2 gap-3">
                <div class="brand-floating-card">
                  <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('checkin.currentStreak') }}</p>
                  <p class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">
                    {{ t('checkin.days', { count: checkinStatus?.current_streak || 0 }) }}
                  </p>
                </div>
                <div class="brand-floating-card">
                  <p class="text-xs text-slate-500 dark:text-slate-400">{{ t('checkin.lifetimeDays') }}</p>
                  <p class="mt-1 text-lg font-semibold text-slate-950 dark:text-white">
                    {{ t('checkin.days', { count: checkinStatus?.lifetime_checkin_days || 0 }) }}
                  </p>
                </div>
              </div>

              <div class="brand-floating-card mt-3">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">
                      {{ t('checkin.eligibilityTitle') }}
                    </p>
                    <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                      {{ eligibilityMessage }}
                    </p>
                  </div>
                  <span
                    class="brand-floating-chip shrink-0 px-2.5 py-1 text-[11px]"
                    :class="checkinStatus?.eligible === false
                      ? 'bg-amber-50/85 text-amber-700 dark:bg-amber-500/14 dark:text-amber-200'
                      : 'bg-emerald-50/85 text-emerald-700 dark:bg-emerald-500/14 dark:text-emerald-200'"
                  >
                    {{ checkinStatus?.eligible === false ? t('checkin.eligibilityPendingBadge') : t('checkin.eligibilityReadyBadge') }}
                  </span>
                </div>
                <div
                  v-if="checkinMinSpend > 0"
                  class="checkin-progress-track mt-3"
                  :aria-label="t('checkin.eligibilityTitle')"
                >
                  <div
                    class="checkin-progress-fill"
                    data-test="daily-checkin-eligibility-progress"
                    :style="{ width: `${eligibilityProgressPercent}%` }"
                  >
                    <span class="checkin-progress-sheen" aria-hidden="true" />
                  </div>
                </div>
              </div>

              <div class="mt-4">
                <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">
                  {{ t('checkin.rewardBreakdown') }}
                </p>
              </div>

              <div class="mt-2 grid grid-cols-2 gap-2">
                <div class="brand-floating-card border-emerald-100/80 bg-emerald-50/72 dark:border-emerald-400/14 dark:bg-emerald-500/10">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-200">
                    {{ t('checkin.baseReward') }}
                  </p>
                  <p class="mt-1 text-sm font-bold text-emerald-800 dark:text-emerald-100">
                    {{ baseRewardLabel }}
                  </p>
                </div>
                <div class="brand-floating-card border-amber-100/80 bg-amber-50/72 dark:border-amber-400/14 dark:bg-amber-500/10">
                  <p class="text-[11px] font-semibold uppercase tracking-wide text-amber-700 dark:text-amber-200">
                    {{ t('checkin.streakBonus') }}
                  </p>
                  <p class="mt-1 text-sm font-bold text-amber-800 dark:text-amber-100">
                    {{ streakBonusLabel }}
                  </p>
                </div>
              </div>

              <div class="brand-floating-card mt-3 border-blue-100/80 bg-[linear-gradient(135deg,rgba(239,246,255,0.96),rgba(250,245,255,0.9))] px-3 py-2 text-xs text-blue-700 dark:border-blue-400/14 dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.18),rgba(124,58,237,0.12))] dark:text-blue-200">
                <p class="font-semibold">{{ t('checkin.streakBonusTitle') }}</p>
                <p class="mt-1">
                  {{ nextStreakBonusMessage }}
                </p>
              </div>

              <div v-if="recentCheckinRecords.length > 0" class="mt-4 space-y-2">
                <p class="text-xs font-medium text-slate-500 dark:text-slate-400">
                  {{ t('checkin.recentRecords') }}
                </p>
                <div
                  v-for="record in recentCheckinRecords"
                  :key="record.id"
                  class="brand-floating-card flex items-center justify-between px-3 py-2 text-xs"
                >
                  <div>
                    <p class="font-semibold text-slate-800 dark:text-slate-100">{{ record.checkin_date }}</p>
                    <p class="text-slate-500 dark:text-slate-400">
                      {{ t('checkin.streakDay', { day: record.streak_day || 1 }) }}
                    </p>
                  </div>
                  <span class="font-semibold text-emerald-600 dark:text-emerald-300">
                    +{{ formatUsd(record.reward_amount || record.total_reward_amount || 0) }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- User Dropdown -->
        <div v-if="user" class="relative shrink-0" ref="dropdownRef">
          <button
            @click="toggleDropdown"
            class="group flex items-center gap-2 rounded-xl border border-transparent p-1.5 transition-all duration-200 hover:border-blue-200/70 hover:bg-blue-50/80 hover:shadow-sm hover:shadow-blue-600/10 focus:outline-none focus:ring-2 focus:ring-blue-500/25 dark:hover:border-blue-400/20 dark:hover:bg-blue-500/10"
            aria-label="User Menu"
            aria-haspopup="menu"
            :aria-expanded="dropdownOpen"
          >
            <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-xl bg-[linear-gradient(135deg,#2563eb,#3b82f6,#06b6d4)] text-sm font-semibold text-white shadow-sm shadow-blue-600/25 ring-1 ring-blue-300/40">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="displayName"
                class="h-full w-full object-cover"
              >
              <span v-else>{{ userInitials }}</span>
            </div>
            <div class="hidden text-left md:block">
              <div class="max-w-28 truncate text-sm font-semibold text-slate-950 dark:text-white">
                {{ displayName }}
              </div>
              <div class="text-xs capitalize text-gray-500 dark:text-dark-400">
                {{ user.role }}
              </div>
            </div>
            <Icon
              name="chevronDown"
              size="sm"
              class="hidden text-slate-400 transition-transform duration-200 group-hover:text-blue-500 md:block"
              :class="{ 'rotate-180 text-blue-500': dropdownOpen }"
            />
          </button>

          <!-- Dropdown Menu -->
          <transition name="dropdown">
            <div
              v-if="dropdownOpen"
              class="dropdown profile-menu right-0 mt-2 w-64 max-w-[calc(100vw-1rem)]"
              role="menu"
            >
              <!-- User Info -->
              <div class="profile-menu-identity">
                <div class="profile-menu-avatar flex h-9 w-9 items-center justify-center overflow-hidden rounded-xl bg-[linear-gradient(135deg,#2563eb,#3b82f6,#06b6d4)] text-sm font-bold text-white shadow-sm shadow-blue-600/20 ring-1 ring-white/70 dark:ring-white/10">
                  <img
                    v-if="avatarUrl"
                    :src="avatarUrl"
                    :alt="displayName"
                    class="h-full w-full object-cover"
                  >
                  <span v-else>{{ userInitials }}</span>
                </div>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-semibold text-slate-950 dark:text-white">
                    {{ displayName }}
                  </div>
                  <div class="mt-0.5 truncate text-xs text-slate-500 dark:text-slate-400">{{ user.email }}</div>
                </div>
              </div>

              <!-- Balance (mobile only) -->
              <div class="profile-menu-section sm:hidden">
                <div class="rounded-lg border border-blue-100/80 bg-blue-50/70 px-2.5 py-2 dark:border-blue-400/15 dark:bg-blue-500/10">
                  <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
                    {{ t('common.balance') }}
                  </div>
                  <div class="mt-0.5 text-base font-semibold text-blue-700 dark:text-blue-200">
                    ${{ user.balance?.toFixed(2) || '0.00' }}
                  </div>
                </div>
              </div>

              <div class="profile-menu-section">
                <router-link to="/profile" @click="closeDropdown" class="profile-menu-item" role="menuitem">
                  <span class="profile-menu-icon">
                    <Icon name="user" size="sm" />
                  </span>
                  <span>{{ t('nav.profile') }}</span>
                </router-link>

                <router-link to="/keys" @click="closeDropdown" class="profile-menu-item" role="menuitem">
                  <span class="profile-menu-icon">
                    <Icon name="key" size="sm" />
                  </span>
                  <span>{{ t('nav.apiKeys') }}</span>
                </router-link>

                <a
                  v-if="authStore.isAdmin"
                  href="https://github.com/Wei-Shaw/sub2api"
                  target="_blank"
                  rel="noopener noreferrer"
                  @click="closeDropdown"
                  class="profile-menu-item"
                  role="menuitem"
                >
                  <span class="profile-menu-icon">
                    <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                      <path
                        fill-rule="evenodd"
                        clip-rule="evenodd"
                        d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.17 6.839 9.49.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.464-1.11-1.464-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.336-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.167 22 16.418 22 12c0-5.523-4.477-10-10-10z"
                      />
                    </svg>
                  </span>
                  <span>{{ t('nav.github') }}</span>
                </a>

              </div>

              <!-- Contact Support (only show if configured) -->
              <div
                v-if="contactInfo"
                class="profile-menu-section"
              >
                <div class="profile-menu-contact-card">
                  <div class="profile-menu-contact-row">
                    <svg
                      class="h-4 w-4 flex-shrink-0 text-blue-500 dark:text-blue-300"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      stroke-width="1.5"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155"
                      />
                    </svg>
                    <span class="shrink-0 font-medium">{{ t('common.contactSupport') }}</span>
                    <span class="profile-menu-contact-value">{{ contactInfo }}</span>
                  </div>
                </div>
              </div>

              <div v-if="showOnboardingButton" class="profile-menu-section">
                <button @click="handleReplayGuide" class="profile-menu-item w-full" role="menuitem">
                  <span class="profile-menu-icon">
                    <Icon name="questionCircle" size="sm" />
                  </span>
                  <span>{{ $t('onboarding.restartTour') }}</span>
                </button>
              </div>

              <div class="profile-menu-section">
                <button
                  @click="handleLogout"
                  class="profile-menu-item profile-menu-item-danger w-full"
                  role="menuitem"
                >
                  <span class="profile-menu-icon profile-menu-icon-danger">
                    <Icon name="login" size="sm" />
                  </span>
                  <span>{{ t('nav.logout') }}</span>
                </button>
              </div>
            </div>
          </transition>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { getCheckinStatus, submitCheckin, type CheckinStatus } from '@/api/checkin'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorMessage } from '@/utils/apiError'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const onboardingStore = useOnboardingStore()

const user = computed(() => authStore.user)
const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const contactInfo = computed(() => appStore.contactInfo)
const docUrl = computed(() => appStore.docUrl)
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const checkinStatus = ref<CheckinStatus | null>(null)
const checkinLoading = ref(false)
const checkinSubmitting = ref(false)
let checkinStatusRequest = 0

const showCheckinButton = computed(() => {
  return Boolean(
    user.value &&
    checkinStatus.value &&
    checkinStatus.value.enabled &&
    !checkinStatus.value.blacklisted
  )
})

const checkinButtonDisabled = computed(() => {
  return (
    checkinLoading.value ||
    checkinSubmitting.value ||
    checkinStatus.value?.checked_in === true ||
    checkinStatus.value?.eligible === false
  )
})

const checkinButtonLabel = computed(() => {
  if (checkinSubmitting.value || checkinLoading.value) return t('checkin.loading')
  if (checkinStatus.value?.checked_in) return t('checkin.checked')
  if (checkinStatus.value?.eligible === false) return t('checkin.unavailable')
  return t('checkin.action')
})

const checkinButtonTitle = computed(() => {
  const status = checkinStatus.value
  if (!status) return checkinButtonLabel.value
  if (status.eligible === false && status.ineligible_reason === 'insufficient_spend') {
    return t('checkin.insufficientSpend', {
      min: formatUsd(status.min_total_usage_usd),
      current: formatUsd(status.total_usage_usd),
    })
  }
  return checkinButtonLabel.value
})

const displayRewardAmount = computed(() => {
  const status = checkinStatus.value
  return Number(status?.total_reward_amount ?? status?.reward_amount ?? 0)
})

const checkinMinSpend = computed(() => {
  return Math.max(0, Number(checkinStatus.value?.min_total_usage_usd ?? 0))
})

const checkinCurrentSpend = computed(() => {
  return Math.max(0, Number(checkinStatus.value?.total_usage_usd ?? 0))
})

const eligibilityProgressPercent = computed(() => {
  if (checkinMinSpend.value <= 0) return 100
  return Math.min(100, Math.max(0, (checkinCurrentSpend.value / checkinMinSpend.value) * 100))
})

const eligibilityMessage = computed(() => {
  const min = formatUsd(checkinMinSpend.value)
  const current = formatUsd(checkinCurrentSpend.value)
  if (checkinMinSpend.value <= 0) {
    return t('checkin.eligibilityNoThreshold')
  }
  if (checkinStatus.value?.eligible === false) {
    return t('checkin.eligibilityPending', { min, current })
  }
  return t('checkin.eligibilitySatisfied', { min, current })
})

const baseRewardLabel = computed(() => {
  const status = checkinStatus.value
  const amount = Number(status?.base_reward_amount ?? status?.reward_amount ?? 0)
  if (!status?.checked_in || amount <= 0) {
    return t('checkin.randomReward')
  }
  return formatUsd(amount)
})

const streakBonusLabel = computed(() => {
  const status = checkinStatus.value
  const amount = Number(status?.bonus_reward_amount ?? 0)
  if (!status?.checked_in) {
    return t('checkin.streakBonusWhenReached')
  }
  if (amount <= 0) {
    return t('checkin.noStreakBonusToday')
  }
  return formatUsd(amount)
})

const nextStreakBonusMessage = computed(() => {
  const rule = checkinStatus.value?.next_streak_rule
  if (!rule) {
    return t('checkin.noUpcomingStreakBonus')
  }
  return t('checkin.nextStreakBonus', {
    day: rule.day,
    amount: formatUsd(rule.bonus_amount),
  })
})

const recentCheckinRecords = computed(() => {
  return checkinStatus.value?.recent_records?.slice(0, 3) ?? []
})

// Only show the onboarding replay button for admins in standard mode.
const showOnboardingButton = computed(() => {
  return !authStore.isSimpleMode && user.value?.role === 'admin'
})

const userInitials = computed(() => {
  if (!user.value) return ''
  // Prefer username, fallback to email
  if (user.value.username) {
    return user.value.username.substring(0, 2).toUpperCase()
  }
  if (user.value.email) {
    // Get the part before @ and take first 2 chars
    const localPart = user.value.email.split('@')[0]
    return localPart.substring(0, 2).toUpperCase()
  }
  return ''
})

const displayName = computed(() => {
  if (!user.value) return ''
  return user.value.username || user.value.email?.split('@')[0] || ''
})

const pageTitle = computed(() => {
  // For custom pages, use the menu item's label instead of generic "鑷畾涔夐〉闈?
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const titleKey = route.meta.titleKey as string
  if (titleKey) {
    return t(titleKey)
  }
  return (route.meta.title as string) || ''
})

const pageDescription = computed(() => {
  const descKey = route.meta.descriptionKey as string
  if (descKey) {
    return t(descKey)
  }
  return (route.meta.description as string) || ''
})

function toggleMobileSidebar() {
  appStore.toggleMobileSidebar()
}

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown() {
  dropdownOpen.value = false
}

async function handleLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    // Ignore logout errors - still redirect to login
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleReplayGuide() {
  closeDropdown()
  onboardingStore.replay()
}

async function loadCheckinStatus() {
  if (!user.value) {
    checkinStatus.value = null
    return
  }

  const requestId = ++checkinStatusRequest
  checkinLoading.value = true
  try {
    const status = await getCheckinStatus()
    if (requestId === checkinStatusRequest) {
      checkinStatus.value = status
    }
  } catch (error) {
    if (requestId === checkinStatusRequest) {
      checkinStatus.value = null
    }
    console.error('Failed to load check-in status:', error)
  } finally {
    if (requestId === checkinStatusRequest) {
      checkinLoading.value = false
    }
  }
}

async function handleCheckin() {
  if (!user.value || checkinButtonDisabled.value) return

  checkinSubmitting.value = true
  try {
    const result = await submitCheckin()
    checkinStatus.value = result
    if (authStore.user && Number.isFinite(result.balance_after)) {
      authStore.user = {
        ...authStore.user,
        balance: result.balance_after
      }
    }
    appStore.showSuccess(t('checkin.success', { amount: (result.reward_amount ?? 0).toFixed(2) }))
    await authStore.refreshUser()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('checkin.failed')))
  } finally {
    checkinSubmitting.value = false
  }
}

function formatUsd(value: number | null | undefined): string {
  const amount = Number.isFinite(value) ? Number(value) : 0
  return `$${amount.toFixed(2)}`
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})

watch(
  () => user.value?.id,
  (id) => {
    checkinStatus.value = null
    checkinStatusRequest += 1
    if (id) {
      void loadCheckinStatus()
    }
  },
  { immediate: true }
)
</script>

<style scoped>
.profile-menu {
  overflow: hidden;
  border-radius: 12px;
  padding: 0;
  border-color: rgba(226, 232, 240, 0.96);
  background: rgba(255, 255, 255, 0.97);
  box-shadow:
    0 10px 28px rgba(15, 23, 42, 0.12),
    0 1px 0 rgba(255, 255, 255, 0.86) inset;
  backdrop-filter: blur(12px);
}

:global(html.dark .profile-menu) {
  border-color: rgba(255, 255, 255, 0.1);
  background: rgba(2, 6, 23, 0.96);
  box-shadow:
    0 14px 34px rgba(0, 0, 0, 0.34),
    0 1px 0 rgba(255, 255, 255, 0.06) inset;
}

.profile-menu-identity {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.82);
  background: rgba(248, 250, 252, 0.72);
}

:global(html.dark .profile-menu-identity) {
  border-bottom-color: rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.72);
}

.profile-menu-section {
  padding: 6px;
  border-top: 1px solid rgba(226, 232, 240, 0.82);
}

.profile-menu-identity + .profile-menu-section {
  border-top: 0;
}

:global(html.dark .profile-menu-section) {
  border-top-color: rgba(255, 255, 255, 0.08);
}

.profile-menu-item {
  display: flex;
  min-height: 36px;
  width: 100%;
  align-items: center;
  gap: 9px;
  border-radius: 10px;
  padding: 7px 8px;
  color: rgb(51 65 85);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.25;
  transition:
    background-color 160ms ease,
    color 160ms ease;
}

.profile-menu-item:hover,
.profile-menu-item:focus-visible {
  color: rgb(29 78 216);
  background: rgba(37, 99, 235, 0.08);
  outline: none;
}

:global(html.dark .profile-menu-item) {
  color: rgb(203 213 225);
}

:global(html.dark .profile-menu-item:hover),
:global(html.dark .profile-menu-item:focus-visible) {
  color: rgb(191 219 254);
  background: rgba(59, 130, 246, 0.12);
}

.profile-menu-icon {
  display: inline-flex;
  height: 24px;
  width: 24px;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: rgb(37 99 235);
  background: rgba(37, 99, 235, 0.08);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.8) inset;
}

:global(html.dark .profile-menu-icon) {
  color: rgb(147 197 253);
  background: rgba(59, 130, 246, 0.14);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.06) inset;
}

.profile-menu-contact-card {
  border-radius: 10px;
  border: 1px solid rgba(203, 213, 225, 0.86);
  background: rgba(248, 250, 252, 0.9);
  padding: 8px 9px;
  color: rgb(100 116 139);
  font-size: 12px;
}

:global(html.dark .profile-menu-contact-card) {
  border-color: rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.045);
  color: rgb(148 163 184);
}

.profile-menu-contact-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  line-height: 1.45;
}

.profile-menu-contact-value {
  min-width: 0;
  overflow-wrap: anywhere;
  color: rgb(51 65 85);
  font-weight: 600;
}

:global(html.dark .profile-menu-contact-value) {
  color: rgb(226 232 240);
}

.profile-menu-item-danger {
  color: rgb(220 38 38);
}

.profile-menu-item-danger:hover,
.profile-menu-item-danger:focus-visible {
  color: rgb(185 28 28);
  background: rgba(239, 68, 68, 0.09);
}

:global(html.dark .profile-menu-item-danger) {
  color: rgb(248 113 113);
}

:global(html.dark .profile-menu-item-danger:hover),
:global(html.dark .profile-menu-item-danger:focus-visible) {
  color: rgb(252 165 165);
  background: rgba(239, 68, 68, 0.12);
}

.profile-menu-icon-danger {
  color: rgb(220 38 38);
  background: rgba(239, 68, 68, 0.10);
}

:global(html.dark .profile-menu-icon-danger) {
  color: rgb(248 113 113);
  background: rgba(239, 68, 68, 0.14);
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition:
    opacity 180ms ease,
    transform 180ms ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.98) translateY(-4px);
}

.checkin-progress-track {
  position: relative;
  overflow: hidden;
  height: 0.875rem;
  border-radius: 9999px;
  border: 1px solid rgba(191, 219, 254, 0.72);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(241, 245, 249, 0.94));
  box-shadow:
    inset 0 1px 2px rgba(148, 163, 184, 0.18),
    0 1px 0 rgba(255, 255, 255, 0.9);
}

.checkin-progress-track::after {
  content: '';
  position: absolute;
  inset: 2px;
  border-radius: 9999px;
  background-image:
    linear-gradient(
      90deg,
      rgba(148, 163, 184, 0.08) 0,
      rgba(148, 163, 184, 0.08) 10px,
      transparent 10px,
      transparent 20px
    );
  pointer-events: none;
}

.checkin-progress-fill {
  position: relative;
  height: 100%;
  border-radius: 9999px;
  background:
    linear-gradient(90deg, rgba(37, 99, 235, 0.96), rgba(59, 130, 246, 0.94), rgba(6, 182, 212, 0.92));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.3),
    0 8px 18px rgba(37, 99, 235, 0.22);
  transition:
    width 240ms ease,
    box-shadow 240ms ease;
}

.checkin-progress-sheen {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background:
    linear-gradient(120deg, transparent 0%, rgba(255, 255, 255, 0.16) 38%, rgba(255, 255, 255, 0.52) 50%, rgba(255, 255, 255, 0.16) 62%, transparent 100%);
  background-size: 160% 100%;
  animation: checkin-progress-flow 2.4s linear infinite;
  pointer-events: none;
}

.dark .checkin-progress-track {
  border-color: rgba(96, 165, 250, 0.24);
  background:
    linear-gradient(180deg, rgba(2, 6, 23, 0.92), rgba(15, 23, 42, 0.92));
  box-shadow:
    inset 0 1px 3px rgba(2, 6, 23, 0.6),
    0 1px 0 rgba(255, 255, 255, 0.04);
}

.dark .checkin-progress-track::after {
  background-image:
    linear-gradient(
      90deg,
      rgba(96, 165, 250, 0.1) 0,
      rgba(96, 165, 250, 0.1) 10px,
      transparent 10px,
      transparent 20px
    );
}

.dark .checkin-progress-fill {
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.14),
    0 10px 22px rgba(37, 99, 235, 0.28);
}

@keyframes checkin-progress-flow {
  from {
    background-position: 160% 0;
  }
  to {
    background-position: -60% 0;
  }
}
</style>
