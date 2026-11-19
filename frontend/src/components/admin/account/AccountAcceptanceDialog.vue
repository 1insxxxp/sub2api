<template>
  <BaseDialog :show="show && authStore.isAdmin" :title="tt('title')" width="extra-wide" appearance="neutral" @close="emit('close')">
    <div v-if="authStore.isAdmin" class="acceptance space-y-4">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ tt('intro') }}</p>
      <nav class="flex gap-2 border-b border-gray-200 pb-2 dark:border-dark-600" :aria-label="tt('title')">
        <button class="btn btn-sm" :class="tab === 'config' ? 'btn-primary' : 'btn-secondary'" :aria-pressed="tab === 'config'" @click="tab = 'config'">{{ tt('configure') }}</button>
        <button data-testid="history-tab" class="btn btn-sm" :class="tab === 'history' ? 'btn-primary' : 'btn-secondary'" :aria-pressed="tab === 'history'" @click="tab = 'history'">{{ tt('history') }} ({{ history.length }})</button>
      </nav>
      <p v-if="loading" role="status" class="text-sm text-gray-500">{{ tt('loading') }}</p>

      <section v-if="tab === 'config'" class="space-y-4">
        <div class="flex flex-wrap items-center gap-3">
          <button data-testid="select-all-models" class="btn btn-secondary btn-sm" :disabled="loading || starting || !allModelKeys.length" @click="toggleModels(allModelKeys)">{{ tt(allModelsSelected ? 'clearSelection' : 'selectAllModels') }}</button>
          <span class="text-xs text-gray-500 dark:text-gray-400" aria-live="polite">{{ tt('selectedModels', { selected: request.targets.length, total: allModelKeys.length }) }}</span>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ tt('scopeHint') }}</p>
        <p v-if="!loading && !targets.length" class="text-sm">{{ tt('noTargets') }}</p>
        <fieldset v-for="target in targets" :key="target.account_id" class="min-w-0 space-y-2 border-b border-gray-200 pb-3 dark:border-dark-600" :disabled="starting">
          <legend class="mb-2 max-w-full break-words text-sm font-semibold">{{ target.name }} <span class="font-normal text-gray-500">#{{ target.account_id }} · {{ target.platform }}</span></legend>
          <button v-if="targets.length > 1 && targetModelKeys(target).length" :data-testid="`select-account-models-${target.account_id}`" class="btn btn-secondary btn-sm" @click="toggleModels(targetModelKeys(target))">{{ tt(modelsSelected(targetModelKeys(target)) ? 'clearAccountSelection' : 'selectAccountModels') }}</button>
          <p v-if="!target.models?.length" class="text-xs text-gray-500">{{ target.unavailable_reason || tt('noModels') }}</p>
          <div v-for="model in target.models" :key="model.model" class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <label class="flex items-start gap-2 text-sm">
              <input v-model="selected" :disabled="Boolean(model.unavailable_reason)" type="checkbox" class="mt-1" :value="modelKey(target.account_id, model.model)" :data-testid="`model-${target.account_id}-${model.model}`" />
              <span class="min-w-0 break-all"><strong>{{ model.model }}</strong><span v-if="model.upstream_model !== model.model" class="text-gray-500"> → {{ model.upstream_model }}</span><span class="mt-1 block text-xs text-gray-500">{{ model.protocols?.join(' / ') || tt('unsupported') }}</span></span>
            </label>
            <p v-if="model.unavailable_reason" class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ model.unavailable_reason }}</p>
            <p v-else-if="!hasCostEstimate(model)" class="mt-2 text-xs text-amber-700 dark:text-amber-400">{{ tt('noCostEstimate') }}</p>
          </div>
        </fieldset>
        <fieldset :disabled="starting" class="space-y-2">
          <legend class="mb-1 text-sm font-semibold">{{ tt('suite') }}</legend>
          <p class="text-sm">{{ tt('standard') }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ tt('standardHint') }}</p>
          <div class="flex flex-wrap gap-x-5 gap-y-2">
            <button type="button" data-testid="select-all-extras" class="btn btn-secondary btn-sm"
              @click="extras = allExtrasSelected ? [] : [...extraOptions]">
              {{ tt(allExtrasSelected ? 'clearExtras' : 'selectAllExtras') }}
            </button>
            <label v-for="extra in extraOptions" :key="extra" class="flex items-center gap-2 text-sm"><input v-model="extras" type="checkbox" :value="extra" :data-testid="`extra-${extra}`" />{{ tt(`extras.${extra}`) }}</label>
          </div>
        </fieldset>
        <section v-if="preview" class="rounded-lg border border-gray-200 p-3 dark:border-dark-600" aria-live="polite">
          <div class="flex flex-wrap gap-x-6 gap-y-2 text-sm">
            <span>{{ tt('generationCalls') }}: <strong>{{ preview.generation_calls }}</strong></span>
            <span>{{ tt('countCalls') }}: <strong>{{ preview.count_calls }}</strong></span>
            <span>{{ tt('estimated') }}: <strong>{{ money(preview.estimated_cost_usd) }}</strong></span>
          </div>
          <p class="mt-2 text-xs text-gray-500">{{ tt('expires') }}: {{ date(preview.expires_at) }} · {{ preview.rule_version }}</p>
          <ul v-if="preview.issues?.length" class="mt-2 list-inside list-disc text-sm text-amber-700 dark:text-amber-400"><li v-for="issue in preview.issues" :key="issue">{{ issue }}</li></ul>
          <details v-if="preview.samples?.length" class="mt-2 text-xs">
            <summary class="cursor-pointer">{{ tt('samplePlan') }}</summary>
            <div class="mt-2 max-h-56 overflow-auto"><div v-for="sample in preview.samples" :key="sample.id" class="flex flex-wrap gap-2 border-b border-gray-100 py-1 dark:border-dark-700"><span>#{{ sample.target.account_id }} {{ sample.target.model }}</span><span>{{ categoryLabel(sample.category) }} · {{ sample.protocol }} · {{ sample.stream ? tt('stream') : tt('nonstream') }}</span><span>{{ money(sample.estimated_cost_usd) }}</span></div></div>
          </details>
        </section>
      </section>

      <section v-else class="flex flex-col gap-4">
        <div class="flex flex-wrap items-center gap-2">
          <button class="btn btn-secondary btn-sm" :disabled="refreshing" @click="refresh">{{ tt('refresh') }}</button>
          <button data-testid="compare" class="btn btn-secondary btn-sm" :disabled="compareIds.length !== 2 || comparing" @click="compareReports">{{ tt('compare') }} ({{ compareIds.length }}/2)</button>
          <span class="text-xs text-gray-500">{{ tt('background') }}</span>
        </div>
        <p v-if="!history.length && !loading" class="text-sm text-gray-500">{{ tt('noHistory') }}</p>
        <div v-for="run in history" :key="run.id" class="flex flex-wrap items-center gap-3 border-b border-gray-200 py-2 text-sm dark:border-dark-600">
          <input v-model="compareIds" type="checkbox" :value="run.id" :disabled="compareIds.length >= 2 && !compareIds.includes(run.id)" :aria-label="`${tt('compare')} ${run.id}`" :data-testid="`compare-${run.id}`" />
          <button class="min-w-0 flex-1 text-left text-primary-600 dark:text-primary-400" :data-testid="`report-${run.id}`" @click="openReport(run.id)"><span class="break-all">{{ run.id }}</span><span class="block text-xs text-gray-500">{{ date(run.created_at) }}</span></button>
          <span>{{ statusLabel(run.status) }} · {{ run.completed }}/{{ run.total }}</span>
          <span>{{ money(run.spent_cost_usd) }}</span>
        </div>

        <section v-if="comparison" data-testid="comparison" class="space-y-2">
          <h4 class="font-semibold">{{ tt('compare') }}</h4>
          <p class="text-xs text-gray-500">{{ tt('compareHint') }}</p>
          <p v-if="comparison.some(run => run.stale_config)" class="text-sm text-amber-700 dark:text-amber-400">{{ tt('stale') }}</p>
          <p v-if="!comparisonRows.length" class="text-sm">{{ tt('noMatches') }}</p>
          <div v-else class="overflow-x-auto"><table class="w-full min-w-[600px] text-left text-xs"><thead><tr><th class="p-2">{{ tt('sample') }}</th><th v-for="run in comparison" :key="run.id" class="p-2">{{ run.id }}<span class="block font-normal">{{ run.rule_version }}</span></th></tr></thead><tbody><tr v-for="row in comparisonRows" :key="row[0].id" data-testid="comparison-row" class="border-t border-gray-200 dark:border-dark-600"><th class="p-2 font-normal">{{ row[0].target.upstream_model || row[0].target.model }}<span class="block">{{ categoryLabel(row[0].category) }} · {{ row[0].protocol }} · {{ row[0].stream ? tt('stream') : tt('nonstream') }}</span></th><td v-for="sample in row" :key="sample.id" class="p-2">
                  <strong class="block break-words">#{{ sample.target.account_id }} {{ sample.target.account_name }}</strong>
                  <span class="block break-all">{{ sample.target.model }} → {{ sample.target.upstream_model || sample.target.model }}</span>
                  <span class="block">{{ tt('groupContext') }}: {{ sample.target.group_id ? `#${sample.target.group_id}` : tt('noGroup') }}</span>
                  <span class="block break-all" :title="sample.target.fingerprint">{{ tt('configVersion') }}: {{ sample.target.fingerprint?.slice(0, 12) || '—' }}</span>
                  <span v-if="sample.result?.source" class="block break-all">{{ sourceLabel(sample.result.source.kind) }} · {{ sample.result.source.host || '—' }}</span>
                  <span>{{ statusLabel(sample.result?.status ?? sample.status) }}</span><span class="block">{{ tt('raw') }}: {{ number(sample.result?.raw_usage?.input) }} / {{ number(sample.result?.raw_usage?.output) }}</span><span class="block">{{ tt('client') }}: {{ number(sample.result?.client_usage?.input) }} / {{ number(sample.result?.client_usage?.output) }}</span><span class="block">{{ tt('billing') }}: {{ number(sample.result?.billing_usage?.input) }} / {{ number(sample.result?.billing_usage?.output) }}</span><span class="block">{{ money(sample.result?.cost_usd) }}</span><span v-if="usageLayers.some(layer => sample.result?.[layer.key]?.output_includes_thoughts)" class="block text-amber-700 dark:text-amber-400">{{ tt('aggregateOutputHint') }}</span></td></tr></tbody></table></div>
        </section>

        <section v-if="report" class="space-y-3" :class="{ 'order-first': isActive(report) }">
          <div class="flex flex-wrap items-center gap-2"><h4 class="min-w-0 flex-1 break-all font-semibold">{{ report.id }}</h4><button :data-testid="`export-${report.id}`" class="btn btn-secondary btn-sm" :disabled="exporting" @click="exportReport(report.id)">{{ tt('export') }}</button><button v-if="canCancel(report)" :data-testid="`cancel-${report.id}`" class="btn btn-danger btn-sm" :disabled="cancelling || report.status === 'cancelling'" @click="cancelRun(report.id)">{{ tt('cancel') }}</button></div>
          <progress class="acceptance-progress" :value="report.completed" :max="Math.max(1, report.total)" :aria-label="tt('progress')" />
          <div class="flex flex-wrap gap-x-5 gap-y-1 text-xs" role="status"><span>{{ statusLabel(report.status) }} · {{ report.completed }}/{{ report.total }}</span><span>{{ tt('estimated') }}: {{ money(report.estimated_cost_usd) }}</span><span>{{ tt('reserved') }}: {{ money(report.reserved_cost_usd) }}</span><span>{{ tt('calculated') }}: {{ money(report.spent_cost_usd) }}</span><span>{{ report.rule_version }}</span></div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ tt('costHint') }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ tt('measurementHint') }}</p>
          <p v-if="report.stale_config" class="text-sm text-amber-700 dark:text-amber-400">{{ tt('stale') }}</p>
          <div v-for="cache in cacheSummaries" :key="cache.key" class="text-xs"><strong>#{{ cache.accountId }} {{ cache.model }}</strong> · {{ tt('cacheRate') }}: {{ cache.total > 0 ? `${(cache.cached / cache.total * 100).toFixed(1)}%` : '—' }} · {{ tt('cacheHits') }}: {{ cache.evaluable ? `${cache.hits}/${cache.evaluable}` : tt('notAssessable') }} <span class="text-gray-500">({{ tt('cacheConditions') }})</span></div>
          <div class="acceptance-console" data-testid="acceptance-console">
            <div class="acceptance-console-header">
              <span aria-hidden="true" class="acceptance-console-prompt">›_</span>
              <span>{{ tt('executionLog') }}</span>
              <span class="ml-auto text-xs tabular-nums">{{ report.completed }}/{{ report.total }}</span>
            </div>
            <div class="acceptance-console-body">
              <details v-for="sample in report.samples" :key="sample.id" class="acceptance-console-item">
                <summary class="acceptance-console-row">
                  <span class="acceptance-console-state" :data-state="sample.result?.status ?? sample.status">{{ statusLabel(sample.result?.status ?? sample.status) }}</span>
                  <span class="min-w-0">
                    <span class="block break-words">#{{ sample.target.account_id }} {{ sample.target.model }}</span>
                    <span class="acceptance-console-meta">{{ categoryLabel(sample.category) }} · {{ sample.protocol }} · {{ sample.stream ? tt('stream') : tt('nonstream') }}</span>
                  </span>
                  <span v-if="sample.result?.duration_ms != null" class="acceptance-console-meta whitespace-nowrap">{{ sample.result.duration_ms }} ms</span>
                </summary>
                <template v-if="sample.result">
                  <p class="mt-2 break-all text-xs">{{ tt('source') }}: {{ sourceLabel(sample.result.source?.kind) }} · {{ sample.result.source?.channel || '—' }} · {{ sample.result.source?.host }}{{ sample.result.source?.path }}</p>
                  <p class="mt-1 break-all text-xs text-gray-500">{{ tt('groupContext') }}: {{ sample.target.group_id ? `#${sample.target.group_id}` : tt('noGroup') }} · {{ tt('adapter') }}: {{ sample.result.source?.adapter || '—' }} / {{ sample.result.source?.auth_kind || '—' }} · {{ sample.result.source?.requested_model }} → {{ sample.result.source?.upstream_model }} → {{ sample.result.source?.response_model || '—' }}</p>
                  <div class="mt-2 overflow-x-auto"><table class="w-full min-w-[480px] text-right text-xs"><thead><tr><th class="p-2 text-left">{{ tt('usage') }}</th><th v-for="field in usageFields" :key="field" class="p-2">{{ tt(field) }}</th></tr></thead><tbody><tr v-for="layer in usageLayers" :key="layer.key" class="border-t border-gray-100 dark:border-dark-700"><th class="p-2 text-left font-normal">{{ tt(layer.label) }}<span class="block text-gray-500">{{ sample.result[layer.key]?.source }}</span><span v-if="sample.result[layer.key]?.output_includes_thoughts" class="mt-1 block text-amber-700 dark:text-amber-400">{{ tt('aggregateOutputHint') }}</span></th><td v-for="field in usageFields" :key="field" class="p-2" :data-testid="`${layer.label}-${field}-${sample.id}`">{{ number(sample.result[layer.key]?.[field]) }}</td></tr></tbody></table></div>
                  <p class="mt-1 text-xs text-gray-500">{{ tt('missingHint') }}</p>
                  <p class="mt-2 text-xs">HTTP {{ sample.result.http_status }} · {{ sample.result.duration_ms }} ms · {{ sample.result.finish_reason || '—' }} · {{ tt('calculated') }} {{ money(sample.result.cost_usd) }}</p>
                  <ul class="mt-2 space-y-1 text-xs"><li v-for="(finding, index) in sample.result.findings" :key="`${finding.code}-${index}`" :class="finding.level === 'error' ? 'text-red-600 dark:text-red-400' : 'text-gray-600 dark:text-gray-300'">{{ finding.message }}</li></ul>
                  <details class="mt-2 text-xs"><summary class="cursor-pointer">{{ tt('evidence') }}</summary><pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all rounded bg-slate-800 p-2 text-slate-200">{{ JSON.stringify({ parameters: sample.result.effective_parameters, usage_frames: sample.result.usage_frames, excerpt: sample.result.output_excerpt, price: sample.target.price }, null, 2) }}</pre></details>
                </template>
              </details>
            </div>
          </div>
        </section>
      </section>
    </div>
    <template #footer>
      <div class="w-full min-w-0 space-y-2">
        <p v-if="error" role="alert" class="break-words text-sm text-red-600 dark:text-red-400">{{ error }}</p>
        <template v-if="tab === 'config' && authStore.isAdmin">
          <div v-if="preview" class="flex flex-wrap gap-x-5 gap-y-1 text-sm" aria-live="polite">
            <span>{{ tt('generationCalls') }}: <strong>{{ preview.generation_calls }}</strong></span>
            <span>{{ tt('countCalls') }}: <strong>{{ preview.count_calls }}</strong></span>
            <span>{{ tt('estimated') }}: <strong>{{ money(preview.estimated_cost_usd) }}</strong></span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ tt('estimateSource') }}</span>
          </div>
          <p v-if="!loading && validationMessage" role="status" class="break-words text-xs text-amber-700 dark:text-amber-400">{{ validationMessage }}</p>
          <p v-else-if="previewExpired" role="status" class="text-xs text-amber-700 dark:text-amber-400">{{ tt('previewExpired') }}</p>
          <p v-else-if="preview?.issues?.length" role="status" class="break-words text-xs text-amber-700 dark:text-amber-400">{{ preview.issues.join(' · ') }}</p>
          <p v-else-if="!preview" class="text-xs text-gray-500 dark:text-gray-400">{{ tt('flowHint') }}</p>
        </template>
        <div class="flex flex-wrap justify-end gap-2">
          <button class="btn btn-secondary" @click="emit('close')">{{ tt('close') }}</button>
          <template v-if="tab === 'config' && authStore.isAdmin">
            <button data-testid="preview" class="btn" :class="preview ? 'btn-secondary' : 'btn-primary'" :disabled="!validRequest || previewing || starting || loading" @click="previewRequest">{{ previewing ? tt('loading') : tt(preview ? 'repreview' : 'preview') }}</button>
            <button v-if="preview" data-testid="start" class="btn btn-primary" :disabled="!canStart" @click="startRun">{{ starting ? tt('loading') : tt('confirmStart') }}</button>
          </template>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { channelAcceptanceApi as api, type AcceptanceExtra, type AcceptanceModel, type AcceptancePreview, type AcceptancePreviewRequest, type AcceptanceRun, type AcceptanceSample, type AcceptanceTarget } from '@/api/admin/channelAcceptance'

const props = defineProps<{ show: boolean; accountIds: number[] }>()
const emit = defineEmits<{ close: [] }>()
const authStore = useAuthStore()
const { t, te } = useI18n()
const tt = (key: string, params: Record<string, string | number> = {}) => t(`admin.accounts.acceptance.${key}`, params)
const translated = (scope: string, value: string) => te?.(`admin.accounts.acceptance.${scope}.${value}`) ? tt(`${scope}.${value}`) : value
const statusLabel = (value: string) => translated('statuses', value)
const sourceLabel = (value?: string) => translated('sources', value || 'unverified')
const categoryLabel = (value: string) => translated('categories', value)
const number = (value?: number | null) => value == null ? '—' : value.toLocaleString()
const money = (value?: number | null) => value == null || !Number.isFinite(value) ? '—' : `$${value.toFixed(6)}`
const date = (value: string) => new Date(value).toLocaleString()
// Terminal run labels can precede settlement of already-dispatched requests.
// The repository counts cancelled/skipped samples as completed, so this delta
// only keeps polling for outstanding samples, including summary-only list rows.
const isActive = (run: AcceptanceRun) => ['queued', 'pending', 'running', 'cancelling'].includes(run.status)
  || Boolean(run.samples?.some(sample => sample.status === 'running'))
  || (['budget_exhausted', 'cancelled'].includes(run.status) && run.completed < run.total)
const canCancel = (run: AcceptanceRun) => isActive(run) && !['cancelled', 'cancelling'].includes(run.status)
const modelKey = (id: number, model: string) => JSON.stringify([id, model])
const extraOptions: AcceptanceExtra[] = ['long_context', 'image', 'audio', 'tools']
const usageFields = ['input', 'output', 'thoughts', 'cache_read', 'cache_create', 'total'] as const
const usageLayers = [{ key: 'raw_usage', label: 'raw' }, { key: 'client_usage', label: 'client' }, { key: 'billing_usage', label: 'billing' }] as const
const tab = ref<'config' | 'history'>('config')
const targets = ref<AcceptanceTarget[]>([])
const selected = ref<string[]>([])
const extras = ref<AcceptanceExtra[]>([])
const allExtrasSelected = computed(() => extraOptions.every(option => extras.value.includes(option)))
const preview = ref<AcceptancePreview | null>(null)
const history = ref<AcceptanceRun[]>([])
const report = ref<AcceptanceRun | null>(null)
const compareIds = ref<string[]>([])
const comparison = ref<[AcceptanceRun, AcceptanceRun] | null>(null)
const loading = ref(false)
const previewing = ref(false)
const starting = ref(false)
const refreshing = ref(false)
const cancelling = ref(false)
const comparing = ref(false)
const exporting = ref(false)
const error = ref('')
const now = ref(Date.now())
let disposed = false
let epoch = 0
let requestRevision = 0
let reportRevision = 0
let historyRevision = 0
let timer: ReturnType<typeof setTimeout> | undefined
const usable = () => !disposed && props.show && authStore.isAdmin && props.accountIds.length > 0
const hasCostEstimate = (model: AcceptanceModel) => model.price != null && [model.price.input_per_million, model.price.output_per_million, model.price.cache_per_million].every(value => typeof value === 'number' && Number.isFinite(value) && value >= 0)
const targetModelKeys = (target: AcceptanceTarget) => (target.models || []).filter(model => !model.unavailable_reason).map(model => modelKey(target.account_id, model.model))
const allModelKeys = computed(() => [...new Set(targets.value.flatMap(targetModelKeys))])
const modelsSelected = (keys: string[]) => keys.length > 0 && keys.every(key => selected.value.includes(key))
const allModelsSelected = computed(() => modelsSelected(allModelKeys.value))
function toggleModels(keys: string[]) {
  if (!usable() || starting.value || loading.value) return
  const removing = new Set(keys)
  selected.value = modelsSelected(keys) ? selected.value.filter(key => !removing.has(key)) : [...new Set([...selected.value, ...keys])]
}
const request = computed<AcceptancePreviewRequest>(() => ({
  targets: targets.value.flatMap(target => (target.models || []).filter(model => selected.value.includes(modelKey(target.account_id, model.model))).map(model => ({ account_id: target.account_id, model: model.model }))),
  extras: [...extras.value],
}))
const validationMessage = computed(() => {
  if (!request.value.targets.length) return tt('chooseModels')
  if (request.value.targets.length > 25) return tt('tooManyModels')
  for (const target of targets.value) {
    for (const model of target.models || []) {
      const key = modelKey(target.account_id, model.model)
      if (selected.value.includes(key) && !hasCostEstimate(model)) return tt('missingEstimate', { model: `#${target.account_id} ${model.model}` })
    }
  }
  return ''
})
const validRequest = computed(() => usable() && !validationMessage.value)
const previewExpired = computed(() => preview.value && new Date(preview.value.expires_at).getTime() <= now.value)
const canStart = computed(() => usable() && validRequest.value && preview.value?.can_start && new Date(preview.value.expires_at).getTime() > now.value && !starting.value && !previewing.value)
watch([selected, extras], () => { preview.value = null; requestRevision++ }, { deep: true, flush: 'sync' })
watch(compareIds, () => { comparison.value = null }, { deep: true })
function fail(value: unknown) {
  const message = value as { message?: string; response?: { data?: { message?: string } } }
  error.value = message?.response?.data?.message || message?.message || tt('failed')
}
function upsert(run: AcceptanceRun) {
  history.value = [run, ...history.value.filter(item => item.id !== run.id)].sort((a, b) => b.created_at.localeCompare(a.created_at))
}
async function previewRequest() {
  if (!validRequest.value || previewing.value) return
  const version = requestRevision; const scope = epoch
  previewing.value = true; error.value = ''; preview.value = null
  try {
    const result = await api.preview(request.value)
    if (usable() && version === requestRevision && scope === epoch) { preview.value = result; now.value = Date.now() }
  } catch (e) { if (scope === epoch) fail(e) }
  finally { if (scope === epoch) previewing.value = false }
}
async function startRun() {
  now.value = Date.now()
  if (!canStart.value || !preview.value) return
  const scope = epoch
  starting.value = true; error.value = ''
  try {
    const run = await api.start(preview.value.id)
    if (usable() && scope === epoch) { historyRevision++; reportRevision++; upsert(run); report.value = run; preview.value = null; tab.value = 'history' }
  } catch (e) { if (scope === epoch) { preview.value = null; fail(e) } }
  finally { if (scope === epoch) starting.value = false }
}
async function openReport(id: string) {
  const version = ++reportRevision; const scope = epoch
  error.value = ''
  try { const run = await api.get(id); if (usable() && scope === epoch && version === reportRevision) { report.value = run; upsert(run) } }
  catch (e) { if (scope === epoch) fail(e) }
}
async function refresh() {
  if (!usable() || refreshing.value) return
  const scope = epoch; const id = report.value?.id; const version = reportRevision; const historyVersion = historyRevision
  refreshing.value = true
  try {
    const runs = await api.list([...props.accountIds])
    if (scope !== epoch || !usable() || historyVersion !== historyRevision) return
    history.value = runs
    if (id) {
      const detail = await api.get(id)
      if (scope === epoch && usable() && version === reportRevision) report.value = detail
    }
  } catch (e) { if (scope === epoch) fail(e) }
  finally { if (scope === epoch) refreshing.value = false }
}
function poll() {
  clearTimeout(timer)
  if (!usable()) return
  timer = setTimeout(async () => {
    now.value = Date.now()
    if (history.value.some(isActive) || (report.value && isActive(report.value))) await refresh()
    poll()
  }, 3000)
}
async function cancelRun(id: string) {
  if (!usable() || cancelling.value) return
  const scope = epoch
  cancelling.value = true; historyRevision++; reportRevision++; error.value = ''
  try { const run = await api.cancel(id); if (usable() && scope === epoch) { historyRevision++; reportRevision++; upsert(run); if (report.value?.id === id) report.value = run } }
  catch (e) { if (scope === epoch) fail(e) }
  finally { if (scope === epoch) cancelling.value = false }
}
async function exportReport(id: string) {
  if (!usable() || exporting.value) return
  exporting.value = true; error.value = ''
  try {
    const blob = await api.exportReport(id)
    if (!usable()) return
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url; anchor.download = `channel-acceptance-${id.replace(/[^a-zA-Z0-9-]/g, '_')}.json`
    document.body.appendChild(anchor); anchor.click(); anchor.remove()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e) { fail(e) }
  finally { exporting.value = false }
}
async function compareReports() {
  if (!usable() || compareIds.value.length !== 2 || comparing.value) return
  const ids = [...compareIds.value]; const scope = epoch
  comparing.value = true; error.value = ''
  try {
    const [left, right] = await Promise.all(ids.map(id => api.get(id)))
    if (scope === epoch && usable() && JSON.stringify(ids) === JSON.stringify(compareIds.value)) comparison.value = [left, right]
  } catch (e) { if (scope === epoch) fail(e) }
  finally { if (scope === epoch) comparing.value = false }
}
const comparisonRows = computed(() => {
  if (!comparison.value) return [] as [AcceptanceSample, AcceptanceSample][]
  const key = (s: AcceptanceSample) => JSON.stringify([s.target.upstream_model || s.target.model, s.category, s.protocol, s.stream, s.input_size])
  const right = new Map<string, AcceptanceSample[]>()
  for (const sample of comparison.value[1].samples || []) right.set(key(sample), [...(right.get(key(sample)) || []), sample])
  const rows: [AcceptanceSample, AcceptanceSample][] = []
  for (const sample of comparison.value[0].samples || []) {
    const candidates = right.get(key(sample))
    if (!candidates?.length) continue
    // Prefer the same target for historical comparisons, but allow explicitly
    // labelled cross-channel pairs when accounts or pricing contexts differ.
    const sameTarget = candidates.findIndex(other => other.target.account_id === sample.target.account_id && other.target.group_id === sample.target.group_id)
    const [match] = candidates.splice(sameTarget >= 0 ? sameTarget : 0, 1)
    rows.push([sample, match])
  }
  return rows
})
const cacheSummaries = computed(() => {
  const rows = new Map<string, { key: string; accountId: number; model: string; cached: number; total: number; hits: number; evaluable: number }>()
  for (const sample of report.value?.samples || []) {
    if (!sample.category.startsWith('cache')) continue
    const key = JSON.stringify([sample.target.account_id, sample.target.model, sample.target.group_id ?? null])
    const row = rows.get(key) || { key, accountId: sample.target.account_id, model: sample.target.model, cached: 0, total: 0, hits: 0, evaluable: 0 }
    const usage = sample.result?.raw_usage
    const minimum = sample.result?.effective_parameters?.cache_min_input_tokens
    const knownMinimum = typeof minimum === 'number' && Number.isFinite(minimum) && minimum > 0
    const cacheEligible = usage?.cache_read != null && (usage.cache_read > 0 || (knownMinimum && usage.input != null && usage.input >= minimum))
    if (sample.result?.completed && cacheEligible && usage?.source && !/unknown|estimat|fallback/i.test(usage.source) && usage?.input != null && Number.isFinite(usage.input) && usage.input > 0 && usage.cache_read != null && Number.isFinite(usage.cache_read) && usage.cache_read >= 0 && usage.cache_read <= usage.input) {
      row.total += usage.input; row.cached += usage.cache_read; row.evaluable++; if (usage.cache_read > 0) row.hits++
    }
    rows.set(key, row)
  }
  return [...rows.values()]
})
watch([() => props.show, () => props.accountIds.join(','), () => authStore.isAdmin], async () => {
  const scope = ++epoch
  clearTimeout(timer)
  previewing.value = false; starting.value = false; refreshing.value = false; cancelling.value = false; comparing.value = false
  if (!usable()) return
  loading.value = true; error.value = ''; preview.value = null; comparison.value = null; compareIds.value = []
  const results = await Promise.allSettled([api.targets([...props.accountIds]), api.list([...props.accountIds])])
  if (scope !== epoch || !usable()) return
  if (results[0].status === 'fulfilled') {
    targets.value = results[0].value
    const available = new Set(allModelKeys.value)
    selected.value = selected.value.filter(key => available.has(key))
  } else { targets.value = []; fail(results[0].reason) }
  if (results[1].status === 'fulfilled') {
    history.value = results[1].value
    if (report.value && !history.value.some(item => item.id === report.value?.id)) report.value = null
    if (report.value) await openReport(report.value.id)
  } else { history.value = []; report.value = null; fail(results[1].reason) }
  if (scope === epoch) { loading.value = false; poll() }
}, { immediate: true })
onUnmounted(() => { disposed = true; epoch++; clearTimeout(timer) })
</script>


<style scoped>
.acceptance-progress { display: block; width: 100%; height: 4px; accent-color: #0891b2; }
.acceptance-console {
  overflow: hidden;
  border: 1px solid #334155;
  border-radius: 8px;
  background: #101722;
  color: #dbe6f4;
  font-family: "IBM Plex Mono", ui-monospace, SFMono-Regular, Consolas, monospace;
}
.acceptance-console-header {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 13px;
  border-bottom: 1px solid #334155;
  background: #182333;
  font-size: 12px;
}
.acceptance-console-prompt { color: #67e8f9; font-weight: 700; }
.acceptance-console-body { max-height: min(52vh, 34rem); overflow: auto; overscroll-behavior: contain; }
.acceptance-console-item { padding: 0 12px; border-bottom: 1px solid #253145; }
.acceptance-console-item:last-child { border-bottom: 0; }
.acceptance-console-item[open] { padding-bottom: 12px; background: #141e2c; }
.acceptance-console-row {
  display: grid;
  grid-template-columns: minmax(5.5rem, auto) minmax(0, 1fr) auto;
  align-items: start;
  gap: 10px;
  padding: 9px 0;
  cursor: pointer;
  font-size: 12px;
}
.acceptance-console-row::-webkit-details-marker { display: none; }
.acceptance-console-row:focus-visible { outline: 1px solid #67e8f9; outline-offset: -1px; }
.acceptance-console-meta, .acceptance-console-state { color: #9caec6; }
.acceptance-console-state::before { display: inline-block; margin-right: 6px; content: "›"; }
.acceptance-console-item[open] .acceptance-console-state::before { transform: rotate(90deg); }
.acceptance-console-state[data-state="passed"] { color: #6ee7b7; }
.acceptance-console-state[data-state="running"] { color: #67e8f9; }
.acceptance-console-state[data-state="error"], .acceptance-console-state[data-state="anomaly"], .acceptance-console-state[data-state="failed"] { color: #fda4af; }
.acceptance-console-item :deep(.text-gray-500), .acceptance-console-item :deep(.text-gray-600) { color: #9caec6; }
.acceptance-console-item :deep(tr) { border-color: #334155; }
.acceptance-console-item :deep(.text-red-600) { color: #fda4af; }
.acceptance-console-item :deep(.text-amber-700) { color: #fcd34d; }
@media (max-width: 480px) {
  .acceptance-console-row { grid-template-columns: 4.5rem minmax(0, 1fr); gap: 6px; }
  .acceptance-console-row > :last-child:nth-child(3) { grid-column: 2; }
}
</style>
