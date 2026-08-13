<template>
  <BaseDialog
    :show="show"
    :title="
      isEditing
        ? t('admin.groups.systemCustom.editTitle')
        : t('admin.groups.systemCustom.createTitle')
    "
    width="extra-wide"
    @close="requestClose"
  >
    <div class="space-y-5">
      <div
        v-if="errorMessage"
        data-testid="system-custom-error"
        role="alert"
        aria-live="assertive"
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
            data-testid="system-custom-validity-days"
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

      <div class="grid min-h-[24rem] gap-4 lg:h-[34rem] lg:min-h-0 lg:grid-cols-[15rem_minmax(0,1fr)]">
        <aside class="rounded-2xl border border-slate-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800 lg:flex lg:min-h-0 lg:flex-col lg:overflow-hidden">
          <div class="mb-3 px-1 lg:flex-shrink-0">
            <h4 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
              {{ t('admin.groups.systemCustom.sourcesTitle') }}
            </h4>
            <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">
              {{ t('admin.groups.systemCustom.sourcesHint') }}
            </p>
          </div>
          <div
            data-testid="system-custom-source-scroll"
            class="lg:min-h-0 lg:flex-1 lg:overflow-y-auto lg:overscroll-contain lg:pr-1"
          >
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
                  <span
                    v-if="isUnavailableCandidate(candidate)"
                    data-testid="system-custom-source-unavailable"
                    class="mt-1 block text-[11px] font-medium text-amber-600 dark:text-amber-300"
                  >
                    {{ t('admin.groups.systemCustom.sourceUnavailable') }}
                  </span>
                </span>
              </label>
            </div>
          </div>
        </aside>

        <section class="min-w-0 rounded-2xl border border-slate-200 bg-white dark:border-dark-700 dark:bg-dark-800 lg:flex lg:min-h-0 lg:flex-col lg:overflow-hidden">
          <div class="flex flex-wrap items-start justify-between gap-3 border-b border-slate-200 px-4 py-3 dark:border-dark-700 lg:flex-shrink-0">
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

          <div
            v-if="selectedCandidates.length > 0"
            class="flex gap-2 overflow-x-auto border-b border-slate-200 px-4 py-2.5 dark:border-dark-700 lg:flex-shrink-0"
          >
            <button
              v-for="candidate in selectedCandidates"
              :key="candidate.group.id"
              :data-source-id="candidate.group.id"
              data-testid="system-custom-source-nav"
              class="inline-flex flex-none items-center gap-1.5 rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-1.5 text-xs font-medium text-slate-700 transition-colors hover:border-primary-200 hover:bg-primary-50 hover:text-primary-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 dark:border-dark-600 dark:bg-dark-700 dark:text-slate-200 dark:hover:border-primary-500/30 dark:hover:bg-primary-500/10 dark:hover:text-primary-300"
              type="button"
              @click="scrollToSource(candidate.group.id)"
            >
              <span class="max-w-40 truncate">{{ candidate.group.name }}</span>
              <span class="text-[11px] text-slate-400 dark:text-slate-500">
                {{ routesForSource(candidate.group.id).length }}
              </span>
            </button>
          </div>

          <div
            ref="modelScrollRef"
            data-testid="system-custom-model-scroll"
            class="space-y-5 p-4 lg:min-h-0 lg:flex-1 lg:overflow-y-auto lg:overscroll-contain"
          >
            <div v-if="selectedCandidates.length === 0" class="flex min-h-64 items-center justify-center p-8 text-center">
              <div>
                <Icon name="grid" size="lg" class="mx-auto text-slate-300 dark:text-dark-500" />
                <p class="mt-3 text-sm text-slate-500 dark:text-slate-400">
                  {{ t('admin.groups.systemCustom.selectSourceFirst') }}
                </p>
              </div>
            </div>
            <template v-else>
              <section
                v-for="candidate in selectedCandidates"
                :key="candidate.group.id"
                :ref="(element) => setSourceSectionRef(candidate.group.id, element)"
                :data-source-id="candidate.group.id"
                data-testid="system-custom-source-section"
              >
                <div class="sticky top-0 z-10 -mx-1 mb-2 flex items-center gap-2 bg-white/95 px-1 py-2 backdrop-blur dark:bg-dark-800/95">
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
                        @change="toggleRouteSelected(route, ($event.target as HTMLInputElement).checked)"
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
            </template>
          </div>
        </section>
      </div>

      <div
        v-if="conflictingPublicModels.length > 0"
        data-testid="system-custom-conflict"
        role="alert"
        aria-live="polite"
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
            :disabled="busy || confirmingDelete"
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
                :checked="isSyncAddedSelected(added)"
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
                :checked="isSyncMissingDisabled(missing)"
                :disabled="isSyncMissingReadOnly(missing)"
                class="mt-0.5 h-4 w-4 rounded border-slate-300 text-amber-600"
                type="checkbox"
                @change="toggleSyncMissing(missing, ($event.target as HTMLInputElement).checked)"
              />
              <span class="min-w-0 break-all">
                {{ missing.public_model }}
                <span class="mt-0.5 block text-[11px] text-slate-500">
                  {{
                    isSyncMissingReadOnly(missing)
                      ? t('admin.groups.systemCustom.alreadyDisabled')
                      : t('admin.groups.systemCustom.disableSuggestion')
                  }}
                </span>
              </span>
            </label>
          </div>
          <div
            class="rounded-xl border border-red-200 bg-red-50/60 p-3 dark:border-red-900/50 dark:bg-red-950/20"
            role="alert"
            aria-live="polite"
          >
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
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <template v-if="isEditing">
            <button
              v-if="!confirmingDelete"
              data-testid="system-custom-delete"
              class="btn border border-red-200 bg-white text-red-600 hover:bg-red-50 dark:border-red-900/70 dark:bg-dark-800 dark:text-red-300 dark:hover:bg-red-950/30"
              type="button"
              :disabled="busy"
              @click="beginDeleteConfirmation"
            >
              {{ t('admin.groups.systemCustom.deleteAction') }}
            </button>
            <div
              v-else
              class="flex flex-wrap items-center gap-2 rounded-xl border border-red-200 bg-red-50 px-3 py-2 dark:border-red-900/60 dark:bg-red-950/30"
            >
              <span class="text-xs font-medium text-red-700 dark:text-red-300">
                {{ t('admin.groups.systemCustom.deleteConfirm') }}
              </span>
              <button
                data-testid="system-custom-delete-confirm"
                class="rounded-lg bg-red-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-red-700 disabled:opacity-50"
                type="button"
                :disabled="deleting"
                @click="deleteGroup"
              >
                {{ deleting ? t('admin.groups.systemCustom.deleting') : t('common.confirm') }}
              </button>
              <button
                class="rounded-lg px-2 py-1.5 text-xs font-medium text-slate-600 hover:bg-white dark:text-slate-300 dark:hover:bg-dark-700"
                type="button"
                :disabled="deleting"
                @click="confirmingDelete = false"
              >
                {{ t('common.cancel') }}
              </button>
            </div>
          </template>
          <p v-else class="text-xs text-slate-500 dark:text-slate-400">
            {{ t('admin.groups.systemCustom.snapshotHint') }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <button class="btn btn-secondary" type="button" :disabled="busy" @click="requestClose">
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
import type { ComponentPublicInstance } from 'vue'
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
  (event: 'deleted'): void
}

interface RouteDraft extends SystemCustomGroupModelInput {
  key: string
  selected: boolean
  originalEnabled: boolean
  draftInitialized: boolean
}

const props = withDefaults(defineProps<Props>(), { groupId: null })
const emit = defineEmits<Emits>()
const { t } = useI18n()

const candidates = ref<SystemCustomGroupCandidate[]>([])
const routes = reactive(new Map<string, RouteDraft>())
const selectedSourceIDs = ref<number[]>([])
const modelScrollRef = ref<HTMLElement | null>(null)
const sourceSectionRefs = new Map<number, HTMLElement>()
const loading = ref(false)
const saving = ref(false)
const syncing = ref(false)
const deleting = ref(false)
const confirmingDelete = ref(false)
const errorMessage = ref('')
const syncPreview = ref<SystemCustomGroupSyncPreview | null>(null)
let sessionGeneration = 0
const syncMissingPreviousEnabled = new Map<string, boolean>()

const form = reactive({
  name: '',
  description: '',
  daily_limit_usd: '' as number | string | null,
  weekly_limit_usd: '' as number | string | null,
  monthly_limit_usd: '' as number | string | null,
  default_validity_days: 30
})

const isEditing = computed(() => props.groupId !== null)
const busy = computed(() => loading.value || saving.value || syncing.value || deleting.value)
const currentSession = (generation: number, targetID: number | null) =>
  generation === sessionGeneration && props.show && props.groupId === targetID
const beginSession = () => ++sessionGeneration

const routeKey = (sourceGroupID: number, sourceModel: string) =>
  `${sourceGroupID}:${sourceModel.toLowerCase()}`

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
    draftInitialized: false,
    ...values
  })
  routes.set(key, route)
  return route
}

const reset = () => {
  loading.value = false
  saving.value = false
  syncing.value = false
  deleting.value = false
  form.name = ''
  form.description = ''
  form.daily_limit_usd = ''
  form.weekly_limit_usd = ''
  form.monthly_limit_usd = ''
  form.default_validity_days = 30
  candidates.value = []
  routes.clear()
  selectedSourceIDs.value = []
  sourceSectionRefs.clear()
  syncPreview.value = null
  errorMessage.value = ''
  confirmingDelete.value = false
  syncMissingPreviousEnabled.clear()
}

const mergeCandidates = (
  items: SystemCustomGroupCandidate[],
  options: { replace?: boolean } = {}
) => {
  const unavailableDraftSources = options.replace
    ? candidates.value.filter(
        (candidate) =>
          !items.some((fresh) => fresh.group.id === candidate.group.id) &&
          (selectedSourceIDs.value.includes(candidate.group.id) ||
            [...routes.values()].some(
              (route) => route.source_group_id === candidate.group.id && route.selected
            ))
      )
        .map((candidate) => ({
          ...candidate,
          group: { ...candidate.group, status: 'inactive' as const }
        }))
    : []
  candidates.value = [...items, ...unavailableDraftSources]
  for (const candidate of items) {
    for (const model of candidate.models) ensureRoute(candidate.group.id, model)
  }
}

const isUnavailableCandidate = (candidate: SystemCustomGroupCandidate) =>
  candidate.group.status !== 'active' || candidate.group.platform === undefined

const addOrphanCandidates = (models: SystemCustomGroupModel[]) => {
  const known = new Set(candidates.value.map((candidate) => candidate.group.id))
  const orphans = new Map<number, SystemCustomGroupCandidate>()
  for (const model of models) {
    if (known.has(model.source_group_id)) continue
    let candidate = orphans.get(model.source_group_id)
    if (!candidate) {
      candidate = {
        group: {
          id: model.source_group_id,
          name:
            model.source_group?.name ||
            t('admin.groups.systemCustom.unavailableSourceFallback', {
              source: model.source_group_id
            }),
          description: model.source_group?.description,
          status: 'inactive'
        },
        models: []
      }
      orphans.set(model.source_group_id, candidate)
    }
    if (!candidate.models.includes(model.source_model)) candidate.models.push(model.source_model)
  }
  candidates.value = [...candidates.value, ...orphans.values()]
}

const load = async () => {
  const generation = beginSession()
  const targetID = props.groupId
  reset()
  loading.value = true
  try {
    const [candidateItems, detail] = await Promise.all([
      adminAPI.groups.getSystemCustomGroupCandidates(),
      targetID === null
        ? Promise.resolve(null)
        : adminAPI.groups.getSystemCustomGroup(targetID)
    ])
    if (!currentSession(generation, targetID)) return
    mergeCandidates(candidateItems)
    if (detail) {
      addOrphanCandidates(detail.models)
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
          draftInitialized: true,
          selected: true
        })
      }
      selectedSourceIDs.value = [...sourceIDs]
    }
  } catch (error) {
    if (currentSession(generation, targetID)) {
      errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.loadFailed'))
    }
  } finally {
    if (currentSession(generation, targetID)) loading.value = false
  }
}

watch(
  () => [props.show, props.groupId] as const,
  ([show]) => {
    if (show) void load()
    else beginSession()
  },
  { immediate: true }
)

const isSourceSelected = (sourceID: number) => selectedSourceIDs.value.includes(sourceID)

const setSourceSectionRef = (
  sourceID: number,
  element: Element | ComponentPublicInstance | null
) => {
  if (element instanceof HTMLElement) {
    sourceSectionRefs.set(sourceID, element)
    return
  }
  sourceSectionRefs.delete(sourceID)
}

const scrollToSource = (sourceID: number) => {
  const container = modelScrollRef.value
  const section = sourceSectionRefs.get(sourceID)
  if (!container || !section) return
  const containerRect = container.getBoundingClientRect()
  const sectionRect = section.getBoundingClientRect()
  const reducedMotion =
    typeof window.matchMedia !== 'function' ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  container.scrollTo({
    top: Math.max(0, container.scrollTop + sectionRect.top - containerRect.top),
    behavior: reducedMotion ? 'auto' : 'smooth'
  })
}

const beginDeleteConfirmation = () => {
  if (busy.value) return
  confirmingDelete.value = true
}

const toggleSource = (sourceID: number, selected: boolean) => {
  if (selected) {
    if (!selectedSourceIDs.value.includes(sourceID)) {
      selectedSourceIDs.value = [...selectedSourceIDs.value, sourceID]
    }
    return
  }
  selectedSourceIDs.value = selectedSourceIDs.value.filter((id) => id !== sourceID)
}

const toggleRouteSelected = (route: RouteDraft, selected: boolean) => {
  route.selected = selected
  if (selected) route.draftInitialized = true
}

const selectedCandidates = computed(() =>
  candidates.value.filter((candidate) => isSourceSelected(candidate.group.id))
)

const routesBySource = computed(() => {
  const grouped = new Map<number, RouteDraft[]>()
  for (const route of routes.values()) {
    const sourceRoutes = grouped.get(route.source_group_id) ?? []
    sourceRoutes.push(route)
    grouped.set(route.source_group_id, sourceRoutes)
  }
  return grouped
})

const routesForSource = (sourceID: number) => routesBySource.value.get(sourceID) ?? []

const visibleRoutes = computed(() =>
  selectedCandidates.value.flatMap((candidate) => routesForSource(candidate.group.id))
)

const selectedRoutes = computed(() =>
  [...routes.values()].filter(
    (route) => route.selected && selectedSourceIDs.value.includes(route.source_group_id)
  )
)

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
    const key = display.toLowerCase()
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
    busy.value ||
    confirmingDelete.value ||
    !numbersValid.value ||
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

const validNullableLimit = (value: number | string | null) => {
  if (value === '' || value === null || value === undefined) return true
  if (typeof value === 'string' && value.trim() === '') return false
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0
}

const numbersValid = computed(() => {
  const validity = Number(form.default_validity_days)
  return (
    validNullableLimit(form.daily_limit_usd) &&
    validNullableLimit(form.weekly_limit_usd) &&
    validNullableLimit(form.monthly_limit_usd) &&
    Number.isInteger(validity) &&
    validity >= 1 &&
    validity <= 3650
  )
})

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
  const generation = sessionGeneration
  const targetID = props.groupId
  saving.value = true
  errorMessage.value = ''
  try {
    const request = snapshot()
    if (targetID === null) await adminAPI.groups.createSystemCustomGroup(request)
    else await adminAPI.groups.updateSystemCustomGroup(targetID, request)
    if (currentSession(generation, targetID)) emit('saved')
  } catch (error) {
    if (currentSession(generation, targetID)) {
      errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.saveFailed'))
    }
  } finally {
    if (currentSession(generation, targetID)) saving.value = false
  }
}

const loadSyncPreview = async () => {
  if (props.groupId === null || busy.value) return
  const generation = sessionGeneration
  const targetID = props.groupId
  syncing.value = true
  errorMessage.value = ''
  try {
    const [freshCandidates, preview] = await Promise.all([
      adminAPI.groups.getSystemCustomGroupCandidates(),
      adminAPI.groups.getSystemCustomGroupSyncPreview(targetID)
    ])
    if (!currentSession(generation, targetID)) return
    mergeCandidates(freshCandidates, { replace: true })
    syncPreview.value = preview
  } catch (error) {
    if (currentSession(generation, targetID)) {
      errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.syncFailed'))
    }
  } finally {
    if (currentSession(generation, targetID)) syncing.value = false
  }
}

const routeFor = (sourceGroupID: number, sourceModel: string) =>
  routes.get(routeKey(sourceGroupID, sourceModel))

const isSyncAddedSelected = (added: SystemCustomGroupSyncAdded) =>
  routeFor(added.source_group_id, added.source_model)?.selected === true

const isSyncMissingDisabled = (missing: SystemCustomGroupModel) =>
  routeFor(missing.source_group_id, missing.source_model)?.enabled === false

const isSyncMissingReadOnly = (missing: SystemCustomGroupModel) => {
  const key = routeKey(missing.source_group_id, missing.source_model)
  return isSyncMissingDisabled(missing) && !syncMissingPreviousEnabled.has(key)
}

const toggleSyncAdded = (added: SystemCustomGroupSyncAdded, selected: boolean) => {
  const key = routeKey(added.source_group_id, added.source_model)
  if (!selected) {
    const route = routes.get(key)
    if (route) route.selected = false
    return
  }
  const candidate = candidates.value.find(
    (item) =>
      item.group.id === added.source_group_id &&
      item.models.some((model) => model.toLowerCase() === added.source_model.toLowerCase())
  )
  if (!candidate) {
    errorMessage.value = t('admin.groups.systemCustom.syncSourceUnavailable', {
      model: added.source_model,
      source: added.source_group_id
    })
    return
  }
  errorMessage.value = ''
  if (!selectedSourceIDs.value.includes(added.source_group_id)) {
    selectedSourceIDs.value = [...selectedSourceIDs.value, added.source_group_id]
  }
  const route = ensureRoute(added.source_group_id, added.source_model)
  if (!route.draftInitialized) {
    route.public_model = added.public_model
    route.enabled = true
    route.draftInitialized = true
  }
  route.selected = true
}

const toggleSyncMissing = (missing: SystemCustomGroupModel, disable: boolean) => {
  const key = routeKey(missing.source_group_id, missing.source_model)
  const route = ensureRoute(missing.source_group_id, missing.source_model)
  if (disable) {
    if (!syncMissingPreviousEnabled.has(key)) {
      syncMissingPreviousEnabled.set(key, route.enabled)
    }
    route.enabled = false
    return
  }
  if (syncMissingPreviousEnabled.has(key)) {
    route.enabled = syncMissingPreviousEnabled.get(key) ?? route.enabled
    syncMissingPreviousEnabled.delete(key)
  }
}

const asRecord = (value: unknown): Record<string, unknown> | null =>
  typeof value === 'object' && value !== null ? (value as Record<string, unknown>) : null

const formatMetadata = (metadata: unknown) => {
  if (typeof metadata === 'string') return safeErrorText(metadata)
  const record = asRecord(metadata)
  if (!record) return ''
  return Object.entries(record)
    .map(([key, value]) => {
      if (!/^[a-zA-Z0-9_.-]{1,64}$/.test(key)) return ''
      if (typeof value === 'string') {
        const safe = safeErrorText(value)
        return safe ? `${key}: ${safe}` : ''
      }
      if (typeof value === 'number' || typeof value === 'boolean') {
        return `${key}: ${String(value)}`
      }
      return ''
    })
    .filter(Boolean)
    .join(', ')
}

const unsafeErrorText = (value: string) => {
  const normalized = value.trim().toLowerCase()
  return (
    normalized === '' ||
    normalized === 'internal error' ||
    /<\/?[a-z][\s\S]*>/i.test(value) ||
    /^(?:(?:4\d\d|5\d\d)\s+)?(?:bad gateway|gateway timeout|internal server error|service unavailable)$/i.test(
      value.trim()
    )
  )
}

const safeErrorText = (value: string) => {
  const trimmed = value.trim()
  return unsafeErrorText(trimmed) || trimmed.length > 500 ? '' : trimmed
}

const formatApiError = (error: unknown, fallback: string) => {
  const errorRecord = asRecord(error)
  const response = asRecord(errorRecord?.response)
  const rawData = response?.data
  if (typeof rawData === 'string') return safeErrorText(rawData) || fallback
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
      if (typeof value === 'string') {
        const safe = safeErrorText(value)
        if (safe && !parts.includes(safe)) parts.push(safe)
      }
    }
  }
  for (const source of sources) {
    const value = formatMetadata(source.metadata)
    if (value && !parts.includes(value)) parts.push(value)
  }
  return parts.join(' · ') || fallback
}

const deleteGroup = async () => {
  if (props.groupId === null || busy.value) return
  const generation = sessionGeneration
  const targetID = props.groupId
  deleting.value = true
  errorMessage.value = ''
  try {
    await adminAPI.groups.deleteSystemCustomGroup(targetID)
    if (currentSession(generation, targetID)) {
      confirmingDelete.value = false
      emit('deleted')
    }
  } catch (error) {
    if (currentSession(generation, targetID)) {
      errorMessage.value = formatApiError(error, t('admin.groups.systemCustom.deleteFailed'))
    }
  } finally {
    if (currentSession(generation, targetID)) deleting.value = false
  }
}

const requestClose = () => {
  if (busy.value) return
  beginSession()
  emit('close')
}
</script>
