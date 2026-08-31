<template>
  <BaseDialog
    :show="show"
    :title="isEditing ? t('admin.groups.systemCustom.editTitle') : t('admin.groups.systemCustom.createTitle')"
    width="wide"
    @close="requestClose"
  >
    <div class="space-y-5">
      <div
        v-if="errorMessage"
        data-testid="system-custom-error"
        role="alert"
        aria-live="assertive"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
      >
        {{ errorMessage }}
      </div>

      <section class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_11rem]">
        <label class="block">
          <span class="input-label">{{ t('admin.groups.form.name') }}</span>
          <input
            v-model="form.name"
            data-testid="system-custom-name"
            class="input"
            type="text"
            :placeholder="t('admin.groups.systemCustom.namePlaceholder')"
          />
        </label>
        <label class="block">
          <span class="input-label">{{ t('admin.groups.subscription.defaultValidityDays') }}</span>
          <input
            v-model.number="form.default_validity_days"
            data-testid="system-custom-validity-days"
            class="input"
            type="number"
            min="1"
            max="3650"
            step="1"
          />
        </label>
      </section>

      <section class="border-y border-slate-200 py-4 dark:border-dark-700">
        <button
          data-testid="system-custom-advanced-toggle"
          class="flex w-full items-center justify-between gap-3 text-left"
          type="button"
          :aria-expanded="advancedSettingsOpen"
          @click="advancedSettingsOpen = !advancedSettingsOpen"
        >
          <span>
            <span class="block text-sm font-semibold text-slate-900 dark:text-slate-100">
              {{ advancedSettingsOpen ? t('admin.groups.systemCustom.hideAdvancedSettings') : t('admin.groups.systemCustom.showAdvancedSettings') }}
            </span>
            <span class="mt-1 block text-xs leading-5 text-slate-500 dark:text-slate-400">
              {{ t('admin.groups.systemCustom.quotaHint') }}
            </span>
          </span>
          <Icon :name="advancedSettingsOpen ? 'chevronUp' : 'chevronDown'" size="sm" class="flex-none text-slate-500" />
        </button>

        <div
          v-show="advancedSettingsOpen"
          data-testid="system-custom-advanced-settings"
          class="mt-4 grid gap-4 border-t border-slate-100 pt-4 dark:border-dark-700 sm:grid-cols-3"
        >
          <label class="block sm:col-span-3">
            <span class="input-label">{{ t('admin.groups.form.description') }}</span>
            <textarea
              v-model="form.description"
              class="input min-h-20 resize-y"
              :placeholder="t('admin.groups.optionalDescription')"
            />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.groups.subscription.dailyLimit') }}</span>
            <input v-model="form.daily_limit_usd" data-testid="system-custom-daily-limit" class="input" type="number" min="0" step="0.01" :placeholder="t('admin.groups.subscription.noLimit')" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.groups.subscription.weeklyLimit') }}</span>
            <input v-model="form.weekly_limit_usd" class="input" type="number" min="0" step="0.01" :placeholder="t('admin.groups.subscription.noLimit')" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.groups.subscription.monthlyLimit') }}</span>
            <input v-model="form.monthly_limit_usd" data-testid="system-custom-monthly-limit" class="input" type="number" min="0" step="0.01" :placeholder="t('admin.groups.subscription.noLimit')" />
          </label>
        </div>
      </section>

      <section data-testid="system-custom-source-workspace" class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.9fr)]">
        <div class="min-w-0">
          <div class="mb-3 flex items-start gap-3">
            <span class="mt-0.5 flex h-8 w-8 flex-none items-center justify-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-300">
              <Icon name="grid" size="sm" />
            </span>
            <div>
              <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">{{ t('admin.groups.systemCustom.sourcesTitle') }}</h4>
              <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('admin.groups.systemCustom.sourcesHint') }}</p>
            </div>
          </div>

          <div class="overflow-hidden rounded-lg border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-800">
            <p v-if="loading" class="px-4 py-8 text-center text-sm text-slate-500">{{ t('common.loading') }}</p>
            <p v-else-if="candidates.length === 0" class="px-4 py-8 text-center text-sm text-slate-500">{{ t('admin.groups.systemCustom.noSources') }}</p>
            <div v-else class="divide-y divide-slate-100 dark:divide-dark-700">
              <label
                v-for="candidate in candidates"
                :key="candidate.group.id"
                :class="[
                  'flex cursor-pointer items-start gap-3 px-4 py-3 transition-colors',
                  isSourceSelected(candidate.group.id) ? 'bg-primary-50/70 dark:bg-primary-500/10' : 'hover:bg-slate-50 dark:hover:bg-dark-700/60',
                  isUnavailableCandidate(candidate) && !isSourceSelected(candidate.group.id) ? 'cursor-not-allowed opacity-60' : ''
                ]"
              >
                <input
                  :checked="isSourceSelected(candidate.group.id)"
                  :data-source-id="candidate.group.id"
                  data-testid="system-custom-source-select"
                  class="mt-0.5 h-4 w-4 rounded border-slate-300 text-primary-600 focus:ring-primary-500"
                  type="checkbox"
                  :disabled="isUnavailableCandidate(candidate) && !isSourceSelected(candidate.group.id)"
                  @change="toggleSource(candidate.group.id, ($event.target as HTMLInputElement).checked)"
                />
                <span class="min-w-0 flex-1">
                  <span class="flex items-center gap-2">
                    <span class="truncate text-sm font-medium text-slate-800 dark:text-slate-100">{{ candidate.group.name }}</span>
                    <span class="flex-none rounded bg-slate-100 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-slate-500 dark:bg-dark-700 dark:text-slate-400">{{ candidate.group.platform || 'legacy' }}</span>
                  </span>
                  <span class="mt-1 block text-xs text-slate-500 dark:text-slate-400">
                    {{ t('admin.groups.systemCustom.sourceModelCount', { count: candidate.models.length }) }}
                  </span>
                  <span v-if="isUnavailableCandidate(candidate)" data-testid="system-custom-source-unavailable" class="mt-1 block text-[11px] font-medium text-amber-600 dark:text-amber-300">
                    {{ t('admin.groups.systemCustom.sourceUnavailable') }}
                  </span>
                </span>
              </label>
            </div>
          </div>
        </div>

        <div class="min-w-0">
          <div class="mb-3 flex items-start gap-3">
            <span class="mt-0.5 flex h-8 w-8 flex-none items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300">
              <Icon name="arrowsUpDown" size="sm" />
            </span>
            <div>
              <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">{{ t('admin.groups.systemCustom.priorityTitle') }}</h4>
              <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ t('admin.groups.systemCustom.priorityHint') }}</p>
            </div>
          </div>

          <div class="overflow-hidden rounded-lg border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-800">
            <div v-if="selectedCandidates.length === 0" data-testid="system-custom-no-selected-sources" class="px-4 py-10 text-center">
              <Icon name="sparkles" size="lg" class="mx-auto text-slate-300 dark:text-dark-500" />
              <p class="mt-3 text-sm text-slate-500 dark:text-slate-400">{{ t('admin.groups.systemCustom.selectSourceFirst') }}</p>
            </div>
            <ol v-else data-testid="system-custom-priority-list" class="divide-y divide-slate-100 dark:divide-dark-700">
              <li v-for="(candidate, index) in selectedCandidates" :key="candidate.group.id" :data-source-id="candidate.group.id" data-testid="system-custom-priority-row" class="flex items-center gap-3 px-3 py-3">
                <span class="flex h-6 w-6 flex-none items-center justify-center rounded-full bg-slate-100 text-xs font-semibold tabular-nums text-slate-600 dark:bg-dark-700 dark:text-slate-300">{{ index + 1 }}</span>
                <span class="min-w-0 flex-1">
                  <span class="block truncate text-sm font-medium text-slate-800 dark:text-slate-100">{{ candidate.group.name }}</span>
                  <span class="mt-0.5 block text-xs text-slate-500 dark:text-slate-400">{{ candidate.group.platform || 'legacy' }} · {{ t('admin.groups.systemCustom.sourceModelCount', { count: candidate.models.length }) }}</span>
                </span>
                <div class="flex flex-none items-center gap-1">
                  <button data-testid="system-custom-priority-up" class="btn-ghost btn-icon h-8 w-8" type="button" :disabled="index === 0" :title="t('admin.groups.systemCustom.moveUp')" :aria-label="t('admin.groups.systemCustom.moveUp')" @click="moveSource(candidate.group.id, -1)"><Icon name="arrowUp" size="sm" /></button>
                  <button data-testid="system-custom-priority-down" class="btn-ghost btn-icon h-8 w-8" type="button" :disabled="index === selectedCandidates.length - 1" :title="t('admin.groups.systemCustom.moveDown')" :aria-label="t('admin.groups.systemCustom.moveDown')" @click="moveSource(candidate.group.id, 1)"><Icon name="arrowDown" size="sm" /></button>
                  <button data-testid="system-custom-priority-remove" class="btn-ghost btn-icon h-8 w-8 text-red-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30" type="button" :title="t('common.delete')" :aria-label="t('common.delete')" @click="toggleSource(candidate.group.id, false)"><Icon name="x" size="sm" /></button>
                </div>
              </li>
            </ol>
          </div>
        </div>
      </section>

      <section data-testid="system-custom-dynamic-summary" class="rounded-lg border border-primary-100 bg-primary-50/50 px-4 py-4 dark:border-primary-500/20 dark:bg-primary-500/5">
        <div class="flex items-start gap-3">
          <Icon name="sync" size="md" class="mt-0.5 flex-none text-primary-600 dark:text-primary-300" />
          <div class="min-w-0 flex-1">
            <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">{{ t('admin.groups.systemCustom.dynamicModelsTitle') }}</h4>
            <p class="mt-1 text-xs leading-5 text-slate-600 dark:text-slate-300">{{ t('admin.groups.systemCustom.dynamicModelsHint') }}</p>
            <dl class="mt-3 grid grid-cols-3 gap-2 text-center">
              <div class="rounded-md bg-white/80 px-2 py-2 dark:bg-dark-800/70"><dt class="text-[11px] text-slate-500">{{ t('admin.groups.systemCustom.selectedSources') }}</dt><dd data-testid="system-custom-selected-source-count" class="mt-0.5 text-sm font-semibold tabular-nums text-slate-900 dark:text-white">{{ selectedCandidates.length }}</dd></div>
              <div class="rounded-md bg-white/80 px-2 py-2 dark:bg-dark-800/70"><dt class="text-[11px] text-slate-500">{{ t('admin.groups.systemCustom.uniqueModels') }}</dt><dd data-testid="system-custom-unique-model-count" class="mt-0.5 text-sm font-semibold tabular-nums text-slate-900 dark:text-white">{{ modelSummary.uniqueModels }}</dd></div>
              <div class="rounded-md bg-white/80 px-2 py-2 dark:bg-dark-800/70"><dt class="text-[11px] text-slate-500">{{ t('admin.groups.systemCustom.fallbackRoutes') }}</dt><dd data-testid="system-custom-fallback-count" class="mt-0.5 text-sm font-semibold tabular-nums text-slate-900 dark:text-white">{{ modelSummary.fallbackRoutes }}</dd></div>
            </dl>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <button v-if="isEditing && !confirmingDelete" data-testid="system-custom-delete" class="btn border border-red-200 bg-white text-red-600 hover:bg-red-50 dark:border-red-900/70 dark:bg-dark-800 dark:text-red-300 dark:hover:bg-red-950/30" type="button" :disabled="busy" @click="confirmingDelete = true">{{ t('admin.groups.systemCustom.deleteAction') }}</button>
          <div v-else-if="confirmingDelete" class="flex flex-wrap items-center gap-2">
            <span class="text-xs font-medium text-red-600 dark:text-red-300">{{ t('admin.groups.systemCustom.deleteConfirm') }}</span>
            <button data-testid="system-custom-delete-confirm" class="btn bg-red-600 text-white hover:bg-red-700" type="button" :disabled="deleting" @click="deleteGroup">{{ deleting ? t('admin.groups.systemCustom.deleting') : t('common.confirm') }}</button>
            <button class="btn btn-secondary" type="button" :disabled="deleting" @click="confirmingDelete = false">{{ t('common.cancel') }}</button>
          </div>
        </div>
        <div class="flex w-full gap-3 sm:w-auto">
          <button class="btn btn-secondary flex-1 sm:flex-none" type="button" :disabled="busy" @click="requestClose">{{ t('common.cancel') }}</button>
          <button data-testid="system-custom-save" class="btn btn-primary flex-1 sm:flex-none" type="button" :disabled="saveDisabled" @click="save">{{ saving ? t('admin.groups.saving') : t('common.save') }}</button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type {
  CreateSystemCustomGroupRequest,
  SystemCustomGroup,
  SystemCustomGroupCandidate,
  SystemCustomGroupSource
} from '@/types'

interface Props {
  show: boolean
  groupId?: number | null
}

interface Emits {
  (event: 'close'): void
  (event: 'saved', group: SystemCustomGroup): void
  (event: 'deleted', groupID: number): void
}

const props = withDefaults(defineProps<Props>(), { groupId: null })
const emit = defineEmits<Emits>()
const { t } = useI18n()

const candidates = ref<SystemCustomGroupCandidate[]>([])
const selectedSourceIDs = ref<number[]>([])
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const confirmingDelete = ref(false)
const advancedSettingsOpen = ref(false)
const errorMessage = ref('')
let session = 0

const form = reactive({
  name: '',
  description: '',
  daily_limit_usd: '' as number | string | null,
  weekly_limit_usd: '' as number | string | null,
  monthly_limit_usd: '' as number | string | null,
  default_validity_days: 30
})

const isEditing = computed(() => props.groupId !== null)
const busy = computed(() => loading.value || saving.value || deleting.value)
const isCurrentSession = (activeSession: number, targetID: number | null) =>
  activeSession === session && props.show && props.groupId === targetID

const reset = () => {
  form.name = ''
  form.description = ''
  form.daily_limit_usd = ''
  form.weekly_limit_usd = ''
  form.monthly_limit_usd = ''
  form.default_validity_days = 30
  candidates.value = []
  selectedSourceIDs.value = []
  advancedSettingsOpen.value = false
  confirmingDelete.value = false
  errorMessage.value = ''
}

const isSourceSelected = (sourceID: number) => selectedSourceIDs.value.includes(sourceID)

const isUnavailableCandidate = (candidate: SystemCustomGroupCandidate) =>
  candidate.group.status !== 'active' || !candidate.group.platform

const selectedCandidates = computed(() =>
  selectedSourceIDs.value
    .map((sourceID) => candidates.value.find((candidate) => candidate.group.id === sourceID))
    .filter((candidate): candidate is SystemCustomGroupCandidate => Boolean(candidate))
)

const modelSummary = computed(() => {
  const modelSources = new Map<string, number>()
  for (const candidate of selectedCandidates.value) {
    for (const model of new Set(candidate.models.map((item) => item.trim()).filter(Boolean))) {
      const key = model.toLowerCase()
      modelSources.set(key, (modelSources.get(key) ?? 0) + 1)
    }
  }
  return {
    uniqueModels: modelSources.size,
    fallbackRoutes: [...modelSources.values()].reduce((total, count) => total + Math.max(0, count - 1), 0)
  }
})

const addUnavailableSources = (detail: SystemCustomGroup) => {
  const existing = new Set(candidates.value.map((candidate) => candidate.group.id))
  const legacyModelsBySource = new Map<number, string[]>()
  for (const model of detail.models ?? []) {
    const sourceModels = legacyModelsBySource.get(model.source_group_id) ?? []
    sourceModels.push(model.source_model)
    legacyModelsBySource.set(model.source_group_id, sourceModels)
  }
  const sources = detail.sources?.length
    ? detail.sources
    : [...legacyModelsBySource.keys()].map((source_group_id, priority) => ({ source_group_id, priority, group: undefined }))
  for (const source of sources) {
    if (existing.has(source.source_group_id)) continue
    const group: SystemCustomGroupSource = source.group ?? {
      id: source.source_group_id,
      name: t('admin.groups.systemCustom.unavailableSourceFallback', { source: source.source_group_id }),
      status: 'inactive'
    }
    candidates.value.push({ group, models: legacyModelsBySource.get(source.source_group_id) ?? [] })
    existing.add(source.source_group_id)
  }
}

const sourceIDsFromDetail = (detail: SystemCustomGroup) => {
  if (detail.sources?.length) {
    return [...detail.sources]
      .sort((left, right) => left.priority - right.priority)
      .map((source) => source.source_group_id)
  }
  return [...new Set((detail.models ?? []).map((model) => model.source_group_id))]
}

const load = async () => {
  const activeSession = ++session
  const targetID = props.groupId
  reset()
  loading.value = true
  try {
    const [sourceCandidates, detail] = await Promise.all([
      adminAPI.groups.getSystemCustomGroupCandidates(),
      targetID === null ? Promise.resolve(null) : adminAPI.groups.getSystemCustomGroup(targetID)
    ])
    if (!isCurrentSession(activeSession, targetID)) return
    candidates.value = [...sourceCandidates]
    if (detail) {
      addUnavailableSources(detail)
      form.name = detail.group.name
      form.description = detail.group.description || ''
      form.daily_limit_usd = detail.group.daily_limit_usd ?? ''
      form.weekly_limit_usd = detail.group.weekly_limit_usd ?? ''
      form.monthly_limit_usd = detail.group.monthly_limit_usd ?? ''
      form.default_validity_days = detail.group.default_validity_days
      advancedSettingsOpen.value = Boolean(
        detail.group.description || detail.group.daily_limit_usd !== null || detail.group.weekly_limit_usd !== null || detail.group.monthly_limit_usd !== null
      )
      selectedSourceIDs.value = sourceIDsFromDetail(detail)
    }
  } catch (error) {
    if (isCurrentSession(activeSession, targetID)) errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.loadFailed'))
  } finally {
    if (isCurrentSession(activeSession, targetID)) loading.value = false
  }
}

watch(
  () => [props.show, props.groupId] as const,
  ([show]) => {
    if (show) void load()
    else session += 1
  },
  { immediate: true }
)

const toggleSource = (sourceID: number, selected: boolean) => {
  if (selected) {
    if (!isSourceSelected(sourceID)) selectedSourceIDs.value = [...selectedSourceIDs.value, sourceID]
  } else {
    selectedSourceIDs.value = selectedSourceIDs.value.filter((id) => id !== sourceID)
  }
}

const moveSource = (sourceID: number, direction: -1 | 1) => {
  const index = selectedSourceIDs.value.indexOf(sourceID)
  const targetIndex = index + direction
  if (index < 0 || targetIndex < 0 || targetIndex >= selectedSourceIDs.value.length) return
  const next = [...selectedSourceIDs.value]
  ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
  selectedSourceIDs.value = next
}

const nullableNumber = (value: number | string | null) => {
  if (value === '' || value === null || value === undefined) return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

const validNullableLimit = (value: number | string | null) => {
  if (value === '' || value === null || value === undefined) return true
  if (typeof value === 'string' && value.trim() === '') return false
  const number = Number(value)
  return Number.isFinite(number) && number >= 0
}

const numbersValid = computed(() => {
  const validity = Number(form.default_validity_days)
  return validNullableLimit(form.daily_limit_usd) && validNullableLimit(form.weekly_limit_usd) && validNullableLimit(form.monthly_limit_usd) && Number.isInteger(validity) && validity >= 1 && validity <= 3650
})

const saveDisabled = computed(() =>
  busy.value || confirmingDelete.value || !form.name.trim() || selectedSourceIDs.value.length === 0 || !numbersValid.value
)

const snapshot = (): CreateSystemCustomGroupRequest => ({
  name: form.name.trim(),
  description: form.description.trim() || null,
  daily_limit_usd: nullableNumber(form.daily_limit_usd),
  weekly_limit_usd: nullableNumber(form.weekly_limit_usd),
  monthly_limit_usd: nullableNumber(form.monthly_limit_usd),
  default_validity_days: Number(form.default_validity_days) || 30,
  source_group_ids: selectedSourceIDs.value
})

const save = async () => {
  if (saveDisabled.value) return
  const activeSession = session
  const targetID = props.groupId
  saving.value = true
  errorMessage.value = ''
  try {
    const request = snapshot()
    const saved = targetID === null
      ? await adminAPI.groups.createSystemCustomGroup(request)
      : await adminAPI.groups.updateSystemCustomGroup(targetID, request)
    if (isCurrentSession(activeSession, targetID)) emit('saved', saved)
  } catch (error) {
    if (isCurrentSession(activeSession, targetID)) errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.saveFailed'))
  } finally {
    if (isCurrentSession(activeSession, targetID)) saving.value = false
  }
}

const requestClose = () => {
  if (!busy.value) emit('close')
}

const deleteGroup = async () => {
  if (props.groupId === null || deleting.value) return
  const activeSession = session
  const targetID = props.groupId
  deleting.value = true
  errorMessage.value = ''
  try {
    await adminAPI.groups.deleteSystemCustomGroup(targetID)
    if (isCurrentSession(activeSession, targetID)) emit('deleted', targetID)
  } catch (error) {
    if (isCurrentSession(activeSession, targetID)) errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.deleteFailed'))
  } finally {
    if (isCurrentSession(activeSession, targetID)) deleting.value = false
  }
}

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : null

const formatApiError = (error: unknown, fallback: string) => {
  const record = asRecord(error)
  const response = asRecord(record?.response)
  const data = asRecord(response?.data) ?? record
  const nested = asRecord(data?.error)
  const sources = [nested, data, record].filter((item): item is Record<string, unknown> => item !== null)
  const code = sources.map((source) => source.code).find((value): value is string => typeof value === 'string')
  if (code === 'GROUP_EXISTS') return t('admin.groups.systemCustom.groupExists')
  const message = sources
    .flatMap((source) => ['message', 'reason', 'detail'].map((key) => source[key]))
    .find((value): value is string => typeof value === 'string' && value.trim().length > 0 && value.trim().length <= 500)
  return message?.trim() || fallback
}
</script>
