<template>
  <div class="model-probe-panel min-w-0" data-test="model-probe-panel">
    <div class="probe-toolbar">
      <div class="probe-group-field min-w-0">
        <label for="probe-group" class="input-label">{{ t('modelProbes.group') }}</label>
        <Select id="probe-group" v-model="selectedGroup" data-test="probe-group" :options="groupOptions" :searchable="true" :mobile-sheet="false"
          :aria-label="t('modelProbes.group')" :placeholder="t('modelProbes.selectGroup')" :search-placeholder="t('modelProbes.searchGroups')"
          :disabled="checking || denied || loadingGroups" />
      </div>
      <div class="probe-actions">
        <button v-if="running" type="button" data-test="probe-stop" class="btn btn-secondary min-h-11 gap-2" @click="stop">
          <Icon name="x" size="sm" />{{ t('modelProbes.stop') }}
        </button>
        <button v-else type="button" data-test="probe-all" class="btn btn-primary min-h-11 gap-2" :disabled="!rows.length || loadingModels || denied" @click="start(rows)">
          <Icon name="play" size="sm" />{{ t('modelProbes.probeAll') }}
        </button>
        <button type="button" data-test="probe-retry-failed" class="btn btn-secondary min-h-11" :disabled="running || !retryRows.length || denied" @click="start(retryRows)">{{ t('modelProbes.retryFailed') }}</button>
      </div>
    </div>
    <p class="probe-note">{{ t('modelProbes.description') }}</p>
    <p v-if="errorKey" role="alert" class="probe-error">{{ t(errorKey) }} <button v-if="!denied" type="button" class="inline-flex min-h-11 items-center px-2 underline underline-offset-4" @click="reload">{{ t('modelProbes.reload') }}</button></p>
    <p v-if="checking || loadingGroups || loadingModels" role="status" class="probe-empty">{{ t(checking ? 'modelProbes.checkingAccess' : 'modelProbes.loading') }}</p>
    <template v-else-if="rows.length">
      <div class="probe-summary" role="status" aria-live="polite" data-test="probe-progress">
        <span class="probe-summary-total">{{ t('modelProbes.models') }} <strong>{{ rows.length }}</strong></span>
        <span>{{ t('modelProbes.completed') }} <strong>{{ completed }}</strong><span class="probe-divider">/</span>{{ rows.length }}</span>
        <span class="probe-success-text">{{ t('modelProbes.status.success') }} <strong>{{ counts.success }}</strong></span>
        <span>{{ t('modelProbes.failures') }} <strong>{{ failedCount }}</strong></span>
        <span v-if="counts.cancelled">{{ t('modelProbes.status.cancelled') }} <strong>{{ counts.cancelled }}</strong></span>
      </div>
      <div v-if="running || completed" class="probe-progress-track" role="progressbar" :aria-label="t('modelProbes.progress')" :aria-valuenow="completed" :aria-valuemin="0" :aria-valuemax="rows.length"><span :style="{ width: `${completed / rows.length * 100}%` }" /></div>
      <div class="probe-list" :aria-label="t('modelProbes.models')">
        <div class="probe-list-heading" aria-hidden="true"><span>{{ t('modelProbes.model') }}</span><span>{{ t('modelProbes.result') }}</span><span>{{ t('modelProbes.duration') }}</span><span /></div>
        <article v-for="row in rows" :key="row.id" data-test="probe-row" :data-probe-status="row.status" class="probe-row">
          <div class="probe-model"><span class="probe-model-id">{{ row.display_name || row.id }}</span><span v-if="row.display_name && row.display_name !== row.id" class="probe-model-alias">{{ row.id }}</span></div>
          <div class="probe-result"><span class="probe-state" :class="`probe-state-${row.status}`"><span class="probe-state-dot" :class="{ 'motion-safe:animate-pulse': row.status === 'testing' }" />{{ t(`modelProbes.status.${row.status}`) }}</span><p v-if="isFailure(row.status)" class="probe-reason">{{ t(`modelProbes.reasons.${row.status}`) }}</p></div>
          <span class="probe-duration tabular-nums">{{ row.duration_ms !== undefined ? `${Math.max(0, Math.round(row.duration_ms))} ms` : '—' }}</span>
          <button type="button" :data-test="`probe-single-${row.id}`" class="probe-row-action min-h-11" :disabled="running || denied" :aria-label="`${t(isFailure(row.status) || row.status === 'cancelled' ? 'modelProbes.retry' : 'modelProbes.probe')} ${row.id}`" @click="start([row])"><Icon name="refresh" size="sm" /><span>{{ t(isFailure(row.status) || row.status === 'cancelled' ? 'modelProbes.retry' : 'modelProbes.probe') }}</span></button>
        </article>
      </div>
    </template>
    <p v-else-if="!checking && !loadingGroups && !loadingModels && !denied && !errorKey" class="probe-empty">{{ t(selectedGroup ? 'modelProbes.noModels' : groups.length ? 'modelProbes.initial' : 'modelProbes.noGroups') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useUserSupportAccess } from '@/composables/useUserSupportAccess'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import { listGroups, listModels, runProbe, type ProbeGroup, type ProbeModel, type ProbeStatus } from '@/api/admin/modelProbes'

type RowStatus = ProbeStatus | 'idle' | 'pending' | 'testing'
interface ProbeRow extends ProbeModel { status: RowStatus; duration_ms?: number }
const { t } = useI18n()
const emit = defineEmits<{ 'access-denied': [] }>()
const auth = useAuthStore()
const access = useUserSupportAccess(auth)
const selectedGroup = ref<string | number | boolean | null>(null)
const groups = ref<ProbeGroup[]>([])
const rows = ref<ProbeRow[]>([])
const checking = ref(true)
const denied = ref(false)
const loadingGroups = ref(false)
const loadingModels = ref(false)
const running = ref(false)
const errorKey = ref('')
let sessionVersion = 0
let modelVersion = 0
let runVersion = 0
let metadataController: AbortController | null = null
const controllers = new Set<AbortController>()
const identity = () => `${auth.user?.id}:${auth.user?.role}:${auth.token ?? ''}`
const groupOptions = computed(() => groups.value.map(group => ({ value: group.id, label: group.name })))
const isFailure = (status: RowStatus) => ['failed', 'timeout', 'unsupported'].includes(status)
const retryRows = computed(() => rows.value.filter(row => isFailure(row.status) || row.status === 'cancelled'))
const completed = computed(() => rows.value.filter(row => !['idle', 'pending', 'testing'].includes(row.status)).length)
const counts = computed(() => ({
  success: rows.value.filter(row => row.status === 'success').length,
  cancelled: rows.value.filter(row => row.status === 'cancelled').length,
}))
const failedCount = computed(() => rows.value.filter(row => isFailure(row.status)).length)

function isUnauthorized(error: unknown) {
  const failure = error as { status?: number; response?: { status?: number } } | null
  const status = failure?.status ?? failure?.response?.status
  return status === 401 || status === 403
}

function stop() {
  runVersion++
  controllers.forEach(controller => controller.abort())
  controllers.clear()
  rows.value.forEach(row => {
    if (row.status === 'pending' || row.status === 'testing') row.status = 'cancelled'
  })
  running.value = false
}

function revokeAccess() {
  if (denied.value) return
  stop()
  modelVersion++
  metadataController?.abort()
  denied.value = true
  loadingGroups.value = false
  loadingModels.value = false
  rows.value = []
  groups.value = []
  errorKey.value = 'modelProbes.errors.forbidden'
  access.revoke()
  emit('access-denied')
}

async function initialize() {
  const version = ++sessionVersion
  stop()
  modelVersion++
  metadataController?.abort()
  selectedGroup.value = null
  rows.value = []
  groups.value = []
  errorKey.value = ''
  denied.value = false
  checking.value = true
  loadingGroups.value = false
  loadingModels.value = false
  if (!['admin', 'sub_admin'].includes(auth.user?.role ?? '')) {
    checking.value = false
    denied.value = true
    errorKey.value = 'modelProbes.errors.forbidden'
    return
  }
  const allowed = await access.refresh()
  if (version !== sessionVersion) return
  checking.value = false
  if (!allowed) {
    revokeAccess()
    return
  }
  await reload()
}

async function reload() {
  if (denied.value) return
  if (typeof selectedGroup.value === 'number') {
    await loadModels(selectedGroup.value)
    return
  }
  const version = sessionVersion
  metadataController?.abort()
  const controller = new AbortController()
  metadataController = controller
  loadingGroups.value = true
  errorKey.value = ''
  try {
    const data = await listGroups(controller.signal)
    if (version !== sessionVersion || controller.signal.aborted) return
    groups.value = data.items
  } catch (error) {
    if (version !== sessionVersion || controller.signal.aborted) return
    if (isUnauthorized(error)) revokeAccess()
    else errorKey.value = 'modelProbes.errors.groups'
  } finally {
    if (version === sessionVersion && !controller.signal.aborted) loadingGroups.value = false
  }
}

async function loadModels(value: typeof selectedGroup.value) {
  stop()
  const version = ++modelVersion
  metadataController?.abort()
  rows.value = []
  errorKey.value = ''
  loadingModels.value = false
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value <= 0 || denied.value) return
  const controller = new AbortController()
  metadataController = controller
  loadingModels.value = true
  try {
    const data = await listModels(value, controller.signal)
    if (version !== modelVersion || controller.signal.aborted) return
    rows.value = data.items.map(model => ({ id: model.id, display_name: model.display_name, status: 'idle' }))
  } catch (error) {
    if (version !== modelVersion || controller.signal.aborted) return
    if (isUnauthorized(error)) revokeAccess()
    else errorKey.value = 'modelProbes.errors.models'
  } finally {
    if (version === modelVersion && !controller.signal.aborted) loadingModels.value = false
  }
}
watch(selectedGroup, loadModels)

async function start(items: ProbeRow[]) {
  if (running.value || denied.value || checking.value || loadingModels.value || typeof selectedGroup.value !== 'number') return
  const queue = [...items]
  if (!queue.length) return
  const version = ++runVersion
  const groupId = selectedGroup.value
  const actor = identity()
  errorKey.value = ''
  running.value = true
  queue.forEach(row => {
    row.status = 'pending'
    row.duration_ms = undefined
  })
  let next = 0
  const current = () => version === runVersion && groupId === selectedGroup.value && actor === identity() && !denied.value
  async function worker() {
    while (current() && next < queue.length) {
      const row = queue[next++]
      row.status = 'testing'
      const controller = new AbortController()
      controllers.add(controller)
      try {
        const result = await runProbe(groupId, row.id, controller.signal)
        if (!current() || controller.signal.aborted) return
        // Never render upstream text; bind terminal status to the requested public identity.
        const matches = result.group_id === groupId && result.model === row.id
        const terminal = ['success', 'failed', 'timeout', 'cancelled', 'unsupported'].includes(result.status)
        row.status = matches && terminal ? result.status : 'failed'
        row.duration_ms = matches && Number.isFinite(result.duration_ms) && result.duration_ms >= 0
          ? result.duration_ms
          : undefined
      } catch (error) {
        if (!current() || controller.signal.aborted) return
        if (isUnauthorized(error)) {
          revokeAccess()
          return
        }
        row.status = (error as { code?: string } | null)?.code === 'ECONNABORTED' ? 'timeout' : 'failed'
      } finally {
        controllers.delete(controller)
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(3, queue.length) }, worker))
  if (current()) running.value = false
}

watch(identity, initialize, { immediate: true })
watch(access.canAccess, value => {
  if (!checking.value && !value && !denied.value) revokeAccess()
})
onBeforeUnmount(() => {
  sessionVersion++
  modelVersion++
  stop()
  metadataController?.abort()
})
</script>

<style scoped>
.model-probe-panel { color: var(--workspace-ink); }
.probe-toolbar { display: flex; align-items: flex-end; flex-wrap: wrap; gap: 12px; }
.probe-group-field { flex: 1 1 280px; max-width: 480px; }
.probe-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.probe-note { margin: 10px 0 20px; font-size: 12px; line-height: 1.7; color: var(--workspace-muted); }
.probe-error { margin: 14px 0; font-size: 13px; color: rgb(220 38 38); }
.probe-empty { padding: 36px 12px; text-align: center; font-size: 13px; color: var(--workspace-muted); }
.probe-summary { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 20px; padding: 10px 0; font-size: 12px; color: var(--workspace-muted); }
.probe-summary strong { margin-left: 5px; color: var(--workspace-ink); font-variant-numeric: tabular-nums; font-weight: 600; }
.probe-summary-total { margin-right: auto; }
.probe-divider { margin: 0 4px; opacity: .55; }
.probe-success-text { color: rgb(5 150 105); }
.probe-progress-track { height: 2px; margin-bottom: 8px; overflow: hidden; background: var(--workspace-rule); border-radius: 2px; }
.probe-progress-track span { display: block; height: 100%; background: rgb(var(--brand-rgb)); transition: width 150ms ease; }
.probe-list { display: grid; gap: 6px; }
.probe-list-heading, .probe-row { display: grid; grid-template-columns: minmax(0, 1fr) minmax(150px, .5fr) 90px 76px; align-items: center; gap: 16px; }
.probe-list-heading { padding: 7px 14px; font-size: 11px; color: var(--workspace-muted); }
.probe-row { min-width: 0; padding: 7px 14px; border: 1px solid var(--workspace-rule); border-radius: 7px; background: linear-gradient(125deg, var(--workspace-highlight), transparent 65%), var(--workspace-surface); box-shadow: inset 0 1px 0 var(--workspace-highlight), 0 2px 5px rgb(0 0 0 / .025); }
.probe-model { min-width: 0; display: grid; gap: 3px; }
.probe-model-id { font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.probe-model-alias, .probe-reason { font-size: 11px; color: var(--workspace-muted); overflow-wrap: anywhere; }
.probe-reason { margin: 4px 0 0; line-height: 1.5; }
.probe-state { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--workspace-muted); }
.probe-state-dot { height: 5px; width: 5px; border-radius: 50%; background: currentColor; flex-shrink: 0; }
.probe-state-success { color: rgb(5 150 105); }
.probe-state-failed { color: rgb(220 38 38); }
.probe-state-timeout, .probe-state-unsupported { color: rgb(180 83 9); }
.probe-state-testing { color: rgb(var(--brand-rgb)); }
.probe-duration { font-size: 12px; color: var(--workspace-muted); }
.probe-row-action { display: inline-flex; align-items: center; justify-content: center; gap: 5px; border-radius: 5px; color: rgb(var(--brand-rgb)); font-size: 12px; }
.probe-row-action:hover:not(:disabled) { background: var(--workspace-hover); }
.probe-row-action:focus-visible { outline: 2px solid rgb(var(--brand-rgb)); outline-offset: 2px; }
.probe-row-action:disabled { opacity: .4; cursor: not-allowed; }
:global(.dark .model-probe-panel .probe-error), :global(.dark .model-probe-panel .probe-state-failed) { color: rgb(248 113 113); }
:global(.dark .model-probe-panel .probe-state-success), :global(.dark .model-probe-panel .probe-success-text) { color: rgb(52 211 153); }
:global(.dark .model-probe-panel .probe-state-timeout), :global(.dark .model-probe-panel .probe-state-unsupported) { color: rgb(251 191 36); }
@media (max-width: 639px) {
  .probe-group-field { flex-basis: 100%; max-width: none; }
  .probe-actions { width: 100%; }
  .probe-actions > button { flex: 1 1 auto; padding-inline: 10px; font-size: 12px; }
  .probe-list-heading { display: none; }
  .probe-row { grid-template-columns: minmax(0, 1fr) auto; gap: 7px 12px; padding: 10px 12px; }
  .probe-model { grid-column: 1; }
  .probe-result { grid-column: 1; grid-row: 2; }
  .probe-duration { grid-column: 1; grid-row: 3; font-size: 11px; }
  .probe-row-action { grid-column: 2; grid-row: 1 / 4; min-width: 44px; padding-inline: 8px; }
  .probe-summary { gap: 7px 14px; }
}
@media (prefers-reduced-motion: reduce) { .probe-progress-track span { transition: none; } }
</style>
