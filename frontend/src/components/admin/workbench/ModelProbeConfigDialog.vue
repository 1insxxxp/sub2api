<template>
  <BaseDialog :show="!closed" :title="t('modelProbes.config.title')" width="normal" appearance="neutral" :close-on-escape="!saving" :show-close-button="!saving" @close="close">
    <div class="probe-config min-w-0" data-test="probe-config-dialog">
      <p class="config-description">{{ t('modelProbes.config.description') }}</p>
      <label for="probe-config-group" class="input-label">{{ t('modelProbes.config.group') }}</label>
      <Select id="probe-config-group" v-model="selectedGroup" data-test="probe-config-group" :options="groupOptions" :searchable="true" :mobile-sheet="false"
        :placeholder="t('modelProbes.selectGroup')" :search-placeholder="t('modelProbes.searchGroups')" :aria-label="t('modelProbes.config.group')" :disabled="saving || loadingGroups || forbidden" />
      <p class="config-scope">{{ t('modelProbes.config.scopeHint') }}</p>
      <p v-if="errorKey" role="alert" class="config-error">{{ t(errorKey) }}<button v-if="!forbidden && !saving" type="button" data-test="probe-config-reload" class="min-h-11 px-2 underline underline-offset-4" @click="reload">{{ t('modelProbes.reload') }}</button></p>
      <p v-if="loadingGroups || loadingModels" role="status" class="config-empty">{{ t('modelProbes.loading') }}</p>
      <template v-else-if="loaded">
        <div class="config-tools">
          <div class="config-search">
            <Icon name="search" size="sm" class="config-search-icon" />
            <input v-model="query" data-test="probe-config-search" type="search" class="input min-h-11 w-full pl-9 text-sm" :placeholder="t('modelProbes.config.searchModels')" :aria-label="t('modelProbes.config.searchModels')" :disabled="saving || forbidden" />
          </div>
          <button type="button" data-test="probe-config-select-visible" class="config-text-button min-h-11" :disabled="saving || forbidden || !visibleModels.length" @click="selectVisible">{{ t('modelProbes.config.selectVisible') }}</button>
          <button type="button" data-test="probe-config-clear" class="config-text-button min-h-11" :disabled="saving || forbidden || !selectedModels.length" @click="clear">{{ t('modelProbes.config.clear') }}</button>
        </div>
        <div class="config-summary" role="status" aria-live="polite"><span>{{ t('modelProbes.config.selected', { count: selectedModels.length, max: MAX_MODELS }) }}</span><span>{{ t('modelProbes.config.available', { count: candidates.length }) }}</span></div>
        <p v-if="selectionError" role="alert" class="config-error">{{ t(selectionError) }}</p>
        <div v-if="visibleModels.length" class="config-models" :aria-label="t('modelProbes.models')">
          <label v-for="model in visibleModels" :key="model.id" class="config-model min-h-11" :class="{ 'config-model-selected': selectedSet.has(model.id) }">
            <input type="checkbox" data-test="probe-config-model" class="h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :checked="selectedSet.has(model.id)" :aria-label="model.id" :disabled="saving || forbidden || (!selectedSet.has(model.id) && (!candidateIds.has(model.id) || selectedModels.length >= MAX_MODELS))" @change="toggleModel(model.id)" />
            <span class="config-model-name"><span>{{ model.display_name || model.id }}</span><small v-if="model.display_name && model.display_name !== model.id">{{ model.id }}</small></span>
            <small v-if="!candidateIds.has(model.id)" class="config-unavailable">{{ t('modelProbes.config.unavailable') }}</small>
          </label>
        </div>
        <p v-else class="config-empty">{{ t(query ? 'modelProbes.config.noMatchingModels' : 'modelProbes.config.noAvailableModels') }}</p>
        <p class="config-footer-note" :class="{ 'config-empty-selection': !selectedModels.length }">{{ t(invalidSelection ? 'modelProbes.config.staleHint' : selectedModels.length ? 'modelProbes.config.saveHint' : 'modelProbes.config.emptyHint') }}</p>
      </template>
      <p v-else-if="!loadingGroups && !loadingModels && !errorKey" class="config-empty">{{ t(groups.length ? 'modelProbes.config.initial' : 'modelProbes.config.noGroups') }}</p>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary min-h-11" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button type="button" data-test="probe-config-save" class="btn btn-primary min-h-11" :disabled="!canSave" @click="save">{{ t(saving ? 'modelProbes.config.saving' : 'modelProbes.config.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { getGroupConfig, listConfigGroups, saveGroupConfig, type ProbeGroup, type ProbeModel } from '@/api/admin/modelProbes'

const props = defineProps<{ initialGroupId?: number | null }>()
const emit = defineEmits<{ close: []; saved: [groupId: number] }>()
const { t } = useI18n()
const auth = useAuthStore()
const MAX_MODELS = 200
const closed = ref(false)
const selectedGroup = ref<string | number | boolean | null>(null)
const groups = ref<ProbeGroup[]>([])
const candidates = ref<ProbeModel[]>([])
const loadedModels = ref<ProbeModel[]>([])
const selectedModels = ref<string[]>([])
const query = ref('')
const loaded = ref(false)
const loadingGroups = ref(false)
const loadingModels = ref(false)
const saving = ref(false)
const forbidden = ref(false)
const errorKey = ref('')
const selectionError = ref('')
let generation = 0
let metadataController: AbortController | null = null
let saveController: AbortController | null = null
const identity = () => `${auth.user?.id}:${auth.user?.role}:${auth.token ?? ''}`
const actor = identity()
const groupOptions = computed(() => groups.value.map(group => ({ value: group.id, label: group.name })))
const selectedSet = computed(() => new Set(selectedModels.value))
const candidateIds = computed(() => new Set(candidates.value.map(model => model.id)))
const invalidSelection = computed(() => selectedModels.value.some(model => !candidateIds.value.has(model)))
const visibleModels = computed(() => {
  const search = query.value.trim().toLowerCase()
  return loadedModels.value.filter(model => !search || model.id.toLowerCase().includes(search) || model.display_name?.toLowerCase().includes(search))
})
const canSave = computed(() => loaded.value && !loadingGroups.value && !loadingModels.value && !saving.value && !forbidden.value && !closed.value && !invalidSelection.value && !selectionError.value && selectedModels.value.length <= MAX_MODELS)

function cancel() {
  generation++
  metadataController?.abort()
  saveController?.abort()
}
function close() {
  if (closed.value) return
  closed.value = true
  cancel()
  emit('close')
}
function current(version: number, controller: AbortController) {
  return version === generation && !controller.signal.aborted && !closed.value && actor === identity() && auth.user?.role === 'admin'
}
function setError(error: unknown, fallback: string) {
  const failure = error as { status?: number; response?: { status?: number } } | null
  const status = failure?.status ?? failure?.response?.status
  forbidden.value = status === 401 || status === 403
  errorKey.value = forbidden.value ? 'modelProbes.config.errors.forbidden' : fallback
}
async function loadGroups() {
  if (auth.user?.role !== 'admin') { close(); return }
  const version = ++generation
  metadataController?.abort()
  const controller = new AbortController()
  metadataController = controller
  loadingGroups.value = true
  errorKey.value = ''
  try {
    const data = await listConfigGroups(controller.signal)
    if (!current(version, controller)) return
    groups.value = data.items
    if (props.initialGroupId && groups.value.some(group => group.id === props.initialGroupId)) selectedGroup.value = props.initialGroupId
  } catch (error) {
    if (current(version, controller)) setError(error, 'modelProbes.config.errors.groups')
  } finally {
    if (current(version, controller)) loadingGroups.value = false
  }
}
async function loadGroup(value: typeof selectedGroup.value) {
  const version = ++generation
  metadataController?.abort()
  loaded.value = false
  loadingModels.value = false
  candidates.value = []
  loadedModels.value = []
  selectedModels.value = []
  query.value = ''
  errorKey.value = ''
  selectionError.value = ''
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value <= 0 || forbidden.value || closed.value) return
  const controller = new AbortController()
  metadataController = controller
  loadingModels.value = true
  try {
    const data = await getGroupConfig(value, controller.signal)
    if (!current(version, controller)) return
    if (data.group.id !== value) { errorKey.value = 'modelProbes.config.errors.models'; return }
    candidates.value = data.available_models
    selectedModels.value = [...new Set(data.models)]
    const ids = new Set(data.available_models.map(model => model.id))
    loadedModels.value = [...data.available_models, ...selectedModels.value.filter(id => !ids.has(id)).map(id => ({ id }))]
    loaded.value = true
  } catch (error) {
    if (current(version, controller)) setError(error, 'modelProbes.config.errors.models')
  } finally {
    if (current(version, controller)) loadingModels.value = false
  }
}
function reload() {
  if (typeof selectedGroup.value === 'number') void loadGroup(selectedGroup.value)
  else void loadGroups()
}
function toggleModel(id: string) {
  if (!loaded.value || saving.value || forbidden.value) return
  const next = selectedModels.value.filter(model => model !== id)
  if (next.length === selectedModels.value.length) {
    if (!candidateIds.value.has(id) || next.length >= MAX_MODELS) return
    next.push(id)
  }
  selectedModels.value = next
  selectionError.value = ''
}
function selectVisible() {
  if (!loaded.value || saving.value || forbidden.value) return
  const next = [...new Set([...selectedModels.value, ...visibleModels.value.filter(model => candidateIds.value.has(model.id)).map(model => model.id)])]
  if (next.length > MAX_MODELS) { selectionError.value = 'modelProbes.config.errors.limit'; return }
  selectedModels.value = next
  selectionError.value = ''
}
function clear() { selectedModels.value = []; selectionError.value = '' }
async function save() {
  if (!canSave.value || typeof selectedGroup.value !== 'number') return
  const groupId = selectedGroup.value
  const models = [...selectedModels.value]
  const version = ++generation
  const controller = new AbortController()
  saveController = controller
  saving.value = true
  errorKey.value = ''
  try {
    const data = await saveGroupConfig(groupId, models, controller.signal)
    if (!current(version, controller)) return
    if (data.group_id !== groupId) { errorKey.value = 'modelProbes.config.errors.save'; return }
    closed.value = true
    emit('saved', groupId)
  } catch (error) {
    if (current(version, controller)) setError(error, 'modelProbes.config.errors.save')
  } finally {
    if (version === generation && !controller.signal.aborted) saving.value = false
  }
}
watch(selectedGroup, loadGroup)
watch(identity, close)
onBeforeUnmount(cancel)
void loadGroups()
</script>

<style scoped>
.probe-config { color: var(--workspace-ink); }
.config-description, .config-scope, .config-footer-note { color: var(--workspace-muted); font-size: 12px; line-height: 1.65; }
.config-description { margin: 0 0 16px; }
.config-scope { margin: 7px 0 16px; font-size: 11px; }
.config-tools { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; }
.config-search { position: relative; flex: 1 1 100%; }
.config-search-icon { position: absolute; left: 12px; top: 50%; transform: translateY(-50%); color: var(--workspace-muted); pointer-events: none; }
.config-text-button { font-size: 12px; color: rgb(var(--brand-rgb)); transition: opacity 120ms ease; }
.config-text-button:disabled { opacity: .45; }
.config-summary { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 6px; margin: 2px 0 9px; color: var(--workspace-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.config-models { max-height: min(340px, 40dvh); overflow-y: auto; overscroll-behavior: contain; border: 1px solid var(--workspace-rule); border-radius: 7px; background: linear-gradient(125deg, var(--workspace-highlight), transparent 65%), var(--workspace-surface); box-shadow: inset 0 1px 0 var(--workspace-highlight); }
.config-model { display: flex; align-items: center; gap: 10px; padding: 9px 11px; cursor: pointer; transition: background-color 120ms ease; }
.config-model + .config-model { border-top: 1px solid var(--workspace-rule); }
.config-model:hover, .config-model-selected { background: rgb(var(--brand-rgb) / .035); }
.config-model-name { display: grid; min-width: 0; gap: 2px; font-size: 12px; overflow-wrap: anywhere; }
.config-model-name small, .config-unavailable { font-size: 10px; color: var(--workspace-muted); }
.config-unavailable { margin-left: auto; flex-shrink: 0; }
.config-footer-note { margin: 11px 0 0; font-size: 11px; }
.config-empty-selection { color: var(--workspace-ink); }
.config-error { margin: 8px 0; font-size: 12px; line-height: 1.6; color: rgb(220 38 38); overflow-wrap: anywhere; }
.config-empty { padding: 24px 10px; text-align: center; font-size: 12px; color: var(--workspace-muted); }
@media (prefers-reduced-motion: reduce) { .config-model, .config-text-button { transition: none; } }
</style>
