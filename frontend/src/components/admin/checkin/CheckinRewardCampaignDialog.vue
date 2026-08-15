<template>
  <BaseDialog
    :show="show"
    :title="dialogTitle"
    width="wide"
    :close-on-escape="!isMutating"
    :show-close-button="!isMutating"
    @close="requestClose"
  >
    <div data-test="campaign-dialog" class="space-y-5">
      <div
        v-if="apiErrorMessage"
        data-test="campaign-api-error"
        role="alert"
        aria-live="assertive"
        class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200"
      >
        {{ apiErrorMessage }}
      </div>

      <div
        v-if="overlapConflict"
        data-test="campaign-overlap-conflict"
        class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
      >
        {{
          t('admin.checkins.campaigns.overlapDetail', {
            name: overlapConflict.name,
            start: overlapConflict.startDate,
            end: overlapConflict.endDate,
          })
        }}
      </div>

      <div v-if="detailLoading" data-test="campaign-detail-loading" class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
        {{ t('common.loading') }}
      </div>

      <template v-else>
        <div
          v-if="isReadOnly"
          data-test="campaign-read-only-note"
          class="flex items-start gap-3 rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-sm text-blue-700 dark:border-blue-900/50 dark:bg-blue-950/20 dark:text-blue-200"
        >
          <Icon name="infoCircle" size="sm" class="mt-0.5 flex-none" />
          <span>{{ t('admin.checkins.campaigns.readOnly') }}</span>
        </div>

        <section class="grid gap-4 md:grid-cols-2">
          <label class="block md:col-span-2" for="checkin-campaign-name">
            <span class="input-label">{{ t('admin.checkins.campaigns.fields.name') }}</span>
            <input
              id="checkin-campaign-name"
              v-model="draft.name"
              data-test="campaign-name"
              type="text"
              maxlength="120"
              class="input"
              :placeholder="t('admin.checkins.campaigns.fields.namePlaceholder')"
              :disabled="!canEditName || isMutating"
            />
          </label>

          <label class="block" for="checkin-campaign-start-date">
            <span class="input-label">{{ t('admin.checkins.campaigns.fields.startDate') }}</span>
            <input
              id="checkin-campaign-start-date"
              v-model="draft.start_date"
              data-test="campaign-start-date"
              type="date"
              class="input"
              :disabled="!canEditRules || isMutating"
            />
          </label>

          <label class="block" for="checkin-campaign-end-date">
            <span class="input-label">{{ t('admin.checkins.campaigns.fields.endDate') }}</span>
            <input
              id="checkin-campaign-end-date"
              v-model="draft.end_date"
              data-test="campaign-end-date"
              type="date"
              class="input"
              :disabled="!canEditRules || isMutating"
            />
          </label>
        </section>

        <section class="overflow-hidden rounded-2xl border border-slate-200 bg-slate-50/60 dark:border-dark-700 dark:bg-dark-900/30">
          <div class="flex flex-col gap-3 border-b border-slate-200 bg-white px-4 py-4 dark:border-dark-700 dark:bg-dark-800 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h4 class="text-sm font-semibold text-slate-900 dark:text-white">
                {{ t('admin.checkins.campaigns.rewardTiers') }}
              </h4>
              <p class="mt-1 text-xs text-slate-500 dark:text-dark-400">
                {{ t('admin.checkins.campaigns.rewardTiersHint') }}
              </p>
            </div>
            <div class="flex items-center gap-3">
              <span
                data-test="campaign-probability-total"
                class="rounded-full px-3 py-1 text-xs font-semibold tabular-nums"
                :class="probabilityTotalValid ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200' : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-200'"
              >
                {{ t('admin.checkins.campaigns.probabilityTotal', { total: probabilityTotal.toFixed(2) }) }}
              </span>
              <button
                v-if="canEditRules"
                type="button"
                data-test="campaign-add-tier"
                class="btn btn-secondary btn-sm"
                :disabled="isMutating || draft.reward_tiers.length >= maximumTiers"
                @click="addTier"
              >
                <Icon name="plus" size="sm" />
                {{ t('admin.checkins.campaigns.actions.addTier') }}
              </button>
            </div>
          </div>

          <div class="divide-y divide-slate-200 dark:divide-dark-700">
            <div
              v-for="(tier, index) in draft.reward_tiers"
              :key="`campaign-tier-${index}`"
              :data-test="`campaign-tier-row-${index}`"
              class="grid gap-3 bg-white px-4 py-3 dark:bg-dark-800 sm:grid-cols-[2rem_minmax(0,1fr)_minmax(0,1fr)_2.5rem] sm:items-end"
            >
              <div class="hidden h-10 items-center justify-center text-xs font-semibold text-slate-400 sm:flex">
                {{ index + 1 }}
              </div>
              <label class="block">
                <span class="input-label">{{ t('admin.checkins.campaigns.fields.amount') }}</span>
                <div class="relative">
                  <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-sm font-semibold text-slate-400">$</span>
                  <input
                    v-model.number="tier.amount"
                    :data-test="`campaign-tier-amount-${index}`"
                    type="number"
                    min="0.01"
                    step="0.01"
                    class="input pl-7 tabular-nums"
                    :disabled="!canEditRules || isMutating"
                  />
                </div>
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.checkins.campaigns.fields.probability') }}</span>
                <div class="relative">
                  <input
                    v-model.number="tier.probability"
                    :data-test="`campaign-tier-probability-${index}`"
                    type="number"
                    min="0.01"
                    max="100"
                    step="0.01"
                    class="input pr-8 tabular-nums"
                    :disabled="!canEditRules || isMutating"
                  />
                  <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm font-semibold text-slate-400">%</span>
                </div>
              </label>
              <button
                v-if="canEditRules"
                type="button"
                :data-test="`campaign-remove-tier-${index}`"
                class="btn btn-secondary h-10 px-2"
                :title="t('admin.checkins.campaigns.actions.removeTier')"
                :aria-label="t('admin.checkins.campaigns.actions.removeTier')"
                :disabled="isMutating || draft.reward_tiers.length <= 1"
                @click="removeTier(index)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>
        </section>

        <section class="grid gap-3 sm:grid-cols-3" aria-live="polite">
          <div class="campaign-preview-tile">
            <span>{{ t('admin.checkins.campaigns.preview.minimum') }}</span>
            <strong data-test="campaign-preview-min">{{ formatUsd(previewMinimum) }}</strong>
          </div>
          <div class="campaign-preview-tile">
            <span>{{ t('admin.checkins.campaigns.preview.maximum') }}</span>
            <strong data-test="campaign-preview-max">{{ formatUsd(previewMaximum) }}</strong>
          </div>
          <div class="campaign-preview-tile campaign-preview-tile--accent">
            <span>{{ t('admin.checkins.campaigns.preview.average') }}</span>
            <strong data-test="campaign-preview-average">{{ formatUsd(previewAverage) }}</strong>
          </div>
        </section>

        <p
          v-if="validationMessage"
          data-test="campaign-validation-error"
          role="alert"
          class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-200"
        >
          {{ validationMessage }}
        </p>
      </template>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button
          type="button"
          data-test="campaign-cancel"
          class="btn btn-secondary"
          :disabled="isMutating"
          @click="requestClose"
        >
          {{ isReadOnly ? t('common.close') : t('common.cancel') }}
        </button>
        <button
          v-if="showSubmit"
          type="button"
          data-test="campaign-submit"
          class="btn btn-primary"
          :disabled="isMutating || detailLoading"
          @click="submit"
        >
          {{ isMutating ? t('common.saving') : submitLabel }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminCheckinRewardCampaign, CheckinRewardTier } from '@/api/admin'
import { extractApiErrorMessage, extractApiErrorMetadata } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

type DialogMode = 'create' | 'edit' | 'view' | 'copy'

interface CampaignDraft {
  name: string
  start_date: string
  end_date: string
  reward_tiers: CheckinRewardTier[]
}

interface OverlapConflict {
  name: string
  startDate: string
  endDate: string
}

interface Props {
  show: boolean
  mode: DialogMode
  campaignId?: number
  defaultTiers: CheckinRewardTier[]
}

interface Emits {
  (event: 'close'): void
  (event: 'saved', campaign: AdminCheckinRewardCampaign): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()
const maximumTiers = 20

const draft = reactive<CampaignDraft>({
  name: '',
  start_date: '',
  end_date: '',
  reward_tiers: [],
})
const loadedCampaign = ref<AdminCheckinRewardCampaign | null>(null)
const detailLoading = ref(false)
const mutationState = ref<'idle' | 'saving'>('idle')
const validationMessage = ref('')
const apiErrorMessage = ref('')
const overlapConflict = ref<OverlapConflict | null>(null)
let detailGeneration = 0
let detailController: AbortController | null = null

const isMutating = computed(() => mutationState.value !== 'idle')
const canEditRules = computed(() => {
  if (props.mode === 'create') return true
  return props.mode === 'edit' && loadedCampaign.value?.lifecycle_status === 'draft'
})
const canEditName = computed(() => canEditRules.value || props.mode === 'copy')
const isReadOnly = computed(() => !canEditRules.value && props.mode !== 'copy')
const showSubmit = computed(() => {
  return props.mode === 'create' || props.mode === 'copy' || (props.mode === 'edit' && canEditRules.value)
})
const dialogTitle = computed(() => t(`admin.checkins.campaigns.dialog.${props.mode}Title`))
const submitLabel = computed(() => {
  if (props.mode === 'copy') return t('admin.checkins.campaigns.actions.createCopy')
  return props.mode === 'create' ? t('common.create') : t('common.save')
})
const probabilityTotal = computed(() => draft.reward_tiers.reduce((total, tier) => total + safeNumber(tier.probability), 0))
const probabilityTotalValid = computed(() => probabilityBasisPoints() === 10000)
const previewMinimum = computed(() => {
  if (draft.reward_tiers.length === 0) return 0
  return Math.min(...draft.reward_tiers.map((tier) => safeNumber(tier.amount)))
})
const previewMaximum = computed(() => {
  if (draft.reward_tiers.length === 0) return 0
  return Math.max(...draft.reward_tiers.map((tier) => safeNumber(tier.amount)))
})
const previewAverage = computed(() => draft.reward_tiers.reduce((total, tier) => {
  return total + safeNumber(tier.amount) * safeNumber(tier.probability) / 100
}, 0))

function safeNumber(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function cloneTiers(tiers: CheckinRewardTier[]): CheckinRewardTier[] {
  return tiers.map((tier, index) => ({
    amount: safeNumber(tier.amount),
    probability: safeNumber(tier.probability),
    sort_order: index + 1,
  }))
}

function assignDraft(source?: AdminCheckinRewardCampaign): void {
  draft.name = source?.name ?? ''
  draft.start_date = source?.start_date ?? ''
  draft.end_date = source?.end_date ?? ''
  draft.reward_tiers = cloneTiers(source?.reward_tiers ?? props.defaultTiers)
  if (props.mode === 'copy' && source) {
    draft.name = t('admin.checkins.campaigns.copyName', { name: source.name })
  }
}

function resetMessages(): void {
  validationMessage.value = ''
  apiErrorMessage.value = ''
  overlapConflict.value = null
}

function openDialog(): void {
  detailGeneration += 1
  detailController?.abort()
  detailController = null
  resetMessages()
  loadedCampaign.value = null
  detailLoading.value = false
  if (!props.show) return
  if (props.mode === 'create') {
    assignDraft()
    return
  }
  if (!props.campaignId) {
    apiErrorMessage.value = t('admin.checkins.campaigns.errors.invalidCampaign')
    return
  }
  const generation = detailGeneration
  const controller = new AbortController()
  detailController = controller
  detailLoading.value = true
  void adminAPI.checkins.getCampaign(props.campaignId, { signal: controller.signal })
    .then((campaign) => {
      if (generation !== detailGeneration || controller.signal.aborted) return
      loadedCampaign.value = campaign
      assignDraft(campaign)
    })
    .catch((error: unknown) => {
      if (generation !== detailGeneration || controller.signal.aborted || isAbortError(error)) return
      apiErrorMessage.value = extractApiErrorMessage(error, t('admin.checkins.campaigns.errors.load'))
    })
    .finally(() => {
      if (generation === detailGeneration) detailLoading.value = false
    })
}

function isAbortError(error: unknown): boolean {
  if (error instanceof DOMException) return error.name === 'AbortError'
  if (!error || typeof error !== 'object' || !('name' in error)) return false
  return error.name === 'AbortError' || error.name === 'CanceledError'
}

function addTier(): void {
  if (!canEditRules.value || draft.reward_tiers.length >= maximumTiers) return
  const nextAmount = draft.reward_tiers.reduce((maximum, tier) => Math.max(maximum, safeNumber(tier.amount)), 0) + 1
  draft.reward_tiers.push({ amount: nextAmount, probability: 1, sort_order: draft.reward_tiers.length + 1 })
}

function removeTier(index: number): void {
  if (!canEditRules.value || draft.reward_tiers.length <= 1) return
  draft.reward_tiers.splice(index, 1)
  draft.reward_tiers.forEach((tier, tierIndex) => {
    tier.sort_order = tierIndex + 1
  })
}

function hasAtMostTwoDecimals(value: number): boolean {
  if (!Number.isFinite(value)) return false
  const scaled = value * 100
  const rounded = Math.round(scaled)
  return Number.isFinite(scaled) && Math.abs(scaled - rounded) <= 1e-9
}

function probabilityBasisPoints(): number {
  return draft.reward_tiers.reduce((total, tier) => total + Math.round(safeNumber(tier.probability) * 100), 0)
}

function validateDraft(): boolean {
  validationMessage.value = ''
  if (!draft.name.trim()) {
    validationMessage.value = t('admin.checkins.campaigns.validation.nameRequired')
    return false
  }
  if (!draft.start_date || !draft.end_date) {
    validationMessage.value = t('admin.checkins.campaigns.validation.datesRequired')
    return false
  }
  if (draft.start_date > draft.end_date) {
    validationMessage.value = t('admin.checkins.campaigns.validation.dateRange')
    return false
  }
  if (draft.reward_tiers.length === 0) {
    validationMessage.value = t('admin.checkins.campaigns.validation.tierRequired')
    return false
  }
  if (draft.reward_tiers.length > maximumTiers) {
    validationMessage.value = t('admin.checkins.campaigns.validation.maximumTiers', { count: maximumTiers })
    return false
  }
  const amounts = new Set<number>()
  for (const tier of draft.reward_tiers) {
    const amount = safeNumber(tier.amount)
    const probability = safeNumber(tier.probability)
    if (amount <= 0 || probability <= 0) {
      validationMessage.value = t('admin.checkins.campaigns.validation.positiveValues')
      return false
    }
    if (!hasAtMostTwoDecimals(amount) || !hasAtMostTwoDecimals(probability)) {
      validationMessage.value = t('admin.checkins.campaigns.validation.twoDecimals')
      return false
    }
    const amountCents = Math.round(amount * 100)
    if (amounts.has(amountCents)) {
      validationMessage.value = t('admin.checkins.campaigns.validation.uniqueAmounts')
      return false
    }
    amounts.add(amountCents)
  }
  if (!probabilityTotalValid.value) {
    validationMessage.value = t('admin.checkins.campaigns.validation.probabilityTotal')
    return false
  }
  return true
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

async function submit(): Promise<void> {
  if (isMutating.value || detailLoading.value) return
  resetMessages()
  if (props.mode !== 'copy' && !validateDraft()) return
  if (props.mode === 'copy' && !draft.name.trim()) {
    validationMessage.value = t('admin.checkins.campaigns.validation.nameRequired')
    return
  }
  mutationState.value = 'saving'
  try {
    let saved: AdminCheckinRewardCampaign
    if (props.mode === 'create') {
      saved = await adminAPI.checkins.createCampaign({
        name: draft.name.trim(),
        start_date: draft.start_date,
        end_date: draft.end_date,
        reward_tiers: cloneTiers(draft.reward_tiers),
      })
    } else if (props.mode === 'edit' && props.campaignId) {
      saved = await adminAPI.checkins.updateCampaign(props.campaignId, {
        name: draft.name.trim(),
        start_date: draft.start_date,
        end_date: draft.end_date,
        reward_tiers: cloneTiers(draft.reward_tiers),
      })
    } else if (props.mode === 'copy' && props.campaignId) {
      saved = await adminAPI.checkins.copyCampaign(props.campaignId, { name: draft.name.trim() })
    } else {
      apiErrorMessage.value = t('admin.checkins.campaigns.errors.invalidCampaign')
      return
    }
    emit('saved', saved)
  } catch (error: unknown) {
    apiErrorMessage.value = extractApiErrorMessage(error, t('admin.checkins.campaigns.errors.save'))
    overlapConflict.value = parseOverlapConflict(error)
  } finally {
    mutationState.value = 'idle'
  }
}

function requestClose(): void {
  if (isMutating.value) return
  emit('close')
}

function formatUsd(value: number): string {
  return `$${safeNumber(value).toFixed(2)}`
}

watch(
  [() => props.show, () => props.mode, () => props.campaignId],
  openDialog,
  { immediate: true }
)

onUnmounted(() => {
  detailGeneration += 1
  detailController?.abort()
})
</script>

<style scoped>
.campaign-preview-tile {
  display: flex;
  min-height: 5.5rem;
  flex-direction: column;
  justify-content: space-between;
  border: 1px solid rgb(226 232 240 / 0.9);
  border-radius: 0.875rem;
  background: rgb(248 250 252 / 0.85);
  padding: 0.9rem 1rem;
  color: rgb(100 116 139);
  font-size: 0.75rem;
}

.campaign-preview-tile strong {
  color: rgb(15 23 42);
  font-size: 1.25rem;
  font-variant-numeric: tabular-nums;
}

.campaign-preview-tile--accent {
  border-color: rgb(191 219 254 / 0.9);
  background: rgb(239 246 255 / 0.9);
  color: rgb(37 99 235);
}

.campaign-preview-tile--accent strong {
  color: rgb(29 78 216);
}

.dark .campaign-preview-tile {
  border-color: rgb(51 65 85 / 0.95);
  background: rgb(15 23 42 / 0.45);
  color: rgb(148 163 184);
}

.dark .campaign-preview-tile strong {
  color: rgb(248 250 252);
}

.dark .campaign-preview-tile--accent {
  border-color: rgb(37 99 235 / 0.4);
  background: rgb(30 64 175 / 0.16);
  color: rgb(147 197 253);
}
</style>
