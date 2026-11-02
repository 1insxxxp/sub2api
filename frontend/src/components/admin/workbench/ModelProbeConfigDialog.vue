<template>
  <BaseDialog :show="!closed" :title="t('modelProbes.config.title')" width="normal" appearance="neutral" :close-on-escape="!saving" :show-close-button="!saving" @close="close">
    <div class="probe-config min-w-0" data-test="probe-config-dialog">
      <p class="config-description">{{ t('modelProbes.config.description') }}</p>
      <div class="config-search">
        <Icon name="search" size="sm" class="config-search-icon" />
        <input v-model="query" data-test="probe-config-search" type="search" class="input min-h-11 w-full pl-9 text-sm" :placeholder="t('modelProbes.searchGroups')" :aria-label="t('modelProbes.searchGroups')" :disabled="saving || loading || forbidden" />
      </div>
      <p class="config-scope">{{ t('modelProbes.config.scopeHint') }}</p>
      <p v-if="errorKey" role="alert" class="config-error">{{ t(errorKey) }}<button v-if="!forbidden && !saving" type="button" data-test="probe-config-reload" class="min-h-11 px-2 underline underline-offset-4" @click="loadGroups">{{ t('modelProbes.reload') }}</button></p>
      <p v-if="loading" role="status" class="config-empty">{{ t('modelProbes.loading') }}</p>
      <template v-else-if="loaded">
        <div class="config-tools">
          <span class="config-summary" role="status" aria-live="polite">{{ t('modelProbes.config.selected', { count: selectedIds.length, total: groups.length }) }}</span>
          <button type="button" data-test="probe-config-select-visible" class="config-text-button min-h-11" :disabled="saving || forbidden || !visibleGroups.length" @click="selectVisible">{{ t('modelProbes.config.selectVisible') }}</button>
          <button type="button" data-test="probe-config-clear" class="config-text-button min-h-11" :disabled="saving || forbidden || !selectedIds.length" @click="clear">{{ t('modelProbes.config.clear') }}</button>
        </div>
        <p v-if="changes.length > MAX_CHANGES" role="alert" class="config-error">{{ t('modelProbes.config.errors.limit', { max: MAX_CHANGES }) }}</p>
        <div v-if="visibleGroups.length" class="config-groups" :aria-label="t('modelProbes.config.groups')">
          <label v-for="group in visibleGroups" :key="group.id" class="config-group min-h-11" :class="{ 'config-group-selected': selectedSet.has(group.id) }">
            <input type="checkbox" data-test="probe-config-group" :data-group-id="group.id" class="h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :checked="selectedSet.has(group.id)" :aria-label="group.name" :disabled="saving || forbidden" @change="toggleGroup(group.id)" />
            <span class="config-group-name">{{ group.name }}</span>
            <span class="config-group-platform">{{ group.platform }}</span>
          </label>
        </div>
        <p v-else class="config-empty">{{ t(query ? 'modelProbes.config.noMatchingGroups' : 'modelProbes.config.noGroups') }}</p>
        <p class="config-footer-note" :class="{ 'config-empty-selection': !selectedIds.length }">{{ t(selectedIds.length ? 'modelProbes.config.saveHint' : 'modelProbes.config.emptyHint', { count: changes.length }) }}</p>
      </template>
    </div>
    <template #footer>
      <button type="button" data-test="probe-config-cancel" class="btn btn-secondary min-h-11" :disabled="saving" @click="close">{{ t('common.cancel') }}</button>
      <button type="button" data-test="probe-config-save" class="btn btn-primary min-h-11" :disabled="!canSave" @click="save">{{ t(saving ? 'modelProbes.config.saving' : 'modelProbes.config.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { listConfigGroups, saveConfigGroups, type ProbeConfigGroup, type ProbeGroupChange } from '@/api/admin/modelProbes'

const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const MAX_CHANGES = 500
const closed = ref(false)
const groups = ref<ProbeConfigGroup[]>([])
const selectedIds = ref<number[]>([])
const query = ref('')
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const forbidden = ref(false)
const errorKey = ref('')
let generation = 0
let metadataController: AbortController | null = null
let saveController: AbortController | null = null
const identity = () => `${auth.user?.id}:${auth.user?.role}:${auth.token ?? ''}`
const actor = identity()
const selectedSet = computed(() => new Set(selectedIds.value))
const visibleGroups = computed(() => {
  const search = query.value.trim().toLowerCase()
  return groups.value.filter(group => !search || group.name.toLowerCase().includes(search))
})
const changes = computed<ProbeGroupChange[]>(() => groups.value
  .filter(group => selectedSet.value.has(group.id) !== group.enabled)
  .map(group => ({ group_id: group.id, enabled: selectedSet.value.has(group.id) })))
const canSave = computed(() => loaded.value && !loading.value && !saving.value && !forbidden.value && !closed.value && changes.value.length > 0 && changes.value.length <= MAX_CHANGES)

function cancel() {
  generation++
  metadataController?.abort()
  saveController?.abort()
}
function terminate() {
  if (closed.value) return
  closed.value = true
  cancel()
  emit('close')
}
function close() { if (!saving.value) terminate() }
function current(version: number, controller: AbortController) {
  return version === generation && !controller.signal.aborted && !closed.value && actor === identity() && auth.user?.role === 'admin'
}
function setError(error: unknown, fallback: string) {
  const failure = error as { status?: number; response?: { status?: number } } | null
  const status = failure?.status ?? failure?.response?.status
  forbidden.value = status === 401 || status === 403
  errorKey.value = forbidden.value ? 'modelProbes.config.errors.forbidden' : fallback
}
function isConfigSnapshot(value: unknown): value is ProbeConfigGroup[] {
  if (!Array.isArray(value)) return false
  const ids = new Set<number>()
  return value.every(group => {
    if (!group || typeof group !== 'object' || !Number.isSafeInteger(group.id) || group.id <= 0 || ids.has(group.id) || typeof group.enabled !== 'boolean' || typeof group.name !== 'string' || typeof group.platform !== 'string') return false
    ids.add(group.id)
    return true
  })
}
async function loadGroups() {
  if (saving.value || closed.value || forbidden.value) return
  if (auth.user?.role !== 'admin') { terminate(); return }
  const version = ++generation
  metadataController?.abort()
  const controller = new AbortController()
  metadataController = controller
  loading.value = true
  loaded.value = false
  errorKey.value = ''
  try {
    const data = await listConfigGroups(controller.signal)
    if (!current(version, controller)) return
    if (!isConfigSnapshot(data?.items)) { errorKey.value = 'modelProbes.config.errors.groups'; return }
    groups.value = data.items
    selectedIds.value = data.items.filter(group => group.enabled).map(group => group.id)
    loaded.value = true
  } catch (error) {
    if (current(version, controller)) setError(error, 'modelProbes.config.errors.groups')
  } finally {
    if (current(version, controller)) loading.value = false
  }
}
function toggleGroup(id: number) {
  if (!loaded.value || saving.value || forbidden.value) return
  selectedIds.value = selectedSet.value.has(id) ? selectedIds.value.filter(value => value !== id) : [...selectedIds.value, id]
}
function selectVisible() {
  if (!loaded.value || saving.value || forbidden.value) return
  selectedIds.value = [...new Set([...selectedIds.value, ...visibleGroups.value.map(group => group.id)])]
}
function clear() { if (loaded.value && !saving.value && !forbidden.value) selectedIds.value = [] }
function confirmsChanges(items: ProbeGroupChange[], requested: ProbeGroupChange[]) {
  if (!Array.isArray(items) || items.length !== requested.length) return false
  const expected = new Map(requested.map(group => [group.group_id, group.enabled]))
  const seen = new Set<number>()
  return items.every(group => {
    if (!group || !expected.has(group.group_id) || seen.has(group.group_id) || typeof group.enabled !== 'boolean' || expected.get(group.group_id) !== group.enabled) return false
    seen.add(group.group_id)
    return true
  })
}
async function save() {
  if (!canSave.value) return
  const requested = changes.value.map(group => ({ ...group }))
  const version = ++generation
  const controller = new AbortController()
  saveController = controller
  saving.value = true
  errorKey.value = ''
  try {
    const data = await saveConfigGroups(requested, controller.signal)
    if (!current(version, controller)) return
    if (!confirmsChanges(data.items, requested)) { errorKey.value = 'modelProbes.config.errors.save'; return }
    closed.value = true
    emit('saved')
  } catch (error) {
    if (current(version, controller)) setError(error, 'modelProbes.config.errors.save')
  } finally {
    if (version === generation && !controller.signal.aborted) saving.value = false
  }
}
watch(identity, terminate)
onBeforeUnmount(cancel)
void loadGroups()
</script>

<style scoped>
.probe-config { color: var(--workspace-ink); }
.config-description, .config-scope, .config-footer-note { color: var(--workspace-muted); font-size: 12px; line-height: 1.65; }
.config-description { margin: 0 0 16px; }
.config-scope { margin: 7px 0 6px; font-size: 11px; }
.config-tools { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; }
.config-search { position: relative; }
.config-search-icon { position: absolute; left: 12px; top: 50%; transform: translateY(-50%); color: var(--workspace-muted); pointer-events: none; }
.config-text-button { font-size: 12px; color: rgb(var(--brand-rgb)); transition: opacity 120ms ease; }
.config-text-button:disabled { opacity: .45; }
.config-summary { margin-right: auto; color: var(--workspace-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.config-groups { max-height: min(360px, 45dvh); overflow-y: auto; overscroll-behavior: contain; border: 1px solid var(--workspace-rule); border-radius: 7px; background: linear-gradient(125deg, var(--workspace-highlight), transparent 65%), var(--workspace-surface); box-shadow: inset 0 1px 0 var(--workspace-highlight); }
.config-group { display: flex; align-items: center; gap: 10px; padding: 9px 11px; cursor: pointer; transition: background-color 120ms ease; }
.config-group + .config-group { border-top: 1px solid var(--workspace-rule); }
.config-group:hover, .config-group-selected { background: rgb(var(--brand-rgb) / .035); }
.config-group-name { min-width: 0; font-size: 12px; overflow-wrap: anywhere; }
.config-group-platform { margin-left: auto; flex-shrink: 0; color: var(--workspace-muted); font-size: 10px; }
.config-footer-note { margin: 11px 0 0; font-size: 11px; }
.config-empty-selection { color: var(--workspace-ink); }
.config-error { margin: 8px 0; font-size: 12px; line-height: 1.6; color: rgb(220 38 38); overflow-wrap: anywhere; }
.config-empty { padding: 24px 10px; text-align: center; font-size: 12px; color: var(--workspace-muted); }
@media (prefers-reduced-motion: reduce) { .config-group, .config-text-button { transition: none; } }
</style>
