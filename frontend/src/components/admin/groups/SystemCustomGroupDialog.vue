<template>
  <BaseDialog
    :show="show"
    :title="
      isEditing
        ? t('admin.groups.systemCustom.editTitle')
        : t('admin.groups.systemCustom.createTitle')
    "
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <div
        v-if="errorMessage"
        data-testid="system-custom-error"
        class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
      >
        {{ errorMessage }}
      </div>

      <div class="grid gap-4 md:grid-cols-2">
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
            class="input"
            type="number"
            min="1"
            step="1"
          />
        </label>
        <label class="block md:col-span-2">
          <span class="input-label">{{ t('admin.groups.form.description') }}</span>
          <textarea
            v-model="form.description"
            class="input min-h-20 resize-y"
            :placeholder="t('admin.groups.optionalDescription')"
          />
        </label>
      </div>

      <section class="rounded-2xl border border-slate-200 bg-slate-50/70 p-4 dark:border-dark-700 dark:bg-dark-900/30">
        <div class="mb-3">
          <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
            {{ t('admin.groups.systemCustom.quotaTitle') }}
          </h4>
          <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
            {{ t('admin.groups.systemCustom.quotaHint') }}
          </p>
        </div>
        <div class="grid gap-3 sm:grid-cols-3">
          <label class="block">
            <span class="input-label">{{ t('admin.groups.subscription.dailyLimit') }}</span>
            <input
              v-model="form.daily_limit_usd"
              data-testid="system-custom-daily-limit"
              class="input"
              type="number"
              min="0"
              step="0.01"
              :placeholder="t('admin.groups.subscription.noLimit')"
            />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.groups.subscription.weeklyLimit') }}</span>
            <input
              v-model="form.weekly_limit_usd"
              class="input"
              type="number"
              min="0"
              step="0.01"
              :placeholder="t('admin.groups.subscription.noLimit')"
            />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.groups.subscription.monthlyLimit') }}</span>
            <input
              v-model="form.monthly_limit_usd"
              data-testid="system-custom-monthly-limit"
              class="input"
              type="number"
              min="0"
              step="0.01"
              :placeholder="t('admin.groups.subscription.noLimit')"
            />
          </label>
        </div>
      </section>

      <div class="grid min-h-[24rem] gap-4 lg:grid-cols-[15rem_minmax(0,1fr)]">
        <aside class="rounded-2xl border border-slate-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800">
          <div class="mb-3 px-1">
            <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
              {{ t('admin.groups.systemCustom.sourcesTitle') }}
            </h4>
            <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">
              {{ t('admin.groups.systemCustom.sourcesHint') }}
            </p>
          </div>
          <p v-if="loading" class="px-1 py-3 text-xs text-slate-500">
            {{ t('common.loading') }}
          </p>
          <p v-else-if="candidates.length === 0" class="px-1 py-3 text-xs text-slate-500">
            {{ t('admin.groups.systemCustom.noSources') }}
          </p>
          <div v-else class="space-y-1.5">
            <label
              v-for="candidate in candidates"
              :key="candidate.group.id"
              :class="[
                'flex cursor-pointer items-start gap-3 rounded-xl border px-3 py-2.5 transition-colors',
                isSourceSelected(candidate.group.id)
                  ? 'border-primary-200 bg-primary-50/80 dark:border-primary-500/30 dark:bg-primary-500/10'
                  : 'border-transparent hover:border-slate-200 hover:bg-slate-50 dark:hover:border-dark-600 dark:hover:bg-dark-700/70'
              ]"
            >
              <input
                :checked="isSourceSelected(candidate.group.id)"
                :data-source-id="candidate.group.id"
                data-testid="system-custom-source-select"
                class="mt-0.5 h-4 w-4 rounded border-slate-300 text-primary-600 focus:ring-primary-500"
                type="checkbox"
                @change="toggleSource(candidate.group.id, ($event.target as HTMLInputElement).checked)"
              />
              <span class="min-w-0">
                <span class="block truncate text-sm font-medium text-slate-800 dark:text-slate-100">
                  {{ candidate.group.name }}
                </span>
                <span class="mt-0.5 block text-xs text-slate-500 dark:text-slate-400">
                  {{ candidate.group.platform }} · {{ candidate.models.length }}
                </span>
              </span>
            </label>
          </div>
        </aside>

        <section class="min-w-0 rounded-2xl border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="flex flex-wrap items-start justify-between gap-3 border-b border-slate-200 px-4 py-3 dark:border-dark-700">
            <div>
              <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
                {{ t('admin.groups.systemCustom.modelsTitle') }}
              </h4>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                {{ t('admin.groups.systemCustom.modelsHint') }}
              </p>
            </div>
            <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600 dark:bg-dark-700 dark:text-slate-300">
              {{ selectedRoutes.length }} / {{ visibleRoutes.length }}
            </span>
          </div>

          <div v-if="selectedCandidates.length === 0" class="flex min-h-64 items-center justify-center p-8 text-center">
            <div>
              <Icon name="grid" size="lg" class="mx-auto text-slate-300 dark:text-dark-500" />
              <p class="mt-3 text-sm text-slate-500 dark:text-slate-400">
                {{ t('admin.groups.systemCustom.selectSourceFirst') }}
              </p>
            </div>
          </div>
          <div v-else class="max-h-[34rem] space-y-5 overflow-y-auto p-4">
            <section v-for="candidate in selectedCandidates" :key="candidate.group.id">
              <div class="mb-2 flex items-center gap-2">
                <span class="text-sm font-semibold text-slate-800 dark:text-slate-100">
                  {{ candidate.group.name }}
                </span>
                <span class="rounded-md bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-500 dark:bg-dark-700 dark:text-slate-400">
                  {{ candidate.group.platform }}
                </span>
              </div>
              <div class="space-y-2">
                <div
                  v-for="route in routesForSource(candidate.group.id)"
                  :key="route.key"
                  :data-source-id="route.source_group_id"
                  :data-source-model="route.source_model"
                  data-testid="system-custom-model-row"
                  :class="[
                    'grid gap-3 rounded-xl border p-3 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,1fr)_auto] sm:items-center',
                    route.selected
                      ? 'border-primary-200 bg-primary-50/40 dark:border-primary-500/25 dark:bg-primary-500/5'
                      : 'border-slate-200 dark:border-dark-700'
                  ]"
                >
                  <label class="flex min-w-0 cursor-pointer items-start gap-3">
                    <input
                      :checked="route.selected"
                      class="mt-0.5 h-4 w-4 rounded border-slate-300 text-primary-600 focus:ring-primary-500"
                      type="checkbox"
                      @change="route.selected = ($event.target as HTMLInputElement).checked"
                    />
                    <span class="min-w-0">
                      <span class="block break-all font-mono text-xs font-medium text-slate-800 dark:text-slate-200">
                        {{ route.source_model }}
                      </span>
                      <span class="mt-1 block text-[11px] text-slate-500 dark:text-slate-400">
                        {{ t('admin.groups.systemCustom.sourceModel') }}
                      </span>
                    </span>
                  </label>
                  <label class="block min-w-0">
                    <span class="sr-only">{{ t('admin.groups.systemCustom.publicModel') }}</span>
                    <input
                      v-model="route.public_model"
                      data-testid="system-custom-public-model"
                      :disabled="!route.selected"
                      class="input font-mono text-xs"
                      type="text"
                      :placeholder="t('admin.groups.systemCustom.publicModel')"
                    />
                  </label>
                  <label v-if="route.selected" class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-300">
                    <input
                      v-model="route.enabled"
                      class="h-4 w-4 rounded border-slate-300 text-primary-600 focus:ring-primary-500"
                      type="checkbox"
                    />
                    {{ t('common.enabled') }}
                  </label>
                </div>
              </div>
            </section>
          </div>
        </section>
      </div>

      <div
        v-if="conflictingPublicModels.length > 0"
        data-testid="system-custom-conflict"
        class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300"
      >
        <p class="font-medium">{{ t('admin.groups.systemCustom.conflictTitle') }}</p>
        <p class="mt-1 break-all text-xs leading-5">
          {{ conflictingPublicModels.join(', ') }} · {{ t('admin.groups.systemCustom.conflictHint') }}
        </p>
      </div>

      <section v-if="isEditing" class="rounded-2xl border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-4 py-3 dark:border-dark-700">
          <div>
            <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
              {{ t('admin.groups.systemCustom.syncTitle') }}
            </h4>
            <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
              {{ t('admin.groups.systemCustom.syncHint') }}
            </p>
          </div>
          <button
            data-testid="system-custom-sync"
            class="btn btn-secondary"
            type="button"
            :disabled="syncing"
            @click="loadSyncPreview"
          >
            <Icon name="refresh" size="sm" :class="syncing ? 'animate-spin' : ''" />
            {{ t('admin.groups.systemCustom.syncAction') }}
          </button>
        </div>
        <div v-if="syncPreview" class="grid gap-4 p-4 lg:grid-cols-3">
          <div class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-3 dark:border-emerald-900/50 dark:bg-emerald-950/20">
            <h5 class="text-xs font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
              {{ t('admin.groups.systemCustom.syncAdded') }} · {{ syncPreview.added.length }}
            </h5>
            <p v-if="syncPreview.added.length === 0" class="mt-3 text-xs text-slate-500">—</p>
            <label
              v-for="added in syncPreview.added"
              :key="routeKey(added.source_group_id, added.source_model)"
              data-testid="system-custom-sync-added"
              class="mt-3 flex cursor-pointer items-start gap-2 text-xs text-slate-700 dark:text-slate-200"
            >
              <input
                :checked="syncAddedSelections.has(routeKey(added.source_group_id, added.source_model))"
                class="mt-0.5 h-4 w-4 rounded border-slate-300 text-primary-600"
                type="checkbox"
                @change="toggleSyncAdded(added, ($event.target as HTMLInputElement).checked)"
              />
              <span class="min-w-0 break-all">
                {{ added.public_model }}
                <span class="mt-0.5 block text-[11px] text-slate-500">
                  {{ t('admin.groups.systemCustom.addedUnselected') }}
                </span>
              </span>
            </label>
          </div>
          <div class="rounded-xl border border-amber-200 bg-amber-50/60 p-3 dark:border-amber-900/50 dark:bg-amber-950/20">
            <h5 class="text-xs font-semibold uppercase tracking-wide text-amber-700 dark:text-amber-300">
              {{ t('admin.groups.systemCustom.syncMissing') }} · {{ syncPreview.missing.length }}
            </h5>
            <p v-if="syncPreview.missing.length === 0" class="mt-3 text-xs text-slate-500">—</p>
            <label
              v-for="missing in syncPreview.missing"
              :key="routeKey(missing.source_group_id, missing.source_model)"
              data-testid="system-custom-sync-missing"
              class="mt-3 flex cursor-pointer items-start gap-2 text-xs text-slate-700 dark:text-slate-200"
            >
              <input
                :checked="syncMissingDisableSelections.has(routeKey(missing.source_group_id, missing.source_model))"
                class="mt-0.5 h-4 w-4 rounded border-slate-300 text-amber-600"
                type="checkbox"
                @change="toggleSyncMissing(missing, ($event.target as HTMLInputElement).checked)"
              />
              <span class="min-w-0 break-all">
                {{ missing.public_model }}
                <span class="mt-0.5 block text-[11px] text-slate-500">
                  {{ t('admin.groups.systemCustom.disableSuggestion') }}
                </span>
              </span>
            </label>
          </div>
          <div class="rounded-xl border border-red-200 bg-red-50/60 p-3 dark:border-red-900/50 dark:bg-red-950/20">
            <h5 class="text-xs font-semibold uppercase tracking-wide text-red-700 dark:text-red-300">
              {{ t('admin.groups.systemCustom.syncConflicting') }} · {{ syncPreview.conflicting.length }}
            </h5>
            <p v-if="syncPreview.conflicting.length === 0" class="mt-3 text-xs text-slate-500">—</p>
            <div
              v-for="conflict in syncPreview.conflicting"
              :key="`${routeKey(conflict.source_group_id, conflict.source_model)}:${conflict.public_model}`"
              data-testid="system-custom-sync-conflicting"
              class="mt-3 break-all text-xs text-slate-700 dark:text-slate-200"
            >
              {{ conflict.public_model }}
              <span class="mt-0.5 block text-[11px] text-red-600 dark:text-red-300">
                {{ conflict.reason }}
              </span>
            </div>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-between gap-3">
        <p class="text-xs text-slate-500 dark:text-slate-400">
          {{ t('admin.groups.systemCustom.snapshotHint') }}
        </p>
        <div class="flex items-center gap-3">
          <button class="btn btn-secondary" type="button" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button
            data-testid="system-custom-save"
            class="btn btn-primary"
            type="button"
            :disabled="saveDisabled"
            @click="save"
          >
            {{ saving ? t('admin.groups.saving') : t('common.save') }}
          </button>
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
  SystemCustomGroupCandidate,
  SystemCustomGroupModel,
  SystemCustomGroupModelInput,
  SystemCustomGroupSyncAdded,
  SystemCustomGroupSyncPreview
} from '@/types'

interface Props {
  show: boolean
  groupId?: number | null
}

interface Emits {
  (event: 'close'): void
  (event: 'saved'): void
}

interface RouteDraft extends SystemCustomGroupModelInput {
  key: string
  selected: boolean
  originalEnabled: boolean
}

const props = withDefaults(defineProps<Props>(), { groupId: null })
const emit = defineEmits<Emits>()
const { t } = useI18n()

const candidates = ref<SystemCustomGroupCandidate[]>([])
const routes = reactive(new Map<string, RouteDraft>())
const selectedSourceIDs = ref<number[]>([])
const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const errorMessage = ref('')
const syncPreview = ref<SystemCustomGroupSyncPreview | null>(null)
const syncAddedSelections = reactive(new Set<string>())
const syncMissingDisableSelections = reactive(new Set<string>())

const form = reactive({
  name: '',
  description: '',
  daily_limit_usd: '' as number | string | null,
  weekly_limit_usd: '' as number | string | null,
  monthly_limit_usd: '' as number | string | null,
  default_validity_days: 30
})

const isEditing = computed(() => props.groupId !== null)

const routeKey = (sourceGroupID: number, sourceModel: string) =>
  `${sourceGroupID}:${sourceModel.toLocaleLowerCase()}`

const ensureRoute = (
  sourceGroupID: number,
  sourceModel: string,
  values?: Partial<RouteDraft>
) => {
  const key = routeKey(sourceGroupID, sourceModel)
  const existing = routes.get(key)
  if (existing) {
    if (values) Object.assign(existing, values)
    return existing
  }
  const route = reactive<RouteDraft>({
    key,
    public_model: sourceModel,
    source_group_id: sourceGroupID,
    source_model: sourceModel,
    enabled: true,
    selected: false,
    originalEnabled: true,
    ...values
  })
  routes.set(key, route)
  return route
}

const reset = () => {
  form.name = ''
  form.description = ''
  form.daily_limit_usd = ''
  form.weekly_limit_usd = ''
  form.monthly_limit_usd = ''
  form.default_validity_days = 30
  candidates.value = []
  routes.clear()
  selectedSourceIDs.value = []
  syncPreview.value = null
  syncAddedSelections.clear()
  syncMissingDisableSelections.clear()
  errorMessage.value = ''
}

const mergeCandidates = (items: SystemCustomGroupCandidate[]) => {
  candidates.value = items
  for (const candidate of items) {
    for (const model of candidate.models) ensureRoute(candidate.group.id, model)
  }
}

const load = async () => {
  reset()
  loading.value = true
  try {
    const [candidateItems, detail] = await Promise.all([
      adminAPI.groups.getSystemCustomGroupCandidates(),
      props.groupId === null
        ? Promise.resolve(null)
        : adminAPI.groups.getSystemCustomGroup(props.groupId)
    ])
    mergeCandidates(candidateItems)
    if (detail) {
      form.name = detail.group.name
      form.description = detail.group.description || ''
      form.daily_limit_usd = detail.group.daily_limit_usd ?? ''
      form.weekly_limit_usd = detail.group.weekly_limit_usd ?? ''
      form.monthly_limit_usd = detail.group.monthly_limit_usd ?? ''
      form.default_validity_days = detail.group.default_validity_days
      const sourceIDs = new Set<number>()
      for (const model of detail.models) {
        sourceIDs.add(model.source_group_id)
        ensureRoute(model.source_group_id, model.source_model, {
          public_model: model.public_model,
          enabled: model.enabled,
          originalEnabled: model.enabled,
          selected: true
        })
      }
      selectedSourceIDs.value = [...sourceIDs]
    }
  } catch (error) {
    errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.loadFailed'))
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.groupId] as const,
  ([show]) => {
    if (show) void load()
  },
  { immediate: true }
)

const isSourceSelected = (sourceID: number) => selectedSourceIDs.value.includes(sourceID)

const toggleSource = (sourceID: number, selected: boolean) => {
  if (selected) {
    if (!selectedSourceIDs.value.includes(sourceID)) {
      selectedSourceIDs.value = [...selectedSourceIDs.value, sourceID]
    }
    return
  }
  selectedSourceIDs.value = selectedSourceIDs.value.filter((id) => id !== sourceID)
}

const selectedCandidates = computed(() =>
  candidates.value.filter((candidate) => isSourceSelected(candidate.group.id))
)

const routesForSource = (sourceID: number) =>
  [...routes.values()].filter((route) => route.source_group_id === sourceID)

const visibleRoutes = computed(() =>
  selectedCandidates.value.flatMap((candidate) => routesForSource(candidate.group.id))
)

const selectedRoutes = computed(() => [...routes.values()].filter((route) => route.selected))

const duplicateSourceRoutes = computed(() => {
  const counts = new Map<string, number>()
  for (const route of selectedRoutes.value) {
    const key = routeKey(route.source_group_id, route.source_model)
    counts.set(key, (counts.get(key) ?? 0) + 1)
  }
  return [...counts.values()].some((count) => count > 1)
})

const conflictingPublicModels = computed(() => {
  const counts = new Map<string, { count: number; display: string }>()
  for (const route of selectedRoutes.value) {
    const display = route.public_model.trim()
    if (!display) continue
    const key = display.toLocaleLowerCase()
    const current = counts.get(key)
    counts.set(key, { count: (current?.count ?? 0) + 1, display: current?.display ?? display })
  }
  return [...counts.values()].filter(({ count }) => count > 1).map(({ display }) => display)
})

const hasEmptyPublicModel = computed(() =>
  selectedRoutes.value.some((route) => route.public_model.trim() === '')
)

const saveDisabled = computed(
  () =>
    loading.value ||
    saving.value ||
    !form.name.trim() ||
    selectedRoutes.value.length === 0 ||
    duplicateSourceRoutes.value ||
    hasEmptyPublicModel.value ||
    conflictingPublicModels.value.length > 0
)

const nullableNumber = (value: number | string | null) => {
  if (value === '' || value === null || value === undefined) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

const snapshot = (): CreateSystemCustomGroupRequest => ({
  name: form.name.trim(),
  description: form.description.trim() || null,
  daily_limit_usd: nullableNumber(form.daily_limit_usd),
  weekly_limit_usd: nullableNumber(form.weekly_limit_usd),
  monthly_limit_usd: nullableNumber(form.monthly_limit_usd),
  default_validity_days: Number(form.default_validity_days) || 30,
  models: selectedRoutes.value.map((route) => ({
    public_model: route.public_model.trim(),
    source_group_id: route.source_group_id,
    source_model: route.source_model,
    enabled: route.enabled
  }))
})

const save = async () => {
  if (saveDisabled.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    const request = snapshot()
    if (props.groupId === null) await adminAPI.groups.createSystemCustomGroup(request)
    else await adminAPI.groups.updateSystemCustomGroup(props.groupId, request)
    emit('saved')
  } catch (error) {
    errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.saveFailed'))
  } finally {
    saving.value = false
  }
}

const loadSyncPreview = async () => {
  if (props.groupId === null) return
  syncing.value = true
  errorMessage.value = ''
  try {
    syncPreview.value = await adminAPI.groups.getSystemCustomGroupSyncPreview(props.groupId)
    syncAddedSelections.clear()
    syncMissingDisableSelections.clear()
  } catch (error) {
    errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.syncFailed'))
  } finally {
    syncing.value = false
  }
}

const toggleSyncAdded = (added: SystemCustomGroupSyncAdded, selected: boolean) => {
  const key = routeKey(added.source_group_id, added.source_model)
  const route = ensureRoute(added.source_group_id, added.source_model, {
    public_model: added.public_model
  })
  route.selected = selected
  route.enabled = true
  if (selected) syncAddedSelections.add(key)
  else syncAddedSelections.delete(key)
}

const toggleSyncMissing = (missing: SystemCustomGroupModel, disable: boolean) => {
  const key = routeKey(missing.source_group_id, missing.source_model)
  const route = ensureRoute(missing.source_group_id, missing.source_model, {
    public_model: missing.public_model,
    selected: true
  })
  route.enabled = disable ? false : route.originalEnabled
  if (disable) syncMissingDisableSelections.add(key)
  else syncMissingDisableSelections.delete(key)
}

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : null

const formatMetadata = (metadata: unknown) => {
  if (typeof metadata === 'string') return metadata
  const record = asRecord(metadata)
  if (!record) return ''
  return Object.entries(record)
    .filter(([, value]) => ['string', 'number', 'boolean'].includes(typeof value))
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join(', ')
}

const formatApiError = (error: unknown, fallback: string) => {
  const errorRecord = asRecord(error)
  const response = asRecord(errorRecord?.response)
  const rawData = response?.data
  if (typeof rawData === 'string') return rawData
  const data = asRecord(rawData) ?? errorRecord
  const nestedError = asRecord(data?.error)
  const details = asRecord(nestedError?.details) ?? asRecord(data?.details)
  const sources = [nestedError, details, data, errorRecord].filter(
    (source): source is Record<string, unknown> => source !== null
  )
  const parts: string[] = []
  for (const field of ['message', 'reason', 'detail'] as const) {
    for (const source of sources) {
      const value = source[field]
      if (typeof value === 'string' && value.trim() && !parts.includes(value.trim())) {
        parts.push(value.trim())
      }
    }
  }
  for (const source of sources) {
    const value = formatMetadata(source.metadata)
    if (value && !parts.includes(value)) parts.push(value)
  }
  const specificParts = parts.filter((part) => part.toLocaleLowerCase() !== 'internal error')
  return (specificParts.length > 0 ? specificParts : parts).join(' · ') || fallback
}
</script>
