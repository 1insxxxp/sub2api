<template>
  <BaseDialog :show="show" :title="t('categoryDefaults.title')" width="wide" @close="requestClose">
    <div class="category-default-dialog space-y-4" :aria-busy="loading || busy">
      <p class="text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('categoryDefaults.description') }}</p>

      <div class="flex flex-wrap gap-1.5" role="group" :aria-label="t('categoryDefaults.title')">
        <button
          v-for="item in CATEGORY_DEFAULT_CATEGORIES"
          :key="item"
          type="button"
          data-testid="category-default-category"
          :data-category="item"
          :aria-pressed="category === item"
          :disabled="busy"
          class="category-default-tab"
          :class="{ 'is-active': category === item }"
          @click="category = item"
        >{{ t(`keys.providers.${item}`) }}</button>
      </div>

      <div v-if="errorMessage" role="alert" class="rounded-md border border-red-200 px-3 py-2 text-sm leading-6 text-red-700 dark:border-red-900/70 dark:text-red-300">{{ errorMessage }}</div>

      <div class="flex min-h-11 flex-wrap items-center justify-between gap-2 border-y border-gray-200/70 py-2 dark:border-dark-600">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
          <template v-if="config?.group_id">
            <span class="font-medium text-gray-800 dark:text-gray-200">{{ t('categoryDefaults.published', { count: config.models_count }) }}</span>
            <span>{{ t('categoryDefaults.revision', { revision: config.revision }) }}</span>
          </template>
          <span v-else>{{ t('categoryDefaults.notConfigured') }}</span>
        </div>
        <button
          v-if="config?.group_id"
          type="button"
          role="switch"
          data-testid="category-default-status"
          :aria-checked="config.enabled"
          :aria-label="t('categoryDefaults.statusLabel')"
          :disabled="loading || busy"
          class="category-default-status"
          @click="changeStatus"
        >
          <span class="h-1.5 w-1.5 rounded-full" :class="config.enabled ? 'bg-emerald-500' : 'bg-gray-400'" />
          {{ t(config.enabled ? 'categoryDefaults.enabled' : 'categoryDefaults.disabled') }}
        </button>
      </div>

      <div v-if="config?.conflicts?.length" data-testid="category-default-live-conflicts" class="rounded-md border border-amber-200 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-800/70 dark:text-amber-300" role="status">
        <p>{{ t('categoryDefaults.liveConflictHint') }}</p>
        <ul class="mt-1 space-y-1">
          <li v-for="conflict in config.conflicts" :key="conflict.public_model" class="break-words">
            <strong class="font-mono font-medium">{{ conflict.public_model }}</strong> · {{ conflictSources(conflict).join('、') }}
          </li>
        </ul>
      </div>
      <div v-if="config?.unavailable_sources?.length" data-testid="category-default-unavailable-sources" class="text-xs leading-5 text-amber-700 dark:text-amber-400" role="status">
        <p>{{ t('categoryDefaults.unavailableSourcesHint') }}</p>
        <ul>
          <li v-for="source in config.unavailable_sources" :key="source.source_group_id">
            {{ sourceName(source.source_group_id) }} · {{ t(source.reason === 'unbounded' ? 'categoryDefaults.invalidSource' : 'categoryDefaults.sourceUnavailable') }}
          </li>
        </ul>
      </div>

      <section>
        <div class="mb-2 flex items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('categoryDefaults.sources') }}</h3>
          <span class="text-xs tabular-nums text-gray-500 dark:text-gray-400">{{ t('categoryDefaults.sourceCount', { count: selectedIds.length }) }}</span>
        </div>
        <p class="mb-3 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('categoryDefaults.sourceHint') }}</p>
        <div class="relative mb-2">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
          <input v-model="search" type="search" class="input pl-9" :placeholder="t('categoryDefaults.search')" :aria-label="t('categoryDefaults.search')" />
        </div>
        <div v-if="loading" class="py-8 text-center text-sm text-gray-400" role="status">{{ t('common.loading') }}</div>
        <div v-else class="grid max-h-72 grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2">
          <label v-for="candidate in filteredCandidates" :key="candidate.group.id" class="category-default-source" :class="{ 'is-selected': selectedIds.includes(candidate.group.id), 'is-disabled': !!candidate.invalid_reason && !selectedIds.includes(candidate.group.id) }">
            <input
              v-model="selectedIds"
              type="checkbox"
              :value="candidate.group.id"
              :disabled="busy || (!!candidate.invalid_reason && !selectedIds.includes(candidate.group.id))"
              data-testid="category-default-source"
              :data-source-id="candidate.group.id"
              class="mt-1 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
            />
            <span class="min-w-0 flex-1">
              <span class="block break-words text-[13px] font-medium leading-5 text-gray-900 dark:text-gray-100">{{ candidate.group.name }}</span>
              <span class="mt-0.5 block text-xs leading-5 text-gray-500 dark:text-gray-400">{{ candidate.invalid_reason || t('categoryDefaults.availableModels', { count: candidate.models_count }) }}</span>
            </span>
          </label>
          <p v-if="filteredCandidates.length === 0" class="col-span-full py-6 text-center text-sm text-gray-400">{{ t('categoryDefaults.noSources') }}</p>
        </div>
        <div v-if="missingSourceIds.length" class="mt-2 space-y-2 text-xs leading-5 text-amber-700 dark:text-amber-400">
          <p>{{ t('categoryDefaults.missingSources') }}</p>
          <p v-for="id in missingSourceIds" :key="id">{{ sourceName(id) }}</p>
        </div>
      </section>

      <section v-if="preview" data-testid="category-default-preview-result" class="border-t border-gray-200/70 pt-4 dark:border-dark-600" aria-live="polite">
        <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-gray-100">{{ t('categoryDefaults.previewTitle') }}</h3>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
          <div v-for="kind in changeKinds" :key="kind" class="category-default-count">
            <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t(`categoryDefaults.${kind}`) }}</span>
            <strong class="mt-1 block text-lg font-semibold tabular-nums text-gray-900 dark:text-gray-100">{{ preview[kind].length }}</strong>
          </div>
        </div>
        <div v-if="preview.conflicts.length" data-testid="category-default-conflicts" class="mt-3 rounded-md border border-red-200 p-3 dark:border-red-900/70" role="alert">
          <p class="text-xs leading-5 text-red-700 dark:text-red-300">{{ t('categoryDefaults.conflictHint') }}</p>
          <ul class="mt-2 space-y-2 text-xs">
            <li v-for="conflict in preview.conflicts" :key="conflict.public_model" class="break-words">
              <strong class="font-mono font-medium text-red-800 dark:text-red-200">{{ conflict.public_model }}</strong>
              <span class="mt-0.5 block text-gray-500 dark:text-gray-400">{{ conflictSources(conflict).join('、') }}</span>
            </li>
          </ul>
        </div>
        <details v-for="kind in changedKinds" :key="kind" class="mt-2 text-xs">
          <summary class="cursor-pointer py-2 text-gray-600 dark:text-gray-300">{{ t(`categoryDefaults.${kind}`) }} · {{ preview[kind].length }}</summary>
          <ul class="max-h-40 space-y-1 overflow-y-auto rounded-md bg-gray-50/80 px-3 py-2 dark:bg-black/10">
            <li v-for="model in preview[kind]" :key="`${model.source_group_id}:${model.public_model}`" class="flex flex-wrap justify-between gap-x-3 gap-y-0.5 py-1">
              <span class="min-w-0 break-words font-mono text-gray-900 dark:text-gray-100">{{ model.public_model }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ sourceName(model.source_group_id) }}</span>
            </li>
          </ul>
        </details>
      </section>
      <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('categoryDefaults.previewHint') }}</p>
    </div>

    <template #footer>
      <div class="flex flex-wrap justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="requestClose">{{ t('common.cancel') }}</button>
        <button type="button" class="btn btn-secondary" data-testid="category-default-preview" :disabled="loading || busy || selectedIds.length === 0" @click="buildPreview">{{ t('categoryDefaults.previewAction') }}</button>
        <button type="button" class="btn btn-primary" data-testid="category-default-publish" :disabled="!canPublish" @click="publish">{{ t(publishing ? 'categoryDefaults.publishing' : 'categoryDefaults.publishAction') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { categoryDefaultGroupsAPI, type CategoryDefaultCandidate, type CategoryDefaultConfig, type CategoryDefaultConflict, type CategoryDefaultPreview } from '@/api/admin/categoryDefaultGroups'
import { CATEGORY_DEFAULT_CATEGORIES, type CategoryDefaultCategory } from '@/utils/categoryDefaultGroups'

const props = defineProps<{ show: boolean; initialCategory?: CategoryDefaultCategory }>()
const emit = defineEmits<{ close: []; saved: [config: CategoryDefaultConfig] }>()
const { t } = useI18n()
const category = ref<CategoryDefaultCategory>(props.initialCategory ?? 'anthropic')
const configs = ref<CategoryDefaultConfig[]>([])
const candidates = ref<CategoryDefaultCandidate[]>([])
const selectedIds = ref<number[]>([])
const search = ref('')
const loading = ref(false)
const busy = ref(false)
const publishing = ref(false)
const errorMessage = ref('')
const preview = ref<CategoryDefaultPreview | null>(null)
const changeKinds = ['added', 'modified', 'removed', 'unchanged'] as const
const changedKinds = computed(() => changeKinds.filter(kind => kind !== 'unchanged' && (preview.value?.[kind].length ?? 0) > 0))
const config = computed(() => configs.value.find(item => item.category === category.value))
const canPublish = computed(() => !loading.value && !busy.value && !!preview.value && preview.value.conflicts.length === 0 && selectedIds.value.length > 0)
const filteredCandidates = computed(() => candidates.value.filter(item => item.group.name.toLowerCase().includes(search.value.trim().toLowerCase())))
const missingSourceIds = computed(() => (config.value?.source_group_ids ?? []).filter(id => !candidates.value.some(item => item.group.id === id)))
let loadGeneration = 0

function errorText(error: unknown, fallback: string): string {
  const e = error as { response?: { data?: { message?: string; detail?: string } }; message?: string }
  return e?.response?.data?.detail || e?.response?.data?.message || e?.message || t(fallback)
}
function sourceName(id: number) { return candidates.value.find(item => item.group.id === id)?.group.name ?? `#${id}` }
function conflictSources(conflict: CategoryDefaultConflict) { return conflict.source_group_names?.length ? conflict.source_group_names : conflict.source_group_ids.map(sourceName) }
function updateConfig(value: CategoryDefaultConfig) {
  configs.value = [...configs.value.filter(item => item.category !== value.category), value]
}
function requestClose() { if (!busy.value) emit('close') }

async function load() {
  const generation = ++loadGeneration
  preview.value = null
  errorMessage.value = ''
  search.value = ''
  candidates.value = []
  selectedIds.value = []
  if (!props.show) { loading.value = false; return }
  loading.value = true
  const selectedCategory = category.value
  try {
    const [all, sources] = await Promise.all([categoryDefaultGroupsAPI.list(), categoryDefaultGroupsAPI.candidates(selectedCategory)])
    if (generation !== loadGeneration || !props.show) return
    configs.value = all ?? []
    candidates.value = sources ?? []
    selectedIds.value = (config.value?.source_group_ids ?? []).filter(id => candidates.value.some(item => item.group.id === id))
  } catch (error) {
    if (generation === loadGeneration) errorMessage.value = errorText(error, 'categoryDefaults.loadFailed')
  } finally { if (generation === loadGeneration) loading.value = false }
}

async function buildPreview() {
  if (busy.value || loading.value || !selectedIds.value.length) return
  busy.value = true
  errorMessage.value = ''
  preview.value = null
  try {
    candidates.value = await categoryDefaultGroupsAPI.candidates(category.value) ?? []
    selectedIds.value = selectedIds.value.filter(id => candidates.value.some(item => item.group.id === id))
    if (!selectedIds.value.length) {
      errorMessage.value = t('categoryDefaults.selectionUnavailable')
      return
    }
    const result = await categoryDefaultGroupsAPI.preview(category.value, [...selectedIds.value])
    preview.value = { ...result, added: result.added ?? [], modified: result.modified ?? [], removed: result.removed ?? [], unchanged: result.unchanged ?? [], conflicts: result.conflicts ?? [] }
  } catch (error) { errorMessage.value = errorText(error, 'categoryDefaults.previewFailed') }
  finally { busy.value = false }
}

async function publish() {
  if (!canPublish.value || !preview.value) return
  busy.value = true
  publishing.value = true
  errorMessage.value = ''
  try {
    const saved = await categoryDefaultGroupsAPI.apply(category.value, [...selectedIds.value], preview.value)
    updateConfig(saved)
    emit('saved', saved)
  } catch (error) { errorMessage.value = errorText(error, 'categoryDefaults.publishFailed') }
  finally { preview.value = null; busy.value = false; publishing.value = false }
}

async function changeStatus() {
  if (!config.value?.group_id || busy.value || loading.value) return
  busy.value = true
  errorMessage.value = ''
  preview.value = null
  try {
    const saved = await categoryDefaultGroupsAPI.setStatus(category.value, !config.value.enabled)
    updateConfig(saved)
    emit('saved', saved)
  } catch (error) { errorMessage.value = errorText(error, 'categoryDefaults.statusFailed') }
  finally { busy.value = false }
}

watch(selectedIds, () => { preview.value = null }, { deep: true, flush: 'sync' })
watch(() => props.initialCategory, value => { if (value) category.value = value })
watch([() => props.show, category], load, { immediate: true })
</script>

<style scoped>
.category-default-tab {
  min-height: 2.75rem;
  padding: 0.5rem 0.85rem;
  border: 1px solid rgb(148 163 184 / 20%);
  border-radius: 6px;
  color: var(--workspace-text-secondary, #64748b);
  font-size: 0.8125rem;
  transition: color 150ms, border-color 150ms, background-color 150ms;
}
.category-default-tab.is-active { color: rgb(var(--brand-rgb) / 1); border-color: rgb(var(--brand-rgb) / 45%); background: rgb(var(--brand-rgb) / 6%); }
.category-default-status { display: inline-flex; min-height: 2.75rem; align-items: center; gap: 0.5rem; padding: 0 0.5rem; font-size: 0.75rem; color: #64748b; }
.category-default-source { display: flex; min-width: 0; align-items: flex-start; gap: 0.65rem; padding: 0.7rem 0.8rem; border: 1px solid rgb(148 163 184 / 20%); border-radius: 6px; background: linear-gradient(130deg, rgb(255 255 255 / 92%), rgb(255 255 255 / 55%)); box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 4px rgb(15 23 42 / 3%); cursor: pointer; }
.category-default-source.is-selected { border-color: rgb(var(--brand-rgb) / 50%); }
.category-default-source.is-disabled { cursor: not-allowed; opacity: 0.55; }
.category-default-count { border-radius: 6px; padding: 0.65rem 0.75rem; background: rgb(148 163 184 / 6%); }
.dark .category-default-source { background: linear-gradient(130deg, rgb(255 255 255 / 4%), rgb(255 255 255 / 1%)); border-color: rgb(255 255 255 / 11%); box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%); }
.dark .category-default-source.is-selected { border-color: rgb(var(--brand-rgb) / 60%); }
.dark .category-default-status { color: #94a3b8; }
.category-default-tab:focus-visible, .category-default-status:focus-visible { outline: 2px solid rgb(var(--brand-rgb) / 70%); outline-offset: 2px; }
@media (prefers-reduced-motion: reduce) { .category-default-tab { transition: none; } }
</style>
