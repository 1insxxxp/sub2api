<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="border-b border-slate-100 bg-slate-50/80 px-5 py-4 dark:border-dark-700 dark:bg-dark-900/30">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.checkins.overviewTitle') }}
                </h2>
                <span
                  class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
                  :class="configForm.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'"
                >
                  {{ configForm.enabled ? t('admin.checkins.enabledStatus') : t('admin.checkins.disabledStatus') }}
                </span>
              </div>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
                {{ t('admin.checkins.configDescription') }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="btn btn-secondary inline-flex items-center gap-2"
                :disabled="configLoading || statsLoading || recordsLoading || blacklistLoading"
                @click="refreshAll"
              >
                <Icon name="refresh" size="sm" :class="(configLoading || statsLoading || recordsLoading || blacklistLoading) ? 'animate-spin' : ''" />
                {{ t('common.refresh') }}
              </button>
              <button
                type="button"
                class="btn btn-primary inline-flex items-center gap-2"
                :disabled="configSaving || configLoading"
                @click="handleSaveConfig"
              >
                <Icon name="check" size="sm" />
                {{ configSaving ? t('common.saving') : t('common.save') }}
              </button>
            </div>
          </div>
        </div>

        <div class="grid gap-4 p-5 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="item in statsCards"
          :key="item.key"
            class="rounded-lg border border-gray-100 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between gap-3">
            <div>
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
                {{ item.label }}
              </p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ item.value }}
              </p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ item.meta }}
              </p>
            </div>
            <div
              class="flex h-9 w-9 items-center justify-center rounded-lg"
              :class="item.iconClass"
            >
              <Icon :name="item.icon" size="sm" />
            </div>
          </div>
        </div>
        </div>
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(280px,360px),1fr]">
        <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex items-center gap-3">
              <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300">
                <Icon name="shield" size="sm" />
              </div>
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.checkins.baseRules') }}
                </h3>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.baseRulesHint') }}
                </p>
              </div>
            </div>
          </div>

          <div class="space-y-5 p-5">
            <label
              class="flex cursor-pointer items-center justify-between gap-4 rounded-lg border px-4 py-3 transition-colors"
              :class="configForm.enabled ? 'border-primary-200 bg-primary-50/70 dark:border-primary-500/30 dark:bg-primary-900/10' : 'border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-700/40'"
            >
              <span>
                <span class="block text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.checkins.enabled') }}
                </span>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">
                  {{ configForm.enabled ? t('admin.checkins.enabledStatus') : t('admin.checkins.disabledStatus') }}
                </span>
              </span>
              <input
                v-model="configForm.enabled"
                type="checkbox"
                class="h-5 w-5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
            </label>

            <div>
              <label class="input-label">{{ t('admin.checkins.minTotalUsageUsd') }}</label>
              <div class="relative">
                <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm font-semibold text-gray-400">$</span>
                <input
                  v-model.number="configForm.min_total_usage_usd"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input pl-7"
                />
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.checkins.minSpendHint') }}
              </p>
            </div>

            <div class="rounded-lg bg-gray-50 px-4 py-3 dark:bg-dark-700/60">
              <div class="flex items-center justify-between gap-3 text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('admin.checkins.rewardRules') }}</span>
                <span class="font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.checkins.tierCount', { count: configForm.tiers.length }) }}
                </span>
              </div>
              <div class="mt-2 flex items-center justify-between gap-3 text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('admin.checkins.streakRules') }}</span>
                <span class="font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.checkins.ruleCount', { count: configForm.streak_rules.length }) }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div class="flex items-center gap-3">
                  <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-900/20 dark:text-amber-300">
                    <Icon name="gift" size="sm" />
                  </div>
                  <div>
                    <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                      {{ t('admin.checkins.rewardStrategy') }}
                    </h3>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      {{ t('admin.checkins.rewardRulesHint') }}
                    </p>
                  </div>
                </div>
              </div>
              <div
                class="flex flex-wrap items-center gap-2 rounded-lg px-3 py-2 text-xs"
                :class="rewardProbabilityTotalValid ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-200' : 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-200'"
              >
                <span class="font-semibold">{{ t('admin.checkins.probabilityTotal', { total: rewardProbabilityTotal.toFixed(2) }) }}</span>
                <span class="text-gray-400 dark:text-dark-500">/</span>
                <span>{{ t('admin.checkins.averageReward', { amount: formatUsd(estimatedAverageReward) }) }}</span>
              </div>
            </div>
          </div>

          <div class="space-y-6 p-5">
            <section>
              <div class="mb-3 flex items-center justify-between gap-3">
                <div>
                  <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.checkins.rewardRules') }}
                  </h4>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ t('admin.checkins.rewardRulesHint') }}
                  </p>
                </div>
                <button type="button" class="btn btn-secondary btn-sm" @click="addRewardTier">
                  <Icon name="plus" size="sm" />
                  {{ t('admin.checkins.addTier') }}
                </button>
              </div>

              <div class="overflow-hidden rounded-lg border border-gray-100 dark:border-dark-700">
                <div class="grid grid-cols-[minmax(92px,1fr),minmax(92px,1fr),minmax(76px,96px)] gap-3 bg-gray-50 px-3 py-2 text-xs font-semibold text-gray-500 dark:bg-dark-700/60 dark:text-dark-300">
                  <span>{{ t('admin.checkins.rewardAmountUsd') }}</span>
                  <span>{{ t('admin.checkins.probabilityPercent') }}</span>
                  <span class="text-right">{{ t('admin.checkins.columns.actions') }}</span>
                </div>
                <div
                  v-for="(tier, index) in configForm.tiers"
                  :key="`reward-tier-${index}`"
                  class="grid grid-cols-[minmax(92px,1fr),minmax(92px,1fr),minmax(76px,96px)] items-center gap-3 border-t border-gray-100 px-3 py-2 dark:border-dark-700"
                >
                  <div class="flex items-center gap-2">
                    <span class="hidden min-w-8 text-xs font-semibold text-gray-400 sm:inline">
                      #{{ index + 1 }}
                    </span>
                    <input v-model.number="tier.amount" type="number" min="0.01" step="0.01" class="input h-9" />
                  </div>
                  <input v-model.number="tier.probability" type="number" min="0.01" step="0.01" class="input h-9" />
                  <div class="flex justify-end">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm px-2"
                      :disabled="configForm.tiers.length <= 1"
                      :title="t('common.delete')"
                      @click="removeRewardTier(index)"
                    >
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                </div>
              </div>
            </section>

            <section class="border-t border-gray-100 pt-5 dark:border-dark-700">
              <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
                    {{ t('admin.checkins.streakMilestones') }}
                  </h4>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ t('admin.checkins.streakRulesHint') }}
                  </p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <label class="inline-flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-gray-200">
                    <input
                      v-model="configForm.streak_enabled"
                      type="checkbox"
                      class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    />
                    {{ t('admin.checkins.streakEnabled') }}
                  </label>
                  <button type="button" class="btn btn-secondary btn-sm" @click="addStreakRule">
                    <Icon name="plus" size="sm" />
                    {{ t('admin.checkins.addStreakRule') }}
                  </button>
                </div>
              </div>

              <div v-if="configForm.streak_enabled" class="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
                <div
                  v-for="(rule, index) in configForm.streak_rules"
                  :key="`streak-rule-${index}`"
                  class="rounded-lg border border-amber-100 bg-amber-50/40 p-3 dark:border-amber-500/20 dark:bg-amber-900/10"
                >
                  <div class="mb-3 flex items-center justify-between gap-2">
                    <span class="inline-flex items-center rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-amber-700 shadow-sm dark:bg-dark-800 dark:text-amber-200">
                      {{ t('admin.checkins.streakDayValue', { day: rule.day || 0 }) }}
                    </span>
                    <button
                      type="button"
                      class="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-white hover:text-red-500 dark:hover:bg-dark-800"
                      :title="t('common.delete')"
                      @click="removeStreakRule(index)"
                    >
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                  <div class="grid grid-cols-2 gap-2">
                    <div>
                      <label class="input-label">{{ t('admin.checkins.streakDay') }}</label>
                      <input v-model.number="rule.day" type="number" min="1" step="1" class="input h-9" />
                    </div>
                    <div>
                      <label class="input-label">{{ t('admin.checkins.streakBonusUsd') }}</label>
                      <input v-model.number="rule.bonus_amount" type="number" min="0.01" step="0.01" class="input h-9" />
                    </div>
                  </div>
                </div>
                <p v-if="configForm.streak_rules.length === 0" class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-700/50 dark:text-dark-400">
                  {{ t('admin.checkins.emptyStreakRules') }}
                </p>
              </div>
            </section>
          </div>
      </div>
      </section>

      <section class="card rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="border-b border-gray-100 p-4 dark:border-dark-700">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.checkins.recordsTitle') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.checkins.recordsHint') }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-3">
              <div class="min-w-56 flex-1 lg:w-72 lg:flex-none">
                <input
                  v-model="recordFilters.search"
                  type="text"
                  class="input"
                  :placeholder="t('admin.checkins.recordsSearchPlaceholder')"
                  @input="handleRecordSearch"
                />
              </div>
              <input
                v-model="recordFilters.date"
                type="date"
                class="input w-44"
                @change="reloadRecordsFromFirstPage"
              />
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="recordsLoading"
                :title="t('common.refresh')"
                @click="loadRecords"
              >
                <Icon name="refresh" size="md" :class="recordsLoading ? 'animate-spin' : ''" />
              </button>
            </div>
          </div>
        </div>

        <DataTable
          :columns="recordColumns"
          :data="records"
          :loading="recordsLoading"
          row-key="id"
          :sticky-actions-column="false"
        >
          <template #cell-user="{ row }">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ userLabel(row) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.checkins.userId', { id: row.user_id }) }}
              </div>
            </div>
          </template>
          <template #cell-reward_amount="{ value }">
            <span class="font-semibold text-amber-600 dark:text-amber-400">
              +{{ formatUsd(value) }}
            </span>
          </template>
          <template #cell-streak_day="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-200">
              {{ t('admin.checkins.streakDayValue', { day: value || 1 }) }}
            </span>
          </template>
          <template #cell-reward_detail="{ row }">
            <span class="text-xs text-gray-500 dark:text-dark-400">
              {{ formatUsd(row.base_reward_amount || row.reward_amount) }}
              <template v-if="row.bonus_reward_amount > 0">
                + {{ formatUsd(row.bonus_reward_amount) }}
              </template>
            </span>
          </template>
          <template #cell-balance_before="{ value }">
            {{ formatUsd(value) }}
          </template>
          <template #cell-balance_after="{ value }">
            {{ formatUsd(value) }}
          </template>
          <template #cell-created_at="{ value }">
            <span class="text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>
        </DataTable>

        <Pagination
          v-if="recordsPagination.total > 0"
          :page="recordsPagination.page"
          :total="recordsPagination.total"
          :page-size="recordsPagination.page_size"
          @update:page="handleRecordsPageChange"
          @update:pageSize="handleRecordsPageSizeChange"
        />
      </section>

      <section class="grid gap-6 xl:grid-cols-[minmax(320px,420px),1fr]">
        <div class="card rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="mb-4">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.checkins.addBlacklist') }}
            </h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.checkins.addBlacklistHint') }}
            </p>
          </div>

          <div class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.checkins.searchUser') }}</label>
              <div class="flex gap-2">
                <input
                  v-model="blacklistUserSearch"
                  data-test="blacklist-user-search"
                  type="text"
                  class="input"
                  :placeholder="t('admin.checkins.userSearchPlaceholder')"
                  @keyup.enter="searchBlacklistUsers"
                />
                <button
                  type="button"
                  data-test="search-blacklist-user"
                  class="btn btn-secondary"
                  :disabled="userSearchLoading"
                  @click="searchBlacklistUsers"
                >
                  <Icon name="search" size="sm" />
                </button>
              </div>
            </div>

            <div
              v-if="userCandidates.length > 0"
              class="max-h-56 overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-700"
            >
              <button
                v-for="candidate in userCandidates"
                :key="candidate.id"
                type="button"
                :data-test="`select-blacklist-user-${candidate.id}`"
                class="flex w-full items-center justify-between gap-3 border-b border-gray-100 px-3 py-2 text-left transition-colors last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700"
                @click="selectBlacklistUser(candidate)"
              >
                <span class="min-w-0">
                  <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">
                    {{ candidate.email }}
                  </span>
                  <span class="block text-xs text-gray-500 dark:text-dark-400">
                    {{ candidate.username || t('admin.checkins.userId', { id: candidate.id }) }}
                  </span>
                </span>
                <Icon v-if="selectedBlacklistUser?.id === candidate.id" name="check" size="sm" class="text-primary-500" />
              </button>
            </div>

            <div
              v-if="selectedBlacklistUser"
              class="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:bg-dark-700 dark:text-gray-200"
            >
              {{ selectedBlacklistUser.email }}
            </div>

            <div>
              <label class="input-label">{{ t('admin.checkins.reason') }}</label>
              <textarea
                v-model="blacklistReason"
                data-test="blacklist-reason"
                rows="3"
                class="input"
                :placeholder="t('admin.checkins.reasonPlaceholder')"
              ></textarea>
            </div>

            <button
              type="button"
              data-test="add-blacklist"
              class="btn btn-primary w-full"
              :disabled="!selectedBlacklistUser || addingBlacklist"
              @click="handleAddBlacklist"
            >
              {{ addingBlacklist ? t('common.saving') : t('admin.checkins.addToBlacklist') }}
            </button>
          </div>
        </div>

        <div class="card rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-100 p-4 dark:border-dark-700">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.checkins.blacklistTitle') }}
                </h3>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.blacklistHint') }}
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-3">
                <div class="min-w-56 flex-1 lg:w-72 lg:flex-none">
                  <input
                    v-model="blacklistFilters.search"
                    type="text"
                    class="input"
                    :placeholder="t('admin.checkins.blacklistSearchPlaceholder')"
                    @input="handleBlacklistSearch"
                  />
                </div>
                <button
                  type="button"
                  class="btn btn-secondary"
                  :disabled="blacklistLoading"
                  :title="t('common.refresh')"
                  @click="loadBlacklist"
                >
                  <Icon name="refresh" size="md" :class="blacklistLoading ? 'animate-spin' : ''" />
                </button>
              </div>
            </div>
          </div>

          <DataTable
            :columns="blacklistColumns"
            :data="blacklist"
            :loading="blacklistLoading"
            row-key="id"
          >
            <template #cell-user="{ row }">
              <div>
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ userLabel(row) }}
                </div>
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.userId', { id: row.user_id }) }}
                </div>
              </div>
            </template>
            <template #cell-reason="{ value }">
              <span class="text-gray-600 dark:text-gray-300">{{ value || '-' }}</span>
            </template>
            <template #cell-created_at="{ value }">
              <span class="text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
            </template>
            <template #cell-actions="{ row }">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="removingUserId === row.user_id"
                @click="handleRemoveBlacklist(row.user_id)"
              >
                {{ t('admin.checkins.removeBlacklist') }}
              </button>
            </template>
          </DataTable>

          <Pagination
            v-if="blacklistPagination.total > 0"
            :page="blacklistPagination.page"
            :total="blacklistPagination.total"
            :page-size="blacklistPagination.page_size"
            @update:page="handleBlacklistPageChange"
            @update:pageSize="handleBlacklistPageSizeChange"
          />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  AdminCheckinBlacklistEntry,
  AdminCheckinConfig,
  AdminCheckinRecord,
  AdminCheckinStats,
  CheckinRewardTier,
  CheckinStreakRule,
} from '@/api/admin'
import type { AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const stats = ref<AdminCheckinStats | null>(null)
const config = ref<AdminCheckinConfig | null>(null)
const records = ref<AdminCheckinRecord[]>([])
const blacklist = ref<AdminCheckinBlacklistEntry[]>([])
const userCandidates = ref<AdminUser[]>([])
const selectedBlacklistUser = ref<AdminUser | null>(null)

const statsLoading = ref(false)
const configLoading = ref(false)
const configSaving = ref(false)
const recordsLoading = ref(false)
const blacklistLoading = ref(false)
const userSearchLoading = ref(false)
const addingBlacklist = ref(false)
const removingUserId = ref<number | null>(null)

const recordFilters = reactive({
  search: '',
  date: '',
})

const blacklistFilters = reactive({
  search: '',
})

const recordsPagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})

const blacklistPagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})

const blacklistUserSearch = ref('')
const blacklistReason = ref('')
const configForm = reactive({
  enabled: true,
  min_total_usage_usd: 0,
  tiers: [] as CheckinRewardTier[],
  streak_enabled: true,
  streak_rules: [] as CheckinStreakRule[],
})
let recordSearchTimeout: ReturnType<typeof setTimeout> | undefined
let blacklistSearchTimeout: ReturnType<typeof setTimeout> | undefined

const statsCards = computed(() => [
  {
    key: 'today',
    label: t('admin.checkins.todayCount'),
    value: stats.value?.today_count ?? 0,
    meta: formatUsd(stats.value?.today_reward_total ?? 0),
    icon: 'calendar' as const,
    iconClass: 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300',
  },
  {
    key: 'seven',
    label: t('admin.checkins.sevenDayCount'),
    value: stats.value?.seven_day_count ?? 0,
    meta: formatUsd(stats.value?.seven_day_reward_total ?? 0),
    icon: 'chart' as const,
    iconClass: 'bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-300',
  },
  {
    key: 'thirty',
    label: t('admin.checkins.thirtyDayCount'),
    value: stats.value?.thirty_day_count ?? 0,
    meta: formatUsd(stats.value?.thirty_day_reward_total ?? 0),
    icon: 'trendingUp' as const,
    iconClass: 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300',
  },
  {
    key: 'blacklist',
    label: t('admin.checkins.activeBlacklist'),
    value: stats.value?.active_blacklist_count ?? 0,
    meta: stats.value?.current_checkin_day || '-',
    icon: 'ban' as const,
    iconClass: 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-300',
  },
])

const recordColumns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.checkins.columns.user') },
  { key: 'checkin_date', label: t('admin.checkins.columns.checkinDate') },
  { key: 'streak_day', label: t('admin.checkins.columns.streakDay') },
  { key: 'reward_amount', label: t('admin.checkins.columns.reward') },
  { key: 'reward_detail', label: t('admin.checkins.columns.rewardDetail') },
  { key: 'balance_before', label: t('admin.checkins.columns.balanceBefore') },
  { key: 'balance_after', label: t('admin.checkins.columns.balanceAfter') },
  { key: 'created_at', label: t('admin.checkins.columns.createdAt') },
])

const blacklistColumns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.checkins.columns.user') },
  { key: 'reason', label: t('admin.checkins.columns.reason') },
  { key: 'created_at', label: t('admin.checkins.columns.createdAt') },
  { key: 'actions', label: t('admin.checkins.columns.actions') },
])

const rewardProbabilityTotal = computed(() => {
  return configForm.tiers.reduce((sum, tier) => sum + safeNumber(tier.probability), 0)
})

const rewardProbabilityTotalValid = computed(() => Math.abs(rewardProbabilityTotal.value - 100) < 0.0001)

const estimatedAverageReward = computed(() => {
  return configForm.tiers.reduce((sum, tier) => {
    return sum + safeNumber(tier.amount) * safeNumber(tier.probability) / 100
  }, 0)
})

function formatUsd(value: number): string {
  const amount = Number.isFinite(value) ? value : 0
  return `$${amount.toFixed(2)}`
}

function safeNumber(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function cloneTiers(tiers: CheckinRewardTier[] | undefined): CheckinRewardTier[] {
  return (tiers || []).map((tier, index) => ({
    amount: safeNumber(tier.amount),
    probability: safeNumber(tier.probability),
    sort_order: index + 1,
  }))
}

function cloneStreakRules(rules: CheckinStreakRule[] | undefined): CheckinStreakRule[] {
  return (rules || []).map((rule) => ({
    day: Math.floor(safeNumber(rule.day)),
    bonus_amount: safeNumber(rule.bonus_amount),
  }))
}

function userLabel(row: { user_email?: string; username?: string; user_id: number }): string {
  return row.user_email || row.username || t('admin.checkins.userId', { id: row.user_id })
}

async function loadStats() {
  statsLoading.value = true
  try {
    stats.value = await adminAPI.checkins.getStats()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadStats')))
  } finally {
    statsLoading.value = false
  }
}

async function loadConfig() {
  configLoading.value = true
  try {
    config.value = await adminAPI.checkins.getConfig()
    configForm.enabled = config.value.enabled
    configForm.min_total_usage_usd = config.value.min_total_usage_usd
    configForm.tiers = cloneTiers(config.value.tiers)
    configForm.streak_enabled = config.value.streak_enabled
    configForm.streak_rules = cloneStreakRules(config.value.streak_rules)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadConfig')))
  } finally {
    configLoading.value = false
  }
}

async function handleSaveConfig() {
  if (!Number.isFinite(configForm.min_total_usage_usd) || configForm.min_total_usage_usd < 0) {
    appStore.showError(t('admin.checkins.invalidMinTotalUsageUsd'))
    return
  }
  if (!validateRewardRules()) return
  configSaving.value = true
  try {
    config.value = await adminAPI.checkins.updateConfig({
      enabled: configForm.enabled,
      min_total_usage_usd: Number(configForm.min_total_usage_usd),
      tiers: configForm.tiers.map((tier, index) => ({
        amount: Number(tier.amount),
        probability: Number(tier.probability),
        sort_order: index + 1,
      })),
      streak_enabled: configForm.streak_enabled,
      streak_rules: configForm.streak_rules.map((rule) => ({
        day: Math.floor(Number(rule.day)),
        bonus_amount: Number(rule.bonus_amount),
      })),
      probability_total: rewardProbabilityTotal.value,
      preview: {
        min_reward: 0,
        max_reward: 0,
        average_reward: estimatedAverageReward.value,
      },
    })
    configForm.enabled = config.value.enabled
    configForm.min_total_usage_usd = config.value.min_total_usage_usd
    configForm.tiers = cloneTiers(config.value.tiers)
    configForm.streak_enabled = config.value.streak_enabled
    configForm.streak_rules = cloneStreakRules(config.value.streak_rules)
    appStore.showSuccess(t('admin.checkins.configSaved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToSaveConfig')))
  } finally {
    configSaving.value = false
  }
}

function validateRewardRules(): boolean {
  if (configForm.tiers.length === 0) {
    appStore.showError(t('admin.checkins.rewardTierRequired'))
    return false
  }
  if (!rewardProbabilityTotalValid.value) {
    appStore.showError(t('admin.checkins.invalidProbabilityTotal'))
    return false
  }
  const amounts = new Set<string>()
  for (const tier of configForm.tiers) {
    if (safeNumber(tier.amount) <= 0 || safeNumber(tier.probability) <= 0) {
      appStore.showError(t('admin.checkins.invalidRewardTier'))
      return false
    }
    const amountKey = safeNumber(tier.amount).toFixed(2)
    if (amounts.has(amountKey)) {
      appStore.showError(t('admin.checkins.duplicateRewardAmount'))
      return false
    }
    amounts.add(amountKey)
  }
  const days = new Set<number>()
  for (const rule of configForm.streak_rules) {
    const day = Math.floor(safeNumber(rule.day))
    if (day <= 0 || safeNumber(rule.bonus_amount) <= 0) {
      appStore.showError(t('admin.checkins.invalidStreakRule'))
      return false
    }
    if (days.has(day)) {
      appStore.showError(t('admin.checkins.duplicateStreakDay'))
      return false
    }
    days.add(day)
  }
  return true
}

function addRewardTier() {
  configForm.tiers.push({
    amount: 1,
    probability: 1,
    sort_order: configForm.tiers.length + 1,
  })
}

function removeRewardTier(index: number) {
  configForm.tiers.splice(index, 1)
  configForm.tiers.forEach((tier, idx) => {
    tier.sort_order = idx + 1
  })
}

function addStreakRule() {
  const maxDay = configForm.streak_rules.reduce((max, rule) => Math.max(max, safeNumber(rule.day)), 0)
  configForm.streak_rules.push({
    day: Math.max(1, Math.floor(maxDay) + 7),
    bonus_amount: 10,
  })
}

function removeStreakRule(index: number) {
  configForm.streak_rules.splice(index, 1)
}

async function loadRecords() {
  recordsLoading.value = true
  try {
    const response = await adminAPI.checkins.listRecords(
      recordsPagination.page,
      recordsPagination.page_size,
      {
        search: recordFilters.search.trim() || undefined,
        date: recordFilters.date || undefined,
      }
    )
    records.value = response.items
    recordsPagination.total = response.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadRecords')))
  } finally {
    recordsLoading.value = false
  }
}

async function loadBlacklist() {
  blacklistLoading.value = true
  try {
    const response = await adminAPI.checkins.listBlacklist(
      blacklistPagination.page,
      blacklistPagination.page_size,
      {
        active_only: true,
        search: blacklistFilters.search.trim() || undefined,
      }
    )
    blacklist.value = response.items
    blacklistPagination.total = response.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToLoadBlacklist')))
  } finally {
    blacklistLoading.value = false
  }
}

function reloadRecordsFromFirstPage() {
  recordsPagination.page = 1
  void loadRecords()
}

function handleRecordSearch() {
  clearTimeout(recordSearchTimeout)
  recordSearchTimeout = setTimeout(reloadRecordsFromFirstPage, 300)
}

function handleBlacklistSearch() {
  clearTimeout(blacklistSearchTimeout)
  blacklistSearchTimeout = setTimeout(() => {
    blacklistPagination.page = 1
    void loadBlacklist()
  }, 300)
}

function handleRecordsPageChange(page: number) {
  recordsPagination.page = page
  void loadRecords()
}

function handleRecordsPageSizeChange(pageSize: number) {
  recordsPagination.page_size = pageSize
  recordsPagination.page = 1
  void loadRecords()
}

function handleBlacklistPageChange(page: number) {
  blacklistPagination.page = page
  void loadBlacklist()
}

function handleBlacklistPageSizeChange(pageSize: number) {
  blacklistPagination.page_size = pageSize
  blacklistPagination.page = 1
  void loadBlacklist()
}

function refreshAll() {
  void Promise.all([loadConfig(), loadStats(), loadRecords(), loadBlacklist()])
}

async function searchBlacklistUsers() {
  const search = blacklistUserSearch.value.trim()
  if (!search) {
    appStore.showError(t('admin.checkins.userSearchRequired'))
    return
  }
  userSearchLoading.value = true
  try {
    const response = await adminAPI.users.list(1, 8, { search })
    userCandidates.value = response.items
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToSearchUsers')))
  } finally {
    userSearchLoading.value = false
  }
}

function selectBlacklistUser(user: AdminUser) {
  selectedBlacklistUser.value = user
}

async function handleAddBlacklist() {
  if (!selectedBlacklistUser.value) {
    appStore.showError(t('admin.checkins.selectUserRequired'))
    return
  }
  addingBlacklist.value = true
  try {
    await adminAPI.checkins.addBlacklist({
      user_id: selectedBlacklistUser.value.id,
      reason: blacklistReason.value.trim(),
    })
    appStore.showSuccess(t('admin.checkins.blacklistAdded'))
    selectedBlacklistUser.value = null
    blacklistUserSearch.value = ''
    blacklistReason.value = ''
    userCandidates.value = []
    await Promise.all([loadStats(), loadBlacklist()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToAddBlacklist')))
  } finally {
    addingBlacklist.value = false
  }
}

async function handleRemoveBlacklist(userId: number) {
  removingUserId.value = userId
  try {
    await adminAPI.checkins.removeBlacklist(userId)
    appStore.showSuccess(t('admin.checkins.blacklistRemoved'))
    await Promise.all([loadStats(), loadBlacklist()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.failedToRemoveBlacklist')))
  } finally {
    removingUserId.value = null
  }
}

onMounted(() => {
  void Promise.all([loadConfig(), loadStats(), loadRecords(), loadBlacklist()])
})

onUnmounted(() => {
  clearTimeout(recordSearchTimeout)
  clearTimeout(blacklistSearchTimeout)
})
</script>
