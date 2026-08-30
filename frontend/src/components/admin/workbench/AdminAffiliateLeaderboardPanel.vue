<template>
  <section data-test="affiliate-leaderboard-panel" class="min-w-0 space-y-4">
    <header class="flex min-w-0 items-start gap-3">
      <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
        <Icon name="users" size="md" />
      </span>
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-950 dark:text-white">
          {{ t('adminWorkbench.affiliateLeaderboard.title') }}
        </h2>
        <p class="mt-1 text-sm leading-5 text-gray-500 dark:text-dark-400">
          {{ t('adminWorkbench.affiliateLeaderboard.subtitle') }}
        </p>
      </div>
    </header>

    <div
      v-if="loading"
      data-test="affiliate-leaderboard-loading"
      class="flex min-h-56 items-center justify-center border-y border-gray-200 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
    >
      {{ t('common.loading') }}
    </div>

    <div
      v-else-if="errorMessage"
      data-test="affiliate-leaderboard-error"
      class="flex min-h-56 items-center justify-center border-y border-red-100 bg-red-50/50 px-5 text-center text-sm text-red-700 dark:border-red-500/20 dark:bg-red-500/5 dark:text-red-300"
    >
      {{ errorMessage }}
    </div>

    <div
      v-else-if="leaders.length === 0"
      data-test="affiliate-leaderboard-empty"
      class="flex min-h-56 items-center justify-center border-y border-gray-200 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
    >
      {{ t('adminWorkbench.affiliateLeaderboard.empty') }}
    </div>

    <template v-else>
      <div
        data-test="affiliate-leaderboard-desktop"
        class="hidden min-w-0 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900 md:block"
      >
        <table class="w-full table-fixed border-collapse text-left">
          <colgroup>
            <col class="w-20" />
            <col />
            <col class="w-28" />
            <col class="w-28" />
            <col class="w-32" />
            <col class="w-44" />
          </colgroup>
          <thead class="bg-gray-50 text-xs font-medium text-gray-500 dark:bg-dark-800/70 dark:text-dark-400">
            <tr>
              <th class="px-4 py-3">{{ t('adminWorkbench.affiliateLeaderboard.rank') }}</th>
              <th class="px-4 py-3">{{ t('adminWorkbench.affiliateLeaderboard.user') }}</th>
              <th class="px-4 py-3 text-right">{{ t('adminWorkbench.affiliateLeaderboard.invitedCount') }}</th>
              <th class="px-4 py-3 text-right">{{ t('adminWorkbench.affiliateLeaderboard.qualifiedCount') }}</th>
              <th class="px-4 py-3 text-right">{{ t('adminWorkbench.affiliateLeaderboard.totalRebate') }}</th>
              <th class="px-4 py-3">{{ t('adminWorkbench.affiliateLeaderboard.lastInvitedAt') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr
              v-for="(leader, index) in leaders"
              :key="leader.inviter_id"
              :data-test="`affiliate-leaderboard-row-${leader.inviter_id}`"
              class="text-sm text-gray-700 dark:text-dark-200"
            >
              <td class="px-4 py-3.5">
                <span
                  :data-test="`affiliate-leaderboard-rank-${index + 1}`"
                  :class="rankClass(index)"
                >
                  {{ index + 1 }}
                </span>
              </td>
              <td class="min-w-0 px-4 py-3.5">
                <div class="flex min-w-0 items-center gap-3">
                  <span
                    class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-primary-50 text-sm font-semibold text-primary-700 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-200 dark:ring-primary-500/20"
                    aria-hidden="true"
                  >
                    <img
                      v-if="shouldShowAvatar(leader)"
                      :data-test="`affiliate-leaderboard-avatar-image-${leader.inviter_id}`"
                      :src="leader.inviter_avatar_url"
                      alt=""
                      class="h-full w-full object-cover"
                      @error="markAvatarFailed(leader.inviter_id)"
                    />
                    <span
                      v-else
                      :data-test="`affiliate-leaderboard-avatar-fallback-${leader.inviter_id}`"
                    >
                      {{ avatarInitial(leader) }}
                    </span>
                  </span>
                  <div class="min-w-0">
                    <p class="truncate font-medium text-gray-950 dark:text-white" :title="leader.inviter_email || '-'">
                      {{ leader.inviter_email || '-' }}
                    </p>
                    <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-dark-400">
                      {{ leader.inviter_username || t('adminWorkbench.affiliateLeaderboard.noUsername') }} · #{{ leader.inviter_id }}
                    </p>
                  </div>
                </div>
              </td>
              <td class="px-4 py-3.5 text-right text-base font-semibold tabular-nums text-primary-700 dark:text-primary-300">
                {{ leader.invited_count }}
              </td>
              <td class="px-4 py-3.5 text-right font-semibold tabular-nums text-emerald-700 dark:text-emerald-300">
                {{ leader.qualified_invitee_count }}
              </td>
              <td class="px-4 py-3.5 text-right font-medium tabular-nums text-gray-950 dark:text-white">
                {{ formatCurrency(leader.total_rebate) }}
              </td>
              <td class="px-4 py-3.5 text-xs text-gray-500 dark:text-dark-400">
                {{ formatLastInvite(leader.last_invited_at) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div data-test="affiliate-leaderboard-mobile" class="space-y-3 md:hidden">
        <article
          v-for="(leader, index) in leaders"
          :key="leader.inviter_id"
          class="rounded-lg border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-700 dark:bg-dark-900 min-[360px]:p-4"
        >
          <div class="flex min-w-0 items-start gap-3">
            <span :class="rankClass(index)">{{ index + 1 }}</span>
            <span
              class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-primary-50 text-sm font-semibold text-primary-700 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-200 dark:ring-primary-500/20"
              aria-hidden="true"
            >
              <img
                v-if="shouldShowAvatar(leader)"
                :data-test="`affiliate-leaderboard-avatar-image-${leader.inviter_id}`"
                :src="leader.inviter_avatar_url"
                alt=""
                class="h-full w-full object-cover"
                @error="markAvatarFailed(leader.inviter_id)"
              />
              <span
                v-else
                :data-test="`affiliate-leaderboard-avatar-fallback-${leader.inviter_id}`"
              >
                {{ avatarInitial(leader) }}
              </span>
            </span>
            <div
              :data-test="`affiliate-leaderboard-mobile-identity-${leader.inviter_id}`"
              class="min-w-0 flex-1 break-words"
            >
              <p class="break-words text-sm font-semibold text-gray-950 dark:text-white">
                {{ leader.inviter_email || '-' }}
              </p>
              <p class="mt-0.5 break-words text-xs text-gray-500 dark:text-dark-400">
                {{ leader.inviter_username || t('adminWorkbench.affiliateLeaderboard.noUsername') }} · #{{ leader.inviter_id }}
              </p>
            </div>
          </div>
          <dl
            :data-test="`affiliate-leaderboard-mobile-metrics-${leader.inviter_id}`"
            class="mt-4 grid grid-cols-2 gap-2 border-t border-gray-100 pt-3 dark:border-dark-700"
          >
            <div class="min-w-0 rounded-md bg-primary-50/70 px-3 py-2 dark:bg-primary-500/10">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.affiliateLeaderboard.invitedCount') }}</dt>
              <dd class="mt-1 whitespace-nowrap font-semibold tabular-nums text-primary-700 dark:text-primary-300">{{ leader.invited_count }}</dd>
            </div>
            <div class="min-w-0 rounded-md bg-emerald-50/70 px-3 py-2 dark:bg-emerald-500/10">
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.affiliateLeaderboard.qualifiedCount') }}</dt>
              <dd class="mt-1 whitespace-nowrap font-semibold tabular-nums text-emerald-700 dark:text-emerald-300">{{ leader.qualified_invitee_count }}</dd>
            </div>
            <div
              :data-test="`affiliate-leaderboard-mobile-rebate-${leader.inviter_id}`"
              class="col-span-2 flex min-w-0 items-center justify-between gap-3 rounded-md bg-gray-50 px-3 py-2 dark:bg-dark-800/70"
            >
              <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('adminWorkbench.affiliateLeaderboard.totalRebate') }}</dt>
              <dd class="shrink-0 whitespace-nowrap font-semibold tabular-nums text-gray-950 dark:text-white">{{ formatCurrency(leader.total_rebate) }}</dd>
            </div>
          </dl>
          <p class="mt-3 break-words text-xs leading-5 text-gray-500 dark:text-dark-400">
            {{ t('adminWorkbench.affiliateLeaderboard.lastInvitedAt') }}: {{ formatLastInvite(leader.last_invited_at) }}
          </p>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { affiliatesAPI, type WorkbenchAffiliateLeaderboardItem } from '@/api/admin/affiliates'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const leaders = ref<WorkbenchAffiliateLeaderboardItem[]>([])
const loading = ref(true)
const errorMessage = ref('')
const failedAvatarIds = ref<Set<number>>(new Set())

function shouldShowAvatar(leader: WorkbenchAffiliateLeaderboardItem): boolean {
  return Boolean(leader.inviter_avatar_url?.trim()) && !failedAvatarIds.value.has(leader.inviter_id)
}

function avatarInitial(leader: WorkbenchAffiliateLeaderboardItem): string {
  const identity = leader.inviter_username?.trim() || leader.inviter_email?.trim() || String(leader.inviter_id)
  return Array.from(identity)[0]?.toUpperCase() || '?'
}

function markAvatarFailed(inviterId: number): void {
  failedAvatarIds.value = new Set(failedAvatarIds.value).add(inviterId)
}

function rankClass(index: number): string {
  const base = 'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold tabular-nums'
  if (index === 0) return `${base} bg-amber-100 text-amber-800 dark:bg-amber-500/20 dark:text-amber-200`
  if (index === 1) return `${base} bg-gray-200 text-gray-700 dark:bg-dark-700 dark:text-dark-200`
  if (index === 2) return `${base} bg-orange-100 text-orange-800 dark:bg-orange-500/20 dark:text-orange-200`
  return `${base} bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-300`
}

function formatLastInvite(value: string | null): string {
  return value ? formatDateTime(value) : '-'
}

async function loadLeaderboard() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await affiliatesAPI.getWorkbenchLeaderboard()
    leaders.value = response.items || []
  } catch (error) {
    errorMessage.value = extractApiErrorMessage(error, t('adminWorkbench.affiliateLeaderboard.loadFailed'))
    appStore.showError(errorMessage.value)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadLeaderboard()
})
</script>
