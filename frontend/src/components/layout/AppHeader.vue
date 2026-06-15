<template>
  <header class="glass sticky top-0 z-30 border-b border-gray-200/50 dark:border-dark-700/50">
    <div class="flex h-16 items-center justify-between px-4 md:px-6">
      <!-- Left: Mobile Menu Toggle + Page Title -->
      <div class="flex items-center gap-4">
        <button
          @click="toggleMobileSidebar"
          class="btn-ghost btn-icon lg:hidden"
          aria-label="Toggle Menu"
        >
          <Icon name="menu" size="md" />
        </button>

        <div class="hidden lg:block">
          <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ pageTitle }}
          </h1>
          <p v-if="pageDescription" class="text-xs text-gray-500 dark:text-dark-400">
            {{ pageDescription }}
          </p>
        </div>
      </div>

      <!-- Right: Announcements + Docs + Language + Subscriptions + Balance + User Dropdown -->
      <div class="flex items-center gap-3">
        <!-- Announcement Bell -->
        <AnnouncementBell v-if="user" />

        <!-- Docs Link -->
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
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
          class="hidden items-center gap-2 rounded-xl bg-primary-50 px-3 py-1.5 dark:bg-primary-900/20 sm:flex"
        >
          <svg
            class="h-4 w-4 text-primary-600 dark:text-primary-400"
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
          <span class="text-sm font-semibold text-primary-700 dark:text-primary-300">
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
            class="inline-flex items-center gap-1.5 rounded-xl border border-amber-200 bg-amber-50 px-3 py-1.5 text-sm font-semibold text-amber-700 transition-colors hover:bg-amber-100 disabled:cursor-not-allowed disabled:border-gray-200 disabled:bg-gray-100 disabled:text-gray-400 dark:border-amber-500/30 dark:bg-amber-900/20 dark:text-amber-200 dark:hover:bg-amber-900/30 dark:disabled:border-dark-700 dark:disabled:bg-dark-800 dark:disabled:text-dark-400"
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
            class="pointer-events-none absolute right-0 top-full z-50 mt-3 w-[25rem] max-w-[calc(100vw-2rem)] translate-y-1 rounded-2xl border border-emerald-100 bg-white p-4 text-left opacity-0 shadow-[0_24px_70px_-32px_rgba(15,118,110,0.45)] ring-1 ring-emerald-50 transition-all duration-150 group-hover/checkin:pointer-events-auto group-hover/checkin:translate-y-0 group-hover/checkin:opacity-100 group-focus-within/checkin:pointer-events-auto group-focus-within/checkin:translate-y-0 group-focus-within/checkin:opacity-100 dark:border-dark-700 dark:bg-dark-800 dark:ring-dark-700"
            data-test="daily-checkin-popover"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('checkin.cardTitle') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ checkinStatus?.checked_in ? t('checkin.cardCheckedHint') : t('checkin.cardHint') }}
                </p>
              </div>
              <span
                v-if="checkinStatus?.checked_in && displayRewardAmount > 0"
                class="rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200"
              >
                +{{ formatUsd(displayRewardAmount) }}
              </span>
            </div>

            <div class="mt-4 grid grid-cols-2 gap-3">
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('checkin.currentStreak') }}</p>
                <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('checkin.days', { count: checkinStatus?.current_streak || 0 }) }}
                </p>
              </div>
              <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-700">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('checkin.lifetimeDays') }}</p>
                <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('checkin.days', { count: checkinStatus?.lifetime_checkin_days || 0 }) }}
                </p>
              </div>
            </div>

            <div class="mt-3 rounded-xl border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-700">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-xs font-semibold text-gray-700 dark:text-gray-200">
                    {{ t('checkin.eligibilityTitle') }}
                  </p>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ eligibilityMessage }}
                  </p>
                </div>
                <span
                  class="shrink-0 rounded-full px-2.5 py-1 text-[11px] font-semibold"
                  :class="checkinStatus?.eligible === false
                    ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
                    : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'"
                >
                  {{ checkinStatus?.eligible === false ? t('checkin.eligibilityPendingBadge') : t('checkin.eligibilityReadyBadge') }}
                </span>
              </div>
              <div v-if="checkinMinSpend > 0" class="mt-3 h-1.5 overflow-hidden rounded-full bg-white dark:bg-dark-800">
                <div
                  class="h-full rounded-full bg-emerald-500 transition-all"
                  :style="{ width: `${eligibilityProgressPercent}%` }"
                />
              </div>
            </div>

            <div class="mt-4">
              <p class="text-xs font-semibold text-gray-700 dark:text-gray-200">
                {{ t('checkin.rewardBreakdown') }}
              </p>
            </div>

            <div class="mt-2 grid grid-cols-2 gap-2">
              <div class="rounded-xl border border-emerald-100 bg-emerald-50 p-3 dark:border-emerald-500/20 dark:bg-emerald-900/20">
                <p class="text-[11px] font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-200">
                  {{ t('checkin.baseReward') }}
                </p>
                <p class="mt-1 text-sm font-bold text-emerald-800 dark:text-emerald-100">
                  {{ baseRewardLabel }}
                </p>
              </div>
              <div class="rounded-xl border border-amber-100 bg-amber-50 p-3 dark:border-amber-500/20 dark:bg-amber-900/20">
                <p class="text-[11px] font-semibold uppercase tracking-wide text-amber-700 dark:text-amber-200">
                  {{ t('checkin.streakBonus') }}
                </p>
                <p class="mt-1 text-sm font-bold text-amber-800 dark:text-amber-100">
                  {{ streakBonusLabel }}
                </p>
              </div>
            </div>

            <div class="mt-3 rounded-xl border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-900/20 dark:text-amber-200">
              <p class="font-semibold">{{ t('checkin.streakBonusTitle') }}</p>
              <p class="mt-1">
                {{ nextStreakBonusMessage }}
              </p>
            </div>

            <div v-if="recentCheckinRecords.length > 0" class="mt-4 space-y-2">
              <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                {{ t('checkin.recentRecords') }}
              </p>
              <div
                v-for="record in recentCheckinRecords"
                :key="record.id"
                class="flex items-center justify-between rounded-xl bg-gray-50 px-3 py-2 text-xs dark:bg-dark-700"
              >
                <div>
                  <p class="font-semibold text-gray-800 dark:text-gray-100">{{ record.checkin_date }}</p>
                  <p class="text-gray-500 dark:text-dark-400">
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

        <!-- User Dropdown -->
        <div v-if="user" class="relative" ref="dropdownRef">
          <button
            @click="toggleDropdown"
            class="flex items-center gap-2 rounded-xl p-1.5 transition-colors hover:bg-gray-100 dark:hover:bg-dark-800"
            aria-label="User Menu"
          >
            <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 text-sm font-medium text-white shadow-sm">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="displayName"
                class="h-full w-full object-cover"
              >
              <span v-else>{{ userInitials }}</span>
            </div>
            <div class="hidden text-left md:block">
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ displayName }}
              </div>
              <div class="text-xs capitalize text-gray-500 dark:text-dark-400">
                {{ user.role }}
              </div>
            </div>
            <Icon name="chevronDown" size="sm" class="hidden text-gray-400 md:block" />
          </button>

          <!-- Dropdown Menu -->
          <transition name="dropdown">
            <div v-if="dropdownOpen" class="dropdown right-0 mt-2 w-56">
              <!-- User Info -->
              <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ displayName }}
                </div>
                <div class="text-xs text-gray-500 dark:text-dark-400">{{ user.email }}</div>
              </div>

              <!-- Balance (mobile only) -->
              <div class="border-b border-gray-100 px-4 py-2 dark:border-dark-700 sm:hidden">
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('common.balance') }}
                </div>
                <div class="text-sm font-semibold text-primary-600 dark:text-primary-400">
                  ${{ user.balance?.toFixed(2) || '0.00' }}
                </div>
              </div>

              <div class="py-1">
                <router-link to="/profile" @click="closeDropdown" class="dropdown-item">
                  <Icon name="user" size="sm" />
                  {{ t('nav.profile') }}
                </router-link>

                <router-link to="/keys" @click="closeDropdown" class="dropdown-item">
                  <Icon name="key" size="sm" />
                  {{ t('nav.apiKeys') }}
                </router-link>

                <a
                  v-if="authStore.isAdmin"
                  href="https://github.com/Wei-Shaw/sub2api"
                  target="_blank"
                  rel="noopener noreferrer"
                  @click="closeDropdown"
                  class="dropdown-item"
                >
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                    <path
                      fill-rule="evenodd"
                      clip-rule="evenodd"
                      d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.17 6.839 9.49.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.464-1.11-1.464-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.336-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.167 22 16.418 22 12c0-5.523-4.477-10-10-10z"
                    />
                  </svg>
                  {{ t('nav.github') }}
                </a>

              </div>

              <!-- Contact Support (only show if configured) -->
              <div
                v-if="contactInfo"
                class="border-t border-gray-100 px-4 py-2.5 dark:border-dark-700"
              >
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <svg
                    class="h-3.5 w-3.5 flex-shrink-0"
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
                  <span>{{ t('common.contactSupport') }}:</span>
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{
                    contactInfo
                  }}</span>
                </div>
              </div>

              <div v-if="showOnboardingButton" class="border-t border-gray-100 py-1 dark:border-dark-700">
                <button @click="handleReplayGuide" class="dropdown-item w-full">
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                    <path
                      d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 14a1 1 0 110 2 1 1 0 010-2zm1.07-7.75c0-.6-.49-1.25-1.32-1.25-.7 0-1.22.4-1.43 1.02a1 1 0 11-1.9-.62A3.41 3.41 0 0111.8 5c2.02 0 3.25 1.4 3.25 2.9 0 2-1.83 2.55-2.43 3.12-.43.4-.47.75-.47 1.23a1 1 0 01-2 0c0-1 .16-1.82 1.1-2.7.69-.64 1.82-1.05 1.82-2.06z"
                    />
                  </svg>
                  {{ $t('onboarding.restartTour') }}
                </button>
              </div>

              <div class="border-t border-gray-100 py-1 dark:border-dark-700">
                <button
                  @click="handleLogout"
                  class="dropdown-item w-full text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75"
                    />
                  </svg>
                  {{ t('nav.logout') }}
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
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
