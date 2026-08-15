<template>
  <section class="admin-surface overflow-hidden rounded-2xl" data-test="campaign-panel">
    <div class="admin-panel-header">
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div class="flex items-start gap-3">
          <div class="flex h-10 w-10 flex-none items-center justify-center rounded-xl bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-300">
            <Icon name="calendar" size="sm" />
          </div>
          <div>
            <h3 class="text-base font-semibold text-slate-900 dark:text-white">
              {{ t('admin.checkins.campaigns.title') }}
            </h3>
            <p class="mt-1 text-sm text-slate-500 dark:text-dark-400">
              {{ t('admin.checkins.campaigns.description') }}
            </p>
            <p data-test="campaign-beijing-calendar" class="mt-2 text-xs font-medium text-blue-600 dark:text-blue-300">
              {{ t('admin.checkins.campaigns.beijingCalendar') }}
            </p>
          </div>
        </div>
        <button
          type="button"
          data-test="campaign-create"
          class="btn btn-primary inline-flex items-center gap-2"
          :disabled="mutationBusy"
          @click="openCreate"
        >
          <Icon name="plus" size="sm" />
          {{ t('admin.checkins.campaigns.actions.create') }}
        </button>
      </div>
    </div>

    <div class="border-b border-slate-200 px-4 py-3 dark:border-dark-700 sm:px-5">
      <div class="flex flex-wrap items-center gap-2" role="group" :aria-label="t('admin.checkins.campaigns.filters.label')">
        <button
          v-for="filter in lifecycleFilters"
          :key="filter.value"
          type="button"
          :data-test="`campaign-filter-${filter.value}`"
          class="rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30"
          :class="selectedLifecycle === filter.value ? 'border-primary-200 bg-primary-50 text-primary-700 dark:border-primary-500/40 dark:bg-primary-500/10 dark:text-primary-200' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300 hover:bg-slate-50 dark:border-dark-600 dark:bg-dark-800 dark:text-slate-300 dark:hover:bg-dark-700'"
          :aria-pressed="selectedLifecycle === filter.value"
          :disabled="mutationBusy"
          @click="selectLifecycle(filter.value)"
        >
          {{ t(filter.labelKey) }}
        </button>
        <button
          type="button"
          data-test="campaign-refresh"
          class="ml-auto btn btn-secondary btn-sm"
          :disabled="loading || mutationBusy"
          :title="t('common.refresh')"
          @click="loadCampaigns"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>
    </div>

    <div
      v-if="overlapConflict"
      data-test="campaign-panel-overlap-conflict"
      role="alert"
      class="m-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200 sm:m-5"
    >
      {{
        t('admin.checkins.campaigns.overlapDetail', {
          name: overlapConflict.name,
          start: overlapConflict.startDate,
          end: overlapConflict.endDate,
        })
      }}
    </div>

    <div v-if="loading && campaigns.length === 0" class="px-5 py-16 text-center text-sm text-slate-500 dark:text-dark-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="campaigns.length === 0" data-test="campaign-empty" class="px-5 py-16 text-center">
      <p class="text-sm font-medium text-slate-700 dark:text-slate-200">
        {{ t('admin.checkins.campaigns.empty') }}
      </p>
      <p class="mt-1 text-xs text-slate-500 dark:text-dark-400">
        {{ t('admin.checkins.campaigns.emptyHint') }}
      </p>
    </div>
    <div v-else class="grid gap-4 p-4 sm:p-5 lg:grid-cols-2 2xl:grid-cols-3">
      <article
        v-for="campaign in campaigns"
        :key="campaign.id"
        :data-test="`campaign-card-${campaign.id}`"
        class="campaign-card flex min-h-64 flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex items-start justify-between gap-3 border-b border-slate-100 px-4 py-4 dark:border-dark-700">
          <div class="min-w-0">
            <h4 class="truncate text-sm font-semibold text-slate-900 dark:text-white">
              {{ campaign.name }}
            </h4>
            <p :data-test="`campaign-date-range-${campaign.id}`" class="mt-1.5 text-xs font-medium tabular-nums text-slate-500 dark:text-dark-400">
              {{ t('admin.checkins.campaigns.dateRange', { start: campaign.start_date, end: campaign.end_date }) }}
            </p>
          </div>
          <span
            :data-test="`campaign-status-${campaign.lifecycle_status}`"
            class="inline-flex flex-none items-center rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="lifecycleClass(campaign.lifecycle_status)"
          >
            {{ t(`admin.checkins.campaigns.lifecycle.${campaign.lifecycle_status}`) }}
          </span>
        </div>

        <div class="grid grid-cols-3 gap-px bg-slate-100 dark:bg-dark-700">
          <div class="bg-slate-50 px-3 py-3 dark:bg-dark-900/40">
            <span class="block text-[11px] text-slate-500 dark:text-dark-400">{{ t('admin.checkins.campaigns.preview.minimum') }}</span>
            <strong class="mt-1 block text-sm tabular-nums text-slate-900 dark:text-white">{{ formatUsd(campaign.preview.min_reward) }}</strong>
          </div>
          <div class="bg-slate-50 px-3 py-3 dark:bg-dark-900/40">
            <span class="block text-[11px] text-slate-500 dark:text-dark-400">{{ t('admin.checkins.campaigns.preview.maximum') }}</span>
            <strong class="mt-1 block text-sm tabular-nums text-slate-900 dark:text-white">{{ formatUsd(campaign.preview.max_reward) }}</strong>
          </div>
          <div class="bg-blue-50 px-3 py-3 dark:bg-blue-950/20">
            <span class="block text-[11px] text-blue-600 dark:text-blue-300">{{ t('admin.checkins.campaigns.preview.average') }}</span>
            <strong class="mt-1 block text-sm tabular-nums text-blue-700 dark:text-blue-200">{{ formatUsd(campaign.preview.average_reward) }}</strong>
          </div>
        </div>

        <div class="flex flex-1 flex-col justify-between gap-4 px-4 py-4">
          <div class="flex items-center justify-between text-xs text-slate-500 dark:text-dark-400">
            <span>{{ t('admin.checkins.campaigns.tierCount', { count: campaign.reward_tiers.length }) }}</span>
            <span class="tabular-nums">{{ t('admin.checkins.campaigns.probabilityTotal', { total: campaign.probability_total.toFixed(2) }) }}</span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              v-if="campaign.lifecycle_status === 'draft'"
              type="button"
              :data-test="`campaign-edit-${campaign.id}`"
              class="btn btn-secondary btn-sm"
              :disabled="mutationBusy"
              @click="openDialog('edit', campaign.id)"
            >
              {{ t('common.edit') }}
            </button>
            <button
              v-else
              type="button"
              :data-test="`campaign-view-${campaign.id}`"
              class="btn btn-secondary btn-sm"
              :disabled="mutationBusy"
              @click="openDialog('view', campaign.id)"
            >
              {{ t('common.view') }}
            </button>
            <button
              type="button"
              :data-test="`campaign-copy-${campaign.id}`"
              class="btn btn-secondary btn-sm"
              :disabled="mutationBusy"
              @click="openDialog('copy', campaign.id)"
            >
              {{ t('admin.checkins.campaigns.actions.copy') }}
            </button>
            <button
              v-if="canEnable(campaign)"
              type="button"
              :data-test="`campaign-enable-${campaign.id}`"
              class="btn btn-primary btn-sm"
              :disabled="mutationBusy"
              @click="requestMutation('enable', campaign)"
            >
              {{ t('admin.checkins.campaigns.actions.enable') }}
            </button>
            <button
              v-if="canDisable(campaign)"
              type="button"
              :data-test="`campaign-disable-${campaign.id}`"
              class="btn btn-secondary btn-sm text-amber-700 dark:text-amber-200"
              :disabled="mutationBusy"
              @click="requestMutation('disable', campaign)"
            >
              {{ t('admin.checkins.campaigns.actions.disable') }}
            </button>
            <button
              v-if="campaign.lifecycle_status === 'draft'"
              type="button"
              :data-test="`campaign-delete-${campaign.id}`"
              class="btn btn-secondary btn-sm ml-auto text-red-600 dark:text-red-300"
              :disabled="mutationBusy"
              @click="requestMutation('delete', campaign)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </article>
    </div>
  </section>

  <CheckinRewardCampaignDialog
    :show="dialogOpen"
    :mode="dialogMode"
    :campaign-id="dialogCampaignId"
    :default-tiers="defaultTiers"
    @close="closeDialog"
    @saved="handleDialogSaved"
  />

  <ConfirmDialog
    :show="confirmTarget !== null"
    :title="confirmTitle"
    :message="confirmMessage"
    :danger="confirmTarget?.action === 'delete'"
    @confirm="performConfirmedMutation"
    @cancel="confirmTarget = null"
  />
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  AdminCheckinRewardCampaign,
  CheckinRewardCampaignLifecycle,
  CheckinRewardTier,
} from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage, extractApiErrorMetadata } from '@/utils/apiError'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import CheckinRewardCampaignDialog from './CheckinRewardCampaignDialog.vue'

type LifecycleFilter = CheckinRewardCampaignLifecycle | 'all'
type DialogMode = 'create' | 'edit' | 'view' | 'copy'
type MutationAction = 'enable' | 'disable' | 'delete'

interface LifecycleFilterOption {
  value: LifecycleFilter
  labelKey: string
}

interface ConfirmTarget {
  action: MutationAction
  campaign: AdminCheckinRewardCampaign
}

interface OverlapConflict {
  name: string
  startDate: string
  endDate: string
}

interface Props {
  defaultTiers: CheckinRewardTier[]
}

defineProps<Props>()
const { t } = useI18n()
const appStore = useAppStore()

const lifecycleFilters: LifecycleFilterOption[] = [
  { value: 'all', labelKey: 'admin.checkins.campaigns.filters.all' },
  { value: 'active', labelKey: 'admin.checkins.campaigns.lifecycle.active' },
  { value: 'upcoming', labelKey: 'admin.checkins.campaigns.lifecycle.upcoming' },
  { value: 'draft', labelKey: 'admin.checkins.campaigns.lifecycle.draft' },
  { value: 'ended', labelKey: 'admin.checkins.campaigns.lifecycle.ended' },
  { value: 'disabled', labelKey: 'admin.checkins.campaigns.lifecycle.disabled' },
]

const campaigns = ref<AdminCheckinRewardCampaign[]>([])
const selectedLifecycle = ref<LifecycleFilter>('all')
const loading = ref(false)
const mutationState = ref<{ action: MutationAction; campaignId: number } | null>(null)
const confirmTarget = ref<ConfirmTarget | null>(null)
const overlapConflict = ref<OverlapConflict | null>(null)
const dialogOpen = ref(false)
const dialogMode = ref<DialogMode>('create')
const dialogCampaignId = ref<number>()
let listGeneration = 0
let listController: AbortController | null = null

const mutationBusy = computed(() => mutationState.value !== null)
const confirmTitle = computed(() => {
  if (!confirmTarget.value) return ''
  return t(`admin.checkins.campaigns.confirm.${confirmTarget.value.action}Title`)
})
const confirmMessage = computed(() => {
  if (!confirmTarget.value) return ''
  return t(`admin.checkins.campaigns.confirm.${confirmTarget.value.action}Message`, {
    name: confirmTarget.value.campaign.name,
  })
})

function selectLifecycle(lifecycle: LifecycleFilter): void {
  if (selectedLifecycle.value === lifecycle || mutationBusy.value) return
  selectedLifecycle.value = lifecycle
  void loadCampaigns()
}

async function loadCampaigns(): Promise<void> {
  const generation = ++listGeneration
  listController?.abort()
  const controller = new AbortController()
  listController = controller
  loading.value = true
  try {
    const result = await adminAPI.checkins.listCampaigns(selectedLifecycle.value, { signal: controller.signal })
    if (generation !== listGeneration || controller.signal.aborted) return
    campaigns.value = result
  } catch (error: unknown) {
    if (generation !== listGeneration || controller.signal.aborted || isAbortError(error)) return
    appStore.showError(extractApiErrorMessage(error, t('admin.checkins.campaigns.errors.load')))
  } finally {
    if (generation === listGeneration) loading.value = false
  }
}

function isAbortError(error: unknown): boolean {
  if (error instanceof DOMException) return error.name === 'AbortError'
  if (!error || typeof error !== 'object' || !('name' in error)) return false
  return error.name === 'AbortError' || error.name === 'CanceledError'
}

function openCreate(): void {
  if (mutationBusy.value) return
  dialogMode.value = 'create'
  dialogCampaignId.value = undefined
  dialogOpen.value = true
}

function openDialog(mode: DialogMode, campaignId: number): void {
  if (mutationBusy.value) return
  dialogMode.value = mode
  dialogCampaignId.value = campaignId
  dialogOpen.value = true
}

function closeDialog(): void {
  dialogOpen.value = false
  dialogCampaignId.value = undefined
}

function handleDialogSaved(): void {
  closeDialog()
  appStore.showSuccess(t('admin.checkins.campaigns.success.saved'))
  void loadCampaigns()
}

function requestMutation(action: MutationAction, campaign: AdminCheckinRewardCampaign): void {
  if (mutationBusy.value) return
  overlapConflict.value = null
  confirmTarget.value = { action, campaign }
}

async function performConfirmedMutation(): Promise<void> {
  const target = confirmTarget.value
  if (!target || mutationBusy.value) return
  confirmTarget.value = null
  mutationState.value = { action: target.action, campaignId: target.campaign.id }
  overlapConflict.value = null
  try {
    if (target.action === 'enable') {
      await adminAPI.checkins.enableCampaign(target.campaign.id)
    } else if (target.action === 'disable') {
      await adminAPI.checkins.disableCampaign(target.campaign.id)
    } else {
      await adminAPI.checkins.deleteCampaign(target.campaign.id)
    }
    appStore.showSuccess(t(`admin.checkins.campaigns.success.${target.action}`))
    await loadCampaigns()
  } catch (error: unknown) {
    const message = extractApiErrorMessage(error, t(`admin.checkins.campaigns.errors.${target.action}`))
    appStore.showError(message)
    overlapConflict.value = parseOverlapConflict(error)
  } finally {
    mutationState.value = null
  }
}

function parseOverlapConflict(error: unknown): OverlapConflict | null {
  const metadata = extractApiErrorMetadata(error)
  if (!metadata) return null
  const name = metadata.conflict_campaign_name
  const startDate = metadata.conflict_start_date
  const endDate = metadata.conflict_end_date
  if (typeof name !== 'string' || typeof startDate !== 'string' || typeof endDate !== 'string') return null
  return { name, startDate, endDate }
}

function canEnable(campaign: AdminCheckinRewardCampaign): boolean {
  if (campaign.lifecycle_status === 'draft') return true
  return campaign.lifecycle_status === 'disabled' && campaign.start_date > beijingToday()
}

function canDisable(campaign: AdminCheckinRewardCampaign): boolean {
  return campaign.lifecycle_status === 'active' || campaign.lifecycle_status === 'upcoming'
}

function beijingToday(): string {
  const parts = new Intl.DateTimeFormat('en', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(new Date())
  const year = parts.find((part) => part.type === 'year')?.value ?? ''
  const month = parts.find((part) => part.type === 'month')?.value ?? ''
  const day = parts.find((part) => part.type === 'day')?.value ?? ''
  return `${year}-${month}-${day}`
}

function lifecycleClass(lifecycle: CheckinRewardCampaignLifecycle): string {
  const classes: Record<CheckinRewardCampaignLifecycle, string> = {
    active: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
    upcoming: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-200',
    ended: 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-dark-300',
    draft: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200',
    disabled: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200',
  }
  return classes[lifecycle]
}

function formatUsd(value: number): string {
  return `$${Number.isFinite(value) ? value.toFixed(2) : '0.00'}`
}

onMounted(() => {
  void loadCampaigns()
})

onUnmounted(() => {
  listGeneration += 1
  listController?.abort()
})
</script>

<style scoped>
.campaign-card {
  box-shadow: 0 14px 32px rgb(15 23 42 / 0.045);
  transition: border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}

.campaign-card:hover {
  border-color: rgb(147 197 253 / 0.85);
  box-shadow: 0 18px 40px rgb(15 23 42 / 0.075);
  transform: translateY(-1px);
}

.dark .campaign-card {
  box-shadow: 0 16px 36px rgb(0 0 0 / 0.16);
}
</style>
