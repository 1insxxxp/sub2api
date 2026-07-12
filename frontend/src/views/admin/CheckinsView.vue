<template>
  <AppLayout>
    <div class="checkins-admin-page space-y-6">
      <div class="admin-toolbar-surface" data-test="checkins-action-toolbar">
        <div class="admin-toolbar">
          <div class="admin-toolbar-group flex-1">
            <span
              class="checkins-status-pill inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold"
              :class="configForm.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300'"
            >
              {{ configForm.enabled ? t('admin.checkins.enabledStatus') : t('admin.checkins.disabledStatus') }}
            </span>
            <span class="admin-page-meta-chip">
              <span>{{ t('admin.checkins.activeBlacklist') }}</span>
              <strong>{{ stats?.active_blacklist_count ?? 0 }}</strong>
            </span>
            <span class="admin-page-meta-chip">
              <span>{{ t('admin.checkins.rewardRules') }}</span>
              <strong>{{ t('admin.checkins.tierCount', { count: configForm.tiers.length }) }}</strong>
            </span>
          </div>

          <div class="admin-toolbar-group w-full justify-end lg:w-auto lg:flex-none">
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
              data-test="save-checkin-config"
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

      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="item in statsCards"
          :key="item.key"
          class="checkins-stat-card"
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

      <section class="grid gap-6 xl:grid-cols-[minmax(280px,360px),1fr]">
        <div class="admin-surface checkins-editor-card">
          <div class="admin-panel-header">
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
              class="admin-form-section flex cursor-pointer items-center justify-between gap-4 !space-y-0 px-4 py-3 transition-colors"
              :class="configForm.enabled ? 'border-primary-200 bg-primary-50/70 dark:border-primary-500/30 dark:bg-primary-900/10' : ''"
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

            <div>
              <label class="input-label">{{ t('admin.checkins.minTotalRechargeUsd') }}</label>
              <div class="relative">
                <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm font-semibold text-gray-400">$</span>
                <input
                  v-model.number="configForm.min_total_recharge_usd"
                  data-test="min-total-recharge-usd"
                  type="number"
                  min="0"
                  step="0.01"
                  class="input pl-7"
                />
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.checkins.minRechargeHint') }}
              </p>
            </div>

            <div class="checkins-summary-strip admin-form-section !space-y-0 px-4 py-3">
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

        <div class="admin-surface checkins-editor-card">
          <div class="admin-panel-header">
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

              <div class="checkins-grid-panel overflow-hidden rounded-lg border border-gray-100 dark:border-dark-700">
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
                  class="checkins-streak-card rounded-lg border border-amber-100 bg-amber-50/40 p-3 dark:border-amber-500/20 dark:bg-amber-900/10"
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
                <p v-if="configForm.streak_rules.length === 0" class="admin-form-section !space-y-0 p-4 text-sm text-gray-500 dark:text-dark-400">
                  {{ t('admin.checkins.emptyStreakRules') }}
                </p>
              </div>
            </section>
          </div>
        </div>
      </section>

      <section class="admin-surface checkins-table-card rounded-2xl">
        <div class="admin-panel-header">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.checkins.recordsTitle') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.checkins.recordsHint') }}
            </p>
          </div>
        </div>

        <div class="px-4 pb-4 pt-4 sm:px-5">
          <div class="admin-toolbar-surface">
            <div class="admin-toolbar">
              <div class="admin-toolbar-group flex-1">
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
              </div>
              <div class="admin-toolbar-group lg:ml-auto">
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
        <div class="admin-surface checkins-blacklist-form rounded-2xl p-4">
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
              class="admin-list-surface max-h-56 overflow-y-auto"
            >
              <button
                v-for="candidate in userCandidates"
                :key="candidate.id"
                type="button"
                :data-test="`select-blacklist-user-${candidate.id}`"
                class="admin-list-row flex w-full items-center justify-between gap-3 border-b border-gray-100 px-3 py-2 text-left last:border-b-0 dark:border-dark-700"
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
              class="checkins-selected-user admin-form-section !space-y-0 px-3 py-2 text-sm text-gray-700 dark:text-gray-200"
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

        <div class="admin-surface checkins-table-card rounded-2xl">
          <div class="admin-panel-header">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('admin.checkins.blacklistTitle') }}
              </h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.checkins.blacklistHint') }}
              </p>
            </div>
          </div>

          <div class="px-4 pb-4 pt-4 sm:px-5">
            <div class="admin-toolbar-surface">
              <div class="admin-toolbar">
                <div class="admin-toolbar-group flex-1">
                  <div class="min-w-56 flex-1 lg:w-72 lg:flex-none">
                    <input
                      v-model="blacklistFilters.search"
                      type="text"
                      class="input"
                      :placeholder="t('admin.checkins.blacklistSearchPlaceholder')"
                      @input="handleBlacklistSearch"
                    />
                  </div>
                </div>
                <div class="admin-toolbar-group lg:ml-auto">
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
  min_total_recharge_usd: 0,
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
    configForm.min_total_recharge_usd = config.value.min_total_recharge_usd
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
  if (!Number.isFinite(configForm.min_total_recharge_usd) || configForm.min_total_recharge_usd < 0) {
    appStore.showError(t('admin.checkins.invalidMinTotalRechargeUsd'))
    return
  }
  if (!validateRewardRules()) return
  configSaving.value = true
  try {
    config.value = await adminAPI.checkins.updateConfig({
      enabled: configForm.enabled,
      min_total_usage_usd: Number(configForm.min_total_usage_usd),
      min_total_recharge_usd: Number(configForm.min_total_recharge_usd),
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
    configForm.min_total_recharge_usd = config.value.min_total_recharge_usd
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

<style>
/* Check-ins admin surface polish */
.checkins-admin-page {
  position: relative;
}

.checkins-editor-card,
.checkins-table-card,
.checkins-blacklist-form {
  position: relative;
  overflow: hidden;
}

.checkins-editor-card::before,
.checkins-table-card::before,
.checkins-blacklist-form::before {
  content: "";
  position: absolute;
  inset: 0 0 auto 0;
  height: 1px;
  background: linear-gradient(90deg, rgba(37, 99, 235, 0.9), rgba(59, 130, 246, 0.45), rgba(34, 211, 238, 0.4));
  pointer-events: none;
}

.checkins-status-pill {
  box-shadow:
    0 8px 18px rgba(15, 23, 42, 0.08),
    0 1px 0 rgba(255, 255, 255, 0.72) inset;
}

.checkins-stat-card {
  position: relative;
  height: 100%;
  overflow: hidden;
  border-radius: 1rem;
  border: 1px solid rgba(191, 219, 254, 0.42);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.9)),
    rgba(255, 255, 255, 0.94);
  padding: 1rem;
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.82) inset,
    0 18px 38px rgba(15, 23, 42, 0.055);
}

.checkins-stat-card::after {
  content: "";
  position: absolute;
  inset: 0 auto auto 0;
  width: 4.75rem;
  height: 4.75rem;
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(59, 130, 246, 0.15), transparent 70%);
  transform: translate(-24%, -24%);
  pointer-events: none;
}

.checkins-summary-strip {
  border: 1px solid rgba(226, 232, 240, 0.84);
  background:
    linear-gradient(135deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.92)),
    rgba(248, 250, 252, 0.94);
}

.checkins-grid-panel {
  border-color: rgba(203, 213, 225, 0.8);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.74) inset;
}

.checkins-streak-card {
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.7) inset,
    0 12px 28px rgba(245, 158, 11, 0.08);
}

.checkins-selected-user {
  border: 1px solid rgba(147, 197, 253, 0.42);
  background:
    linear-gradient(135deg, rgba(239, 246, 255, 0.94), rgba(248, 250, 252, 0.92)),
    rgba(239, 246, 255, 0.86);
}

.dark .checkins-status-pill {
  box-shadow:
    0 10px 24px rgba(0, 0, 0, 0.2),
    0 1px 0 rgba(255, 255, 255, 0.05) inset;
}

.dark .checkins-stat-card {
  border-color: rgba(96, 165, 250, 0.18);
  background:
    linear-gradient(135deg, rgba(15, 23, 42, 0.96), rgba(15, 23, 42, 0.92)),
    rgba(15, 23, 42, 0.94);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.05) inset,
    0 22px 44px rgba(0, 0, 0, 0.22);
}

.dark .checkins-stat-card::after {
  background: radial-gradient(circle, rgba(56, 189, 248, 0.13), transparent 70%);
}

.dark .checkins-summary-strip {
  border-color: rgba(51, 65, 85, 0.92);
  background:
    linear-gradient(135deg, rgba(30, 41, 59, 0.94), rgba(15, 23, 42, 0.92)),
    rgba(15, 23, 42, 0.88);
}

.dark .checkins-grid-panel {
  border-color: rgba(51, 65, 85, 0.92);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

.dark .checkins-streak-card {
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.04) inset,
    0 18px 36px rgba(0, 0, 0, 0.18);
}

.dark .checkins-selected-user {
  border-color: rgba(96, 165, 250, 0.2);
  background:
    linear-gradient(135deg, rgba(30, 41, 59, 0.9), rgba(15, 23, 42, 0.88)),
    rgba(15, 23, 42, 0.86);
}
</style>
