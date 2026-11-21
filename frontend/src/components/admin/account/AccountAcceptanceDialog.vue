<template>
  <BaseDialog :show="show && authStore.isAdmin" :title="tt('title')" width="extra-wide" appearance="neutral" @close="emit('close')">
    <div v-if="authStore.isAdmin" class="acceptance space-y-4">
      <p v-if="loading" role="status" class="text-sm text-gray-500">{{ tt('loading') }}</p>

      <section v-if="!report" class="space-y-4">
        <div class="flex flex-wrap items-center gap-3">
          <button data-testid="select-all-models" class="btn btn-secondary btn-sm" :disabled="loading || starting || !allModelKeys.length" @click="toggleModels(allModelKeys)">{{ tt(allModelsSelected ? 'clearSelection' : 'selectAllModels') }}</button>
          <span class="text-xs text-gray-500 dark:text-gray-400" aria-live="polite">{{ tt('selectedModels', { selected: request.targets.length, total: allModelKeys.length }) }}</span>
          <button v-if="allModelKeys.length" type="button" class="ml-auto text-xs underline" :aria-expanded="showMappings" @click="showMappings = !showMappings">{{ tt('mappingDetails') }}</button>
        </div>
        <p v-if="!loading && !targets.length" class="text-sm">{{ tt('noTargets') }}</p>
        <fieldset v-for="target in targets" :key="target.account_id" class="min-w-0 space-y-2 border-b border-gray-200 pb-3 dark:border-dark-600" :disabled="starting">
          <legend class="mb-2 max-w-full break-words text-sm font-semibold">{{ target.name }} <span class="font-normal text-gray-500">#{{ target.account_id }} · {{ target.platform }}</span></legend>
          <div v-if="targetModelKeys(target).length" class="flex flex-wrap items-center gap-x-4 gap-y-2">
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <label :for="`acceptance-source-${target.account_id}`">{{ tt('declaredSource') }}</label>
              <Select
                v-model="declarations[target.account_id]"
                :id="`acceptance-source-${target.account_id}`"
                :data-testid="`declared-source-${target.account_id}`"
                :aria-label="tt('declaredSource')"
                :options="sourceOptions"
                :placeholder="tt('sourceOff')"
                :disabled="starting"
                :searchable="false"
                class="acceptance-source-select"
              />
            </div>
            <button v-if="targets.length > 1 && targetModelKeys(target).length" :data-testid="`select-account-models-${target.account_id}`" class="btn btn-secondary btn-sm" @click="toggleModels(targetModelKeys(target))">{{ tt(modelsSelected(targetModelKeys(target)) ? 'clearAccountSelection' : 'selectAccountModels') }}</button>
          </div>
          <p v-if="!target.models?.length" class="text-xs text-gray-500">{{ target.unavailable_reason || tt('noModels') }}</p>
          <div class="acceptance-model-list" :class="{ expanded: showMappings }">
          <div v-for="model in target.models" :key="model.model" class="acceptance-model">
            <label class="flex items-start gap-2 text-sm">
              <input v-model="selected" :disabled="Boolean(model.unavailable_reason)" type="checkbox" class="mt-1" :value="modelKey(target.account_id, model.model)" :data-testid="`model-${target.account_id}-${model.model}`" />
              <span :class="showMappings ? 'acceptance-model-fields' : 'min-w-0 break-all'"><strong>{{ model.model }}</strong><span v-if="showMappings && model.upstream_model !== model.model" class="text-gray-500"> → {{ model.upstream_model }}</span><span v-if="showMappings" class="mt-1 block text-xs text-gray-500">{{ model.protocols?.join(' / ') || tt('unsupported') }}</span></span>
            </label>
            <p v-if="model.unavailable_reason" class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ model.unavailable_reason }}</p>
            <p v-else-if="!hasCostEstimate(model)" class="mt-2 text-xs text-amber-700 dark:text-amber-400">{{ tt('noCostEstimate') }}</p>
          </div>
          </div>
        </fieldset>
        <p class="text-xs text-gray-500">{{ tt('standard') }}</p>
      </section>

      <section v-else class="space-y-3">
          <section class="acceptance-console" data-testid="acceptance-console" :aria-label="tt('executionLog')">
            <header class="console-head">
              <h4 class="min-w-0 font-semibold"><span aria-hidden="true" class="console-prompt">›_</span> {{ tt('executionLog') }}</h4>
              <div class="console-actions"><button :data-testid="`export-${report.id}`" class="btn btn-secondary btn-sm" :disabled="exporting" @click="exportReport(report.id)">{{ tt('export') }}</button><button v-if="canCancel(report)" :data-testid="`cancel-${report.id}`" class="btn btn-danger btn-sm" :disabled="cancelling || report.status === 'cancelling'" @click="cancelRun(report.id)">{{ tt('cancel') }}</button></div>
            </header>
            <div class="console-progress">
              <span aria-live="polite">{{ statusLabel(report.status) }}</span>
              <progress class="acceptance-progress" :value="report.completed" :max="Math.max(1, report.total)" :aria-label="tt('progress')" />
              <b>{{ report.completed }} / {{ report.total }}</b>
            </div>
            <p v-if="modelOverview.length === 1" class="console-target">#{{ modelOverview[0].target.account_id }} {{ modelOverview[0].target.model }}</p>
            <div ref="consoleBody" class="acceptance-console-body" @scroll.passive="trackConsolePosition">
              <p v-if="!logSamples.length && isActive(report)" class="console-target">{{ tt('awaitingSample') }}</p>
              <TransitionGroup :key="report.id" name="console-line">
                <details v-for="sample in logSamples" :key="sample.id" class="acceptance-console-item">
                  <summary class="acceptance-console-row">
                    <span class="console-line-number">{{ Number.isInteger(sample.ordinal) ? sample.ordinal + 1 : "·" }}</span>
                    <span class="acceptance-console-state" :data-state="sample.result?.status ?? sample.status">{{ statusLabel(sample.result?.status ?? sample.status) }}</span>
                    <span class="min-w-0">
                      <span class="acceptance-console-meta">{{ categoryLabel(sample.category) }} · {{ sample.protocol }} · {{ sample.stream ? tt('stream') : tt('nonstream') }}</span>
                      <Transition name="console-result"><span v-if="sample.result" class="block truncate" :title="observedResult(sample)">{{ observedResult(sample) }}</span></Transition>
                      <span v-if="modelOverview.length > 1" class="block break-words text-xs text-gray-500">#{{ sample.target.account_id }} {{ sample.target.model }}</span>
                    </span>
                    <span v-if="sample.result?.duration_ms != null" class="acceptance-console-meta whitespace-nowrap">{{ sample.result.duration_ms }} ms</span>
                  </summary>
                  <dl class="console-reading">
                    <dt>{{ tt('reading.purpose') }}</dt><dd>{{ tt(`reading.checks.${metricCategory(sample.category)}`) }}</dd>
                    <template v-if="sample.result">
                      <dt>{{ tt('reading.result') }}</dt><dd>{{ observedResult(sample) }}</dd>
                      <dt>{{ tt('reading.analysis') }}</dt>
                      <dd><template v-if="sample.result.findings?.length"><p v-for="finding in sample.result.findings" :key="finding.code">{{ finding.message }}</p></template>
                        <template v-else>{{ sampleAnalysis(sample) }}</template></dd>
                      <dt>{{ tt('reading.conclusion') }}</dt><dd>{{ statusLabel(sample.result.status) }}</dd>
                    </template>
                    <template v-else><dt>{{ tt('reading.result') }}</dt><dd>{{ statusLabel(sample.status) }}</dd></template>
                  </dl>
                  <template v-if="sample.result">
                    <details class="mt-2 text-xs"><summary class="cursor-pointer">{{ tt('sampleDetails') }}</summary>
                    <p class="mt-2 break-all text-xs">{{ tt('source') }}: {{ sourceLabel(sample.result.source?.kind) }} · {{ sample.result.source?.channel || '—' }} · {{ sample.result.source?.host }}{{ sample.result.source?.path }}</p>
                    <p class="mt-1 break-all text-xs text-gray-500">{{ tt('groupContext') }}: {{ sample.target.group_id ? `#${sample.target.group_id}` : tt('noGroup') }} · {{ tt('adapter') }}: {{ sample.result.source?.adapter || '—' }} / {{ sample.result.source?.auth_kind || '—' }} · {{ sample.result.source?.requested_model }} → {{ sample.result.source?.upstream_model }} → {{ sample.result.source?.response_model || '—' }}</p>
                    <div class="mt-2 overflow-x-auto"><table class="w-full min-w-[480px] text-right text-xs"><thead><tr><th class="p-2 text-left">{{ tt('usage') }}</th><th v-for="field in usageFields" :key="field" class="p-2">{{ tt(field) }}</th></tr></thead><tbody><tr v-for="layer in usageLayers" :key="layer.key" class="border-t border-gray-100 dark:border-dark-700"><th class="p-2 text-left font-normal">{{ tt(layer.label) }}<span class="block text-gray-500">{{ sample.result[layer.key]?.source }}</span><span v-if="sample.result[layer.key]?.output_includes_thoughts" class="mt-1 block text-amber-700 dark:text-amber-400">{{ tt('aggregateOutputHint') }}</span></th><td v-for="field in usageFields" :key="field" class="p-2" :data-testid="`${layer.label}-${field}-${sample.id}`">{{ number(sample.result[layer.key]?.[field]) }}</td></tr></tbody></table></div>
                    <p class="mt-1 text-xs text-gray-500">{{ tt('missingHint') }}</p>
                    <p class="mt-2 text-xs">HTTP {{ sample.result.http_status }} · {{ sample.result.finish_reason || '—' }} · {{ tt('calculated') }} {{ money(sample.result.cost_usd) }}</p>
                    </details>

                    <details class="mt-2 text-xs"><summary class="cursor-pointer">{{ tt('evidence') }}</summary><pre class="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-all rounded bg-slate-100 p-2 text-slate-800 dark:bg-slate-800 dark:text-slate-200">{{ JSON.stringify({ parameters: sample.result.effective_parameters, usage_frames: sample.result.usage_frames, excerpt: sample.result.output_excerpt, price: sample.target.price }, null, 2) }}</pre></details>
                  </template>
                </details>
              </TransitionGroup>
              <span v-if="isActive(report)" class="console-caret" aria-hidden="true"></span>
            </div>
            <footer class="console-cost"><span>{{ tt('estimated') }} {{ money(report.estimated_cost_usd) }}</span><span>{{ tt('calculated') }} {{ money(report.spent_cost_usd) }}</span></footer>
          </section>
          <p v-if="report.stale_config" class="text-sm text-amber-700 dark:text-amber-400">{{ tt('stale') }}</p>
          <section v-if="!isActive(report) && modelOverview.length" :aria-label="tt('modelOverview')" class="space-y-1">
            <div class="flex flex-wrap items-baseline justify-between gap-2">
              <h5 class="text-sm font-semibold">{{ tt('modelOverview') }}</h5>
              <button v-if="anomalousModels.length" data-testid="retest-anomalies" class="btn btn-secondary btn-sm"
                :disabled="Boolean(retestReason) || starting || loading" @click="launch(true)">{{ tt('retestAnomalies', { count: anomalousModels.length }) }}</button>
            </div>
            <p v-if="anomalousModels.length && retestReason" class="text-xs text-gray-500">{{ retestReason }}</p>
            <div v-for="row in modelOverview" :key="row.key" data-testid="model-overview-row" class="acceptance-overview-row text-xs">
              <div class="min-w-0">
                <strong class="block break-all">{{ row.target.model }}</strong>
                <span class="text-gray-500">#{{ row.target.account_id }} {{ row.target.account_name }}<template v-if="row.target.group_id"> · {{ tt('groupContext') }} #{{ row.target.group_id }}</template></span>
                <span v-if="row.anomalies" class="block text-amber-700">{{ tt('flaggedSamples', { count: row.anomalies }) }}</span>
              </div>
              <div class="min-w-0">
                <strong>{{ tt('source') }}</strong>
                <p v-if="row.target.declared_source">{{ tt('declaredSource') }} · {{ tt(`declaredSources.${row.target.declared_source}`) }}</p>
                <p v-for="[key, source] in row.sources" :key="key" class="break-words">{{ sourceLabel(source.kind) }}<template v-if="source.channel && source.channel !== 'unknown'"> · {{ source.channel }}</template></p>
                <p v-if="!row.sources.size" class="text-gray-500">—</p>
                <AcceptanceFindingList :findings="row.sourceFindings" />
              </div>
              <div class="min-w-0">
                <strong>{{ tt('usage') }}</strong>
                <p class="text-gray-500">{{ row.generations ? tt('measuredSamples', { measured: row.measured, total: row.generations }) : tt('noUsageResults') }}</p>
                <span v-if="cacheSummaries.has(row.key)" class="block text-gray-500" :title="tt('cacheConditions')">{{ tt('cacheRate') }} {{ cacheSummaries.get(row.key) }} </span>
                <AcceptanceFindingList :findings="row.findings" />
              </div>
            </div>
          </section>

      </section>
    </div>
    <template #footer>
      <div class="w-full min-w-0 space-y-2">
        <p v-if="error" role="alert" class="break-words text-sm text-red-600 dark:text-red-400">{{ error }}</p>
        <p v-if="!report && authStore.isAdmin && !loading && validationMessage" role="status" class="break-words text-xs text-amber-700 dark:text-amber-400">{{ validationMessage }}</p>
        <div class="flex flex-wrap justify-end gap-2">
          <button class="btn btn-secondary" @click="emit('close')">{{ tt('close') }}</button>
          <button v-if="report && !isActive(report)" data-testid="new-detection" class="btn btn-secondary" :disabled="starting" @click="resetDetection">{{ tt('newDetection') }}</button>
          <button v-if="!report && authStore.isAdmin" data-testid="start" class="btn btn-primary" :disabled="!validRequest || starting || loading" @click="launch(false)">{{ tt(starting ? 'loading' : 'startDetection') }}</button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { channelAcceptanceApi as api, type AcceptanceDeclaredSource, type AcceptanceExtra, type AcceptanceModel, type AcceptancePreview, type AcceptancePreviewRequest, type AcceptanceRun, type AcceptanceSample, type AcceptanceTarget } from '@/api/admin/channelAcceptance'

import AcceptanceFindingList from './AcceptanceFindingList.vue'
import { reportTargetKey, summarizeAcceptanceModels } from './acceptanceReportModels'

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
// Terminal run labels can precede settlement of already-dispatched requests.
// The repository counts cancelled/skipped samples as completed, so this delta
// only keeps polling for outstanding samples, including summary-only list rows.
const isActive = (run: AcceptanceRun) => ['queued', 'pending', 'running', 'cancelling'].includes(run.status)
  || Boolean(run.samples?.some(sample => sample.status === 'running'))
  || (['budget_exhausted', 'cancelled'].includes(run.status) && run.completed < run.total)
const canCancel = (run: AcceptanceRun) => isActive(run) && !['cancelled', 'cancelling'].includes(run.status)
const modelKey = (id: number, model: string) => JSON.stringify([id, model])
const declaredSourceOptions: AcceptanceDeclaredSource[] = ['unknown', 'vertex', 'google_developer_api', 'google_code_assist', 'antigravity', 'anthropic', 'claude_max', 'kiro', 'bedrock']
const sourceOptions = computed(() => [
  { value: '', label: tt('sourceOff') },
  ...declaredSourceOptions.map(value => ({ value, label: tt(`declaredSources.${value}`) }))
])
const declarations = ref<Record<number, AcceptanceDeclaredSource | ''>>({})
const extraOptions: AcceptanceExtra[] = ['long_context', 'image', 'audio', 'tools']
const usageFields = ['input', 'output', 'thoughts', 'cache_read', 'cache_create', 'total'] as const
const usageLayers = [{ key: 'raw_usage', label: 'raw' }, { key: 'client_usage', label: 'client' }, { key: 'billing_usage', label: 'billing' }] as const
const targets = ref<AcceptanceTarget[]>([])
const showMappings = ref(false)
const selected = ref<string[]>([])
const report = ref<AcceptanceRun | null>(null)
const modelOverview = computed(() => summarizeAcceptanceModels(report.value?.samples ?? []))
// A lower ordinal can start later when a different account slot becomes free.
const logOrder = new Map<string, number>()
const consoleBody = ref<HTMLElement | null>(null)
let followTail = true
watch(() => report.value?.id, () => { logOrder.clear(); followTail = true }, { flush: 'sync' })
watch(report, run => {
  if (!run) return
  const active = isActive(run)
  for (const sample of run.samples ?? []) {
    if (!logOrder.has(sample.id) && (!active || sample.result || !['pending', 'queued'].includes(sample.status))) {
      logOrder.set(sample.id, logOrder.size)
    }
  }
}, { flush: 'sync' })
const logSamples = computed(() => (report.value?.samples ?? []).filter(s => logOrder.has(s.id))
  .sort((a, b) => logOrder.get(a.id)! - logOrder.get(b.id)!))
function trackConsolePosition() {
  const el = consoleBody.value
  if (el) followTail = el.scrollHeight - el.scrollTop - el.clientHeight <= 12
}
watch([report, consoleBody], ([run, el], [previous]) => {
  if (el && run && followTail && (isActive(run) || (previous?.id === run.id && isActive(previous)))) el.scrollTop = el.scrollHeight
}, { flush: 'post' })

function metricCategory(category: string) {
  if (category.startsWith('cache')) return 'cache'
  if (category === 'input_tokens') return 'input'
  if (category === 'challenge') return 'repeat'
  return ['count', 'input', 'protocol', 'repeat', 'tools', 'long_context', 'image', 'audio'].includes(category) ? category : 'other'
}
function observedResult(s: AcceptanceSample) {
  const r = s.result!
  const raw = r.raw_usage
  switch (metricCategory(s.category)) {
    case 'cache': return tt('reading.cache', { cached: number(raw?.cache_read), total: number(raw?.input) })
    case 'count': case 'input': return tt('reading.tokens', { raw: number(raw?.input), client: number(r.client_usage?.input) })
    default: return [r.http_status ? `HTTP ${r.http_status}` : '', r.finish_reason, r.output_excerpt?.slice(0, 100),
      r.has_tool ? tt('reading.tool') : ''].filter(Boolean).join(' · ') || '—'
  }
}
function sampleAnalysis(s: AcceptanceSample) {
  const r = s.result!
  if (r.status !== 'passed' || !r.completed) return tt('reading.insufficient')
  switch (metricCategory(s.category)) {
    case 'cache':
      if (!r.upstream_usage_final || r.raw_usage?.cache_read == null) return tt('reading.insufficient')
      return tt(r.raw_usage.cache_read > 0 ? 'reading.hit' : 'reading.miss')
    case 'count': return tt('reading.baseline')
    case 'input': return tt('reading.compareInputs')
    case 'audio': return tt('reading.audioLimit')
    default: return tt('reading.challengePassed')
  }
}
const staleRetestId = ref('')
const anomalousModels = computed(() => modelOverview.value.filter(row => row.anomalies > 0))
const loading = ref(false)
const starting = ref(false)
const refreshing = ref(false)
const cancelling = ref(false)
const exporting = ref(false)
const error = ref('')
let disposed = false
let epoch = 0
let creatingScope: number | null = null
let requestRevision = 0
let reportRevision = 0
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
  targets: targets.value.flatMap(target => {
    const source = declarations.value[target.account_id]
    return (target.models || [])
      .filter(model => selected.value.includes(modelKey(target.account_id, model.model)))
      .map(model => ({ account_id: target.account_id, model: model.model, ...(source ? { declared_source: source } : {}) }))
  }),
  extras: [...extraOptions],
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
const retestReason = computed(() => {
  const run = report.value
  if (run && run.rule_version !== 'channel-acceptance-v3') return tt('retestRuleChanged')
  if (!run || !['completed', 'budget_exhausted', 'cancelled'].includes(run.status) || !(run.completed >= run.total)) {
    return tt('retestInProgress')
  }
  if (run.stale_config || run.id === staleRetestId.value || modelOverview.value.some(({ target }) =>
    !props.accountIds.includes(target.account_id) || !allModelKeys.value.includes(modelKey(target.account_id, target.model)))) {
    return tt('retestUnavailable')
  }
  return anomalousModels.value.length ? '' : tt('noAnomalies')
})
// Retain the same server plan for retries after an ambiguous start response.
let pendingPlan: AcceptancePreview | null = null
watch([request, () => report.value?.id, retestReason, () => props.show], () => {
  requestRevision++
  pendingPlan = null
}, { deep: true, flush: 'sync' })
function fail(value: unknown) {
  const message = value as { message?: string; response?: { data?: { message?: string } } }
  error.value = message?.response?.data?.message || message?.message || tt('failed')
}
async function launch(retest: boolean) {
  if (!usable() || loading.value || starting.value) return
  if (retest ? !report.value || !!retestReason.value : !validRequest.value) return
  const scope = epoch
  const revision = requestRevision
  const current = () => usable() && scope === epoch && revision === requestRevision
  const runId = report.value?.id
  starting.value = true
  error.value = ''
  try {
    const plan = pendingPlan ?? (retest && runId ? await api.retestPreview(runId) : await api.preview(request.value))
    if (!current()) return
    if (!plan.can_start || (!pendingPlan && !(Date.parse(plan.expires_at) > Date.now()))) {
      pendingPlan = null
      error.value = plan.can_start ? tt('previewExpired') : plan.issues?.join(' · ') || tt('failed')
      return
    }
    if (retest) {
      const planned = new Set(plan.samples?.map(sample => reportTargetKey(sample.target)))
      if (planned.size !== anomalousModels.value.length || anomalousModels.value.some(row => !planned.has(row.key))) {
        error.value = tt('retestScopeMismatch')
        return
      }
    }
    pendingPlan = plan
    creatingScope = scope
    const run = await api.start(plan.id)
    if (creatingScope !== scope || disposed || !authStore.isAdmin) return
    pendingPlan = null
    reportRevision++
    report.value = run
    poll()
  } catch (e) {
    if (current() || creatingScope === scope) {
      if ((e as { reason?: string } | null)?.reason === 'ACCEPTANCE_STALE') {
        pendingPlan = null
        if (retest && runId) staleRetestId.value = runId
      }
      fail(e)
    }
  } finally {
    if (scope === epoch || creatingScope === scope) starting.value = false
    if (creatingScope === scope) creatingScope = null
  }
}
async function refresh() {
  if (!usable() || !report.value || refreshing.value) return
  const id = report.value.id
  const scope = epoch
  const version = reportRevision
  refreshing.value = true
  try {
    const detail = await api.get(id)
    if (scope === epoch && usable() && report.value?.id === id && version === reportRevision) {
      report.value = detail
      error.value = ''
    }
  } catch (e) { if (scope === epoch) fail(e) }
  finally { if (scope === epoch) refreshing.value = false }
}
function poll() {
  clearTimeout(timer)
  if (!usable() || !report.value || !isActive(report.value)) return
  const scope = epoch
  timer = setTimeout(async () => {
    await refresh()
    if (scope === epoch) poll()
  }, 3000)
}
function resetDetection() {
  clearTimeout(timer)
  reportRevision++
  report.value = null
  error.value = ''
  staleRetestId.value = ''
}
async function cancelRun(id: string) {
  if (!usable() || cancelling.value) return
  const scope = epoch
  cancelling.value = true; reportRevision++; error.value = ''
  try { const run = await api.cancel(id); if (usable() && scope === epoch) { reportRevision++; if (report.value?.id === id) report.value = run } }
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
const cacheSummaries = computed(() => {
  const rows = new Map<string, { cached: number; total: number; hits: number; evaluable: number }>()
  for (const sample of report.value?.samples || []) {
    if (!sample.category.startsWith('cache') || !sample.result) continue
    const key = reportTargetKey(sample.target)
    const row = rows.get(key) || { cached: 0, total: 0, hits: 0, evaluable: 0 }
    const usage = sample.result?.raw_usage
    const minimum = sample.result?.effective_parameters?.cache_min_input_tokens
    const knownMinimum = typeof minimum === 'number' && Number.isFinite(minimum) && minimum > 0
    const cacheEligible = usage?.cache_read != null && (usage.cache_read > 0 || (knownMinimum && usage.input != null && usage.input >= minimum))
    if (sample.result?.completed && sample.result.upstream_usage_final === true && cacheEligible && usage?.source && !/unknown|estimat|fallback/i.test(usage.source) && usage?.input != null && Number.isFinite(usage.input) && usage.input > 0 && usage.cache_read != null && Number.isFinite(usage.cache_read) && usage.cache_read >= 0 && usage.cache_read <= usage.input) {
      row.total += usage.input; row.cached += usage.cache_read; row.evaluable++; if (usage.cache_read > 0) row.hits++
    }
    rows.set(key, row)
  }
  return new Map([...rows].map(([key, row]) => [key, row.evaluable
    ? `${(row.cached / row.total * 100).toFixed(1)}% · ${tt('cacheHits')} ${row.hits}/${row.evaluable}`
    : tt('notAssessable')]))
})
let currentAccounts = ''
watch([() => props.show, () => props.accountIds.join(','), () => authStore.isAdmin], async () => {
  showMappings.value = false
  const scope = ++epoch
  clearTimeout(timer)
  refreshing.value = false
  cancelling.value = false
  if (props.accountIds.join(',') !== currentAccounts || !authStore.isAdmin) {
    creatingScope = null
    currentAccounts = props.accountIds.join(',')
    resetDetection()
    selected.value = []
    declarations.value = {}
  }
  starting.value = creatingScope !== null
  if (!usable()) return
  loading.value = true
  error.value = ''
  pendingPlan = null
  try {
    const availableTargets = await api.targets([...props.accountIds])
    if (scope !== epoch || !usable()) return
    targets.value = availableTargets
    const available = new Set(allModelKeys.value)
    selected.value = selected.value.filter(key => available.has(key))
  } catch (e) {
    if (scope !== epoch) return
    targets.value = []
    fail(e)
  } finally {
    if (scope === epoch) loading.value = false
  }
  if (scope === epoch && report.value && isActive(report.value)) {
    await refresh()
    if (scope === epoch) poll()
  }
}, { immediate: true })
onUnmounted(() => { disposed = true; epoch++; clearTimeout(timer) })
</script>


<style scoped>
.acceptance-source-select { width: min(13rem, 100%); }
.acceptance-source-select :deep(.select-trigger) {
  min-height: 2rem;
  padding: .3rem .6rem;
  font: inherit;
}
.acceptance-overview-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.5fr);
  gap: .35rem 1rem;
  padding: .65rem 0;
  border-top: 1px solid var(--term-line);
}
@media (max-width: 640px) {
  .acceptance-overview-row { grid-template-columns: 1fr; }
}
.acceptance-progress { display: block; width: 100%; height: 4px; accent-color: #0891b2; }
.acceptance-console {
  color-scheme: dark;
  --term-surface: #101820;
  --term-subtle: #18232d;
  --term-ink: #dce7f0;
  --term-muted: #a2b3c2;
  --term-line: #34434f;
  --term-accent: #74d8f5;
  --term-success: #81d6a0;
  --term-warning: #ecc77f;
  --term-error: #ff9a9a;
  overflow: hidden;
  border: 1px solid var(--term-line);
  border-radius: 8px;
  background: var(--term-surface);
  color: var(--term-ink);
  font-family: "IBM Plex Mono", ui-monospace, SFMono-Regular, Consolas, monospace;
}
.console-head, .console-actions, .console-cost { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.console-head { justify-content: space-between; padding: 8px 12px; background: var(--term-subtle); border-bottom: 1px solid var(--term-line); font-size: 13px; }
.console-prompt { color: var(--term-accent); margin-right: 4px; }
.console-progress { display: grid; grid-template-columns: auto minmax(36px, 1fr) auto; align-items: center; gap: 12px; padding: 12px; font-size: 12px; }
.console-target, .console-cost { padding: 4px 12px 8px; overflow-wrap: anywhere; color: var(--term-muted); font-size: 12px; }
.console-cost { padding: 8px 12px; border-top: 1px solid var(--term-line); column-gap: 20px; }
.console-line-number { text-align: right; color: var(--term-muted); }
.acceptance-console-body { height: min(48vh, 27rem); height: min(48svh, 27rem); overflow: auto; scrollbar-gutter: stable; overscroll-behavior: contain; }
.acceptance-console-item { padding: 0 12px; border-bottom: 1px solid var(--term-line); }
.console-caret {
  display: block;
  width: .55em;
  height: .95em;
  margin: .6rem .75rem;
  background: var(--term-accent);
}
@media (prefers-reduced-motion: no-preference) {
  .console-line-enter-active,
  .console-result-enter-active { animation: console-write 260ms steps(18, end); }
  .console-caret { animation: console-blink 1.2s step-end infinite; }
}
@keyframes console-write {
  from { clip-path: inset(0 100% 0 0); }
  to { clip-path: inset(0); }
}
@keyframes console-blink {
  50% { opacity: .22; }
}
.acceptance-console-item:last-child { border-bottom: 0; }
.acceptance-console-item[open] { padding-bottom: 12px; background: var(--term-subtle); }
.acceptance-console-row {
  display: grid;
  grid-template-columns: 3ch minmax(5.5rem, auto) minmax(0, 1fr) auto;
  align-items: start;
  gap: 10px;
  padding: 9px 0;
  cursor: pointer;
  font-size: 12px;
}
.acceptance-console-row::-webkit-details-marker { display: none; }
.acceptance-console-row:focus-visible { outline: 1px solid var(--term-accent); outline-offset: -1px; }
.acceptance-console-meta, .acceptance-console-state { color: var(--term-muted); }
.acceptance-console-state::before { display: inline-block; margin-right: 6px; content: "›"; }
.acceptance-console-item[open] .acceptance-console-state::before { transform: rotate(90deg); }
.acceptance-console-state[data-state="passed"] { color: var(--term-success); }
.acceptance-console-state[data-state="running"] { color: var(--term-accent); }
.acceptance-console-state[data-state="error"], .acceptance-console-state[data-state="anomaly"], .acceptance-console-state[data-state="failed"] { color: var(--term-error); }
.acceptance-console-item :deep(.text-gray-500), .acceptance-console-item :deep(.text-gray-600) { color: var(--term-muted); }
.console-reading { display: grid; grid-template-columns: max-content minmax(0,1fr); gap: 4px 12px; margin: 3px 0 8px; font-size: 12px; line-height: 1.55; }
.console-reading dt { color: var(--term-accent); }
.console-reading dd { margin: 0; overflow-wrap: anywhere; }
.acceptance-console-item pre { background: var(--term-surface); color: var(--term-ink); }
.acceptance-console-state[data-state="anomaly"], .acceptance-console-state[data-state="inconclusive"] { color: var(--term-warning); }
.acceptance-console-item :deep(tr) { border-color: var(--term-line); }
.acceptance-console-item :deep(.text-red-600) { color: var(--term-error); }
.acceptance-console-item :deep(.text-amber-700) { color: var(--term-warning); }
@media (max-width: 480px) {
  .acceptance-console-row { grid-template-columns: 2ch auto minmax(0, 1fr); gap: 6px; }
  .acceptance-console-row > :nth-child(4) { grid-column: 3; }
}
</style>

<style>
/* The dialog is teleported. Scope its terminal palette to this panel. */
.modal-content:has(.acceptance) {
  --term-line: #dce2e8;
  --term-muted: #596779;
  --term-accent: #245dc2;
  --term-surface: #ffffff;
  --term-ink: #202b38;
  --term-subtle: #f6f8fa;
  --term-selected: #eff5ff;
  --term-primary: #265bc1;
  --term-on-primary: #ffffff;
  --term-warning: #895200;
  --term-error: #b42338;
  --term-success: #14714a;
  color-scheme: light;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-variant-numeric: tabular-nums;
}
.dark .modal-content:has(.acceptance) {
  --term-line: #344350;
  --term-muted: #a2b2bd;
  --term-accent: #82dec3;
  --term-surface: #151c23;
  --term-ink: #e5edf3;
  --term-subtle: #222d36;
  --term-selected: #1d302e;
  --term-primary: #98e0ca;
  --term-on-primary: #102921;
  --term-warning: #f2c783;
  --term-error: #ffa7ab;
  --term-success: #6ee7b7;
  color-scheme: dark;
}
.modal-content:has(.acceptance),
.modal-content:has(.acceptance) :is(.modal-header,.modal-body,.modal-footer) {
  background: var(--term-surface) !important;
  border-color: var(--term-line) !important;
  color: var(--term-ink);
}
.modal-content:has(.acceptance) :is(.modal-title,legend,h4) {color: var(--term-ink)}
.modal-content:has(.acceptance) .modal-title::before {content: '›_'; color: var(--term-accent); margin-right: .7em}
.modal-content:has(.acceptance) :is(.text-gray-400,.text-gray-500,.text-gray-600) {color: var(--term-muted)}
.modal-content:has(.acceptance) .text-amber-700 {color: var(--term-warning)}
.modal-content:has(.acceptance) .text-red-600 {color: var(--term-error)}
.modal-content:has(.acceptance) :is(.btn-secondary,.btn-primary) {
  background: var(--term-subtle) !important; color: var(--term-ink) !important;
  border: 1px solid var(--term-line) !important; border-radius: 4px; box-shadow: none;
}
.modal-content:has(.acceptance) .btn-primary {background: var(--term-primary) !important; color: var(--term-on-primary) !important}
.modal-content:has(.acceptance) .btn:disabled {opacity: .48}
.modal-content:has(.acceptance) .btn:enabled:hover {filter: brightness(1.14)}
.acceptance :is(nav,fieldset,section,table,tr) {border-color: var(--term-line)}
.acceptance input[type=checkbox] {accent-color: var(--term-accent)}
.acceptance-model-list:not(.expanded) {display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,20rem),1fr));column-gap:1rem}
.acceptance-model {margin: 0 !important; border-top: 1px solid var(--term-line); padding: .65rem .45rem}
.acceptance-model:has(:checked) {background: var(--term-selected)}
.acceptance-model label {cursor: pointer}
.acceptance-model-fields {flex: 1; min-width: 0; display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr) minmax(8rem,.7fr); align-items: baseline; gap: .6rem; overflow-wrap: anywhere}
.acceptance-model-fields > :last-child {grid-column: 3; margin: 0; text-align: right}
.acceptance legend {padding-right: .5rem}
.acceptance input:focus-visible,.acceptance button:focus-visible {outline: 2px solid var(--term-accent); outline-offset: 2px}
@media(max-width:720px){
  .acceptance-model-fields {grid-template-columns: minmax(0,1fr); gap: .15rem}
  .acceptance-model-fields > :last-child {grid-column: 1; text-align: left}
}
</style>
