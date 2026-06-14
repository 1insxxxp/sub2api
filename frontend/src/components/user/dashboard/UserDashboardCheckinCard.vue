<template>
  <div
    v-if="visible"
    data-testid="checkin-card"
    class="card overflow-hidden"
  >
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <div class="flex items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-amber-100 dark:bg-amber-900/30">
            <Icon name="gift" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
          </div>
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.checkin.title') }}
            </h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{ checkedInToday ? t('dashboard.checkin.doneHint') : t('dashboard.checkin.hint') }}
            </p>
          </div>
        </div>
        <span
          v-if="checkedInToday"
          class="inline-flex flex-shrink-0 items-center gap-1 rounded-full bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
        >
          <Icon name="check" size="xs" :stroke-width="2" />
          {{ t('dashboard.checkin.done') }}
        </span>
      </div>
    </div>

    <div class="space-y-4 p-5">
      <div v-if="loading" class="space-y-3">
        <div class="h-4 w-2/3 animate-pulse rounded bg-gray-100 dark:bg-dark-700" />
        <div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700" />
      </div>

      <template v-else>
        <div class="grid grid-cols-2 gap-3">
          <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/50">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.checkin.currentStreak') }}</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.checkin.streak', { days: status?.current_streak || 0 }) }}
            </p>
          </div>
          <div class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800/50">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('dashboard.checkin.lifetimeDays') }}</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('dashboard.checkin.lifetime', { days: status?.lifetime_checkin_days || 0 }) }}
            </p>
          </div>
        </div>

        <p
          v-if="checkedInToday && rewardAmount > 0"
          class="rounded-lg bg-emerald-50 px-3 py-2 text-sm font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300"
        >
          {{ t('dashboard.checkin.reward', { amount: formatMoney(rewardAmount) }) }}
        </p>

        <button
          data-testid="checkin-submit"
          type="button"
          :disabled="!canSubmit"
          class="inline-flex w-full items-center justify-center gap-2 rounded-lg bg-amber-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-amber-700 disabled:cursor-not-allowed disabled:bg-gray-200 disabled:text-gray-500 dark:disabled:bg-dark-700 dark:disabled:text-dark-400"
          @click="submitCheckin"
        >
          <Icon :name="checkedInToday ? 'checkCircle' : 'sparkles'" size="sm" :stroke-width="2" />
          {{ buttonLabel }}
        </button>

        <p v-if="errorMessage" class="text-xs text-red-600 dark:text-red-400">
          {{ errorMessage }}
        </p>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { checkin, getCheckinStatus } from '@/api/user'
import type { UserCheckinResult, UserCheckinStatus } from '@/types'

const emit = defineEmits<{
  (event: 'checked-in', result: UserCheckinResult): void
}>()

const { t } = useI18n()

const status = ref<UserCheckinStatus | null>(null)
const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')

const visible = computed(() => loading.value || status.value?.enabled === true)
const checkedInToday = computed(() => status.value?.checked_in_today === true)
const rewardAmount = computed(() => status.value?.total_reward_amount || 0)
const canSubmit = computed(() => Boolean(status.value?.enabled) && !checkedInToday.value && !submitting.value)
const buttonLabel = computed(() => {
  if (submitting.value) return t('dashboard.checkin.submitting')
  if (checkedInToday.value) return t('dashboard.checkin.checked')
  return t('dashboard.checkin.submit')
})

const formatMoney = (value: number) => value.toFixed(2)

const loadStatus = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    status.value = await getCheckinStatus()
  } catch (error: any) {
    errorMessage.value = error?.message || t('dashboard.checkin.loadFailed')
    status.value = null
  } finally {
    loading.value = false
  }
}

const submitCheckin = async () => {
  if (!canSubmit.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    const result = await checkin()
    status.value = result
    emit('checked-in', result)
  } catch (error: any) {
    errorMessage.value = error?.message || t('dashboard.checkin.failed')
  } finally {
    submitting.value = false
  }
}

onMounted(loadStatus)
</script>
