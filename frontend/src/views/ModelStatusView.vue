<template>
  <component :is="authStore.isAuthenticated ? AppLayout : 'div'" class="model-status-page model-status-workspace" :class="{ 'status-guest': !authStore.isAuthenticated, 'status-capture-mode': captureMode }">
    <PlazaNavBar v-if="!standalone && !authStore.isAuthenticated && !captureMode" login-redirect="/model-status" />
    <component :is="authStore.isAuthenticated ? 'section' : 'main'" class="status-content" :class="{ 'status-content-embedded': authStore.isAuthenticated }" :aria-busy="loading">
      <h1 class="status-page-title" :class="{ 'sr-only': authStore.isAuthenticated }">{{ t('modelStatus.title') }}</h1>
      <div v-if="!captureMode" class="status-toolbar-shell workspace-toolbar">
        <div class="status-filters">
          <label class="model-search"><Icon name="search" size="sm" aria-hidden="true" /><input v-model="modelSearch" type="text" role="searchbox" inputmode="search" :placeholder="t('modelStatus.searchPlaceholder')" :aria-label="t('modelStatus.searchPlaceholder')" data-testid="model-search" /><button v-if="modelSearch" type="button" class="model-search-clear" :aria-label="t('modelStatus.clearSearch')" @click="modelSearch = ''"><Icon name="x" size="sm" /></button></label>
          <Select v-model="groupFilter" class="group-filter" :options="groupOptions" :aria-label="t('modelStatus.group')" mobile-sheet>
            <template #selected>
              <span class="group-filter-selected-text">{{ groupOptions.find(option => option.value === groupFilter)?.label ?? t('modelStatus.allGroups') }}</span>
              <span class="group-filter-mobile-icon" aria-hidden="true"><Icon name="filter" size="sm" /></span>
            </template>
          </Select>
          <div v-if="activeGroup && showGroupContext" class="group-context" aria-live="polite">
            <span class="group-context-name">{{ activeGroup.name }}</span>
          </div>
          <span v-if="report" class="model-count">{{ t('modelStatus.modelCount', { count: filteredModelCount }) }}</span>
          <div class="status-toolbar-actions">
            <span class="refresh-countdown" data-testid="refresh-countdown" aria-live="polite" aria-atomic="true">
              {{ loading ? t('modelStatus.refreshing') : t('modelStatus.autoRefreshIn', { seconds: refreshCountdown }) }}
            </span>
            <a v-if="standalone" class="status-home-link" href="https://api.passionapi.com/">Passion API</a>
            <button
              class="btn btn-secondary btn-icon refresh-button"
              type="button"
              data-testid="refresh"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="refreshAll"
            >
              <Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" />
            </button>
          </div>
        </div>
        <div v-if="customGroupsLoadFailed" data-testid="custom-group-load-warning" role="status" class="status-notice status-notice-warning custom-group-load-warning">
          <Icon name="exclamationTriangle" size="sm" class="shrink-0" />
          <span>{{ t('modelStatus.customGroupsLoadFailed') }}</span>
        </div>
      </div>

      <div v-if="loadFailed" role="alert" class="status-notice status-notice-error">
        <Icon name="exclamationTriangle" size="md" class="shrink-0" />
        <span>{{ t(report ? 'modelStatus.refreshFailed' : 'modelStatus.loadFailed') }}</span>
        <button data-testid="retry" type="button" class="btn btn-secondary btn-sm status-recovery-action" :disabled="loading" @click="loadReport">
          <Icon name="refresh" size="sm" />{{ t('common.retry') }}
        </button>
      </div>

      <div v-if="loading && !report" role="status" class="status-empty">
        <Icon name="refresh" size="lg" class="animate-spin" />
        <span>{{ t('modelStatus.loading') }}</span>
      </div>

      <template v-if="report">
        <div v-if="stale" role="status" class="status-notice status-notice-warning">
          <Icon name="clock" size="md" class="shrink-0" />
          <span>{{ t('modelStatus.staleData') }}</span>
        </div>

        <div v-if="!report.groups.length || !filteredGroups.length" class="status-empty workspace-empty-state">
          <Icon name="search" size="lg" class="text-gray-400" />
          <span>{{ t(emptyStateKey) }}</span>
          <button v-if="groupFilter && report.groups.length" data-testid="clear-filter" type="button" class="btn btn-secondary btn-sm status-recovery-action" @click="clearGroupFilter">
            <Icon name="x" size="sm" />{{ t('modelStatus.allGroups') }}
          </button>
        </div>

        <div v-if="visibleGroups.length" ref="statusGroups" class="status-groups">
          <section v-for="group in visibleGroups" :key="group.id" v-memo="[group, locale, bucketAnchorKey.startsWith(`${group.id}:`) ? bucketAnchorKey : '']" class="status-group" :aria-labelledby="`status-group-${group.id}`">
            <header class="group-heading" :data-group-id="group.id">
              <div class="group-title">
                <h2 :id="`status-group-${group.id}`">{{ group.name }}</h2>
              </div>
            </header>
            <div class="model-grid">
          <article v-for="model in group.visibleModels" :key="`${model.platform}:${model.name}`" class="card model-row workspace-surface" data-testid="model-row">
            <div class="model-row-header">
            <div class="model-identity model-identity-stack model-identity-minimal">
              <div class="model-title">
                <h3>{{ model.name }}</h3>
              </div>
              <span class="sr-only model-health"><span class="badge health-badge health-badge-readable" :class="healthBadgeClasses[modelHealthForDisplay(model)]">{{ t(`modelStatus.health.${modelHealthForDisplay(model)}`) }}</span></span>
              <span class="sr-only health-light" :class="{ 'health-light-observed': hasObservedOutcomes(modelMetricsForDisplay(model)) }" :data-health="modelHealthForDisplay(model)" :style="healthLightStyle(modelMetricsForDisplay(model))" />
              <span class="sr-only outcome-counts">{{ formatCount(modelMetricsForDisplay(model).success) }} {{ formatCount(modelMetricsForDisplay(model).failure) }} {{ formatCount(modelMetricsForDisplay(model).empty) }}</span>
            </div>
            </div>

            <div class="recent-results">
              <template v-if="model.buckets">
                <div class="recent-heading">
                  <span class="recent-heading-label">
                    <span>{{ t('modelStatus.fifteenMinuteBuckets') }}</span>
                  </span>
                  <span class="model-status-rate"><strong>{{ formatRate(modelMetricsForDisplay(model)) }}</strong></span>
                </div>
              <div class="recent-bars status-bucket-grid" :aria-label="t('modelStatus.fifteenMinuteBuckets')">
                <button
                  v-for="(bucket, index) in modelBuckets(model)"
                  :key="`${bucket.start_at}:${index}`"
                  type="button"
                  class="recent-bar bucket-light"
                  data-testid="status-bucket"
                  :class="`bucket-${bucketOutcome(bucket)}`"
                  :data-outcome="bucketOutcome(bucket) === 'unknown' || bucket.total <= 0 ? undefined : bucketOutcome(bucket)"
                  :aria-expanded="bucketAnchorKey === bucketKey(`${group.id}:${model.platform}:${model.name}`, bucket)"
                  :aria-label="bucketLabel(bucket)"
                  @pointerenter="$event.pointerType === 'mouse' && showBucket($event, bucket, `${group.id}:${model.platform}:${model.name}`)"
                  @pointerleave="$event.pointerType === 'mouse' && closeBucket()"
                  @pointerup="$event.isPrimary && $event.pointerType !== 'mouse' && showBucket($event, bucket, `${group.id}:${model.platform}:${model.name}`)"
                  @click="$event.detail === 0 && toggleBucket($event, bucket, `${group.id}:${model.platform}:${model.name}`)"
                  @blur="$event.currentTarget === bucketAnchor && closeBucket()"
                />
              </div>
              <div class="recent-heading recent-heading-axis"><span>{{ t('modelStatus.older') }}</span><span>{{ t('modelStatus.newer') }}</span></div>
              </template>
              <template v-else>
                <div class="recent-heading"><span>{{ t('modelStatus.recentRequests', { count: 30 }) }}</span><span class="model-status-rate"><strong>{{ formatRate(modelMetricsForDisplay(model)) }}</strong></span></div>
                <div class="recent-bars" :aria-label="t('modelStatus.recentRequests', { count: 30 })">
                  <span v-for="index in Math.max(0, 30 - (model.recent?.length ?? 0))" :key="`placeholder-${index}`" class="recent-placeholder" aria-hidden="true" />
                  <span v-for="(result, index) in model.recent ?? []" :key="`${result.at}:${index}`" class="recent-bar" :class="result.outcome === 'unknown' ? 'recent-incomplete' : `outcome-${result.outcome}`" :data-outcome="result.outcome === 'unknown' ? undefined : result.outcome" :title="resultLabel(result.at, result.outcome, result.status_code)" />
                </div>
                <div class="recent-heading recent-heading-axis"><span>{{ t('modelStatus.older') }}</span><span>{{ t('modelStatus.newer') }}</span></div>
              </template>
            </div>

          </article>
            </div>
          </section>
        </div>
        <span v-if="captureMode && !loading && !loadFailed" data-testid="model-status-ready" aria-hidden="true" />
        <div v-if="hasMoreModels" class="load-more-wrap">
          <button
            class="btn btn-secondary load-more-models"
            :class="{ 'is-loading': loadingMoreModels }"
            type="button"
            :disabled="loadingMoreModels"
            :aria-busy="loadingMoreModels"
            aria-live="polite"
            @click="loadMoreModels"
          >
            <Icon v-if="loadingMoreModels" name="refresh" size="sm" class="animate-spin" aria-hidden="true" />
            <span>{{ t(loadingMoreModels ? 'modelStatus.loadingMoreModels' : 'modelStatus.loadMoreModels') }}</span>
          </button>
          <span ref="loadMoreSentinel" class="load-more-sentinel" aria-hidden="true" />
        </div>
      </template>
    </component>
    <button
      v-if="showBackToTop && !captureMode"
      class="btn btn-primary btn-icon back-to-top"
      type="button"
      data-testid="back-to-top"
      :title="t('modelStatus.backToTop')"
      :aria-label="t('modelStatus.backToTop')"
      @click="scrollToTop"
    >
      <Icon name="arrowUp" size="sm" aria-hidden="true" />
    </button>
    <Teleport to="body">
      <div v-if="selectedBucket" ref="bucketPopover" data-testid="bucket-popover" class="bucket-popover" :style="bucketPosition" role="status">
        <p class="bucket-popover-time">{{ formatRange(selectedBucket.start_at, selectedBucket.end_at) }}</p>
        <div class="bucket-popover-status">
          <span class="bucket-popover-dot" :class="`bucket-${bucketOutcome(selectedBucket)}`" aria-hidden="true" />
          {{ bucketStatusLabel(selectedBucket) }}
        </div>
        <dl class="bucket-popover-counts">
          <div><dt>{{ t('modelStatus.outcome.success') }}</dt><dd>{{ formatCount(selectedBucket.success) }}</dd></div>
          <div><dt>{{ t('modelStatus.outcome.failure') }}</dt><dd>{{ formatCount(selectedBucket.failure) }}</dd></div>
        </dl>
      </div>
    </Teleport>
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import { getModelStatus, type ModelStatusBucket, type ModelStatusHealth, type ModelStatusMetrics, type ModelStatusModel, type ModelStatusOutcome, type ModelStatusResponse } from '@/api/modelStatus'
import { filterModelStatusGroups, normalizeModelStatusSearch } from './modelStatusSearch'
import { customGroupsAPI } from '@/api/customGroups'
import type { UserCustomGroup } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlazaNavBar from '@/components/modelPlaza/PlazaNavBar.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

defineProps<{ standalone?: boolean }>()

const { t, locale } = useI18n()
const countFormatter = computed(() => new Intl.NumberFormat(locale.value))
const rateFormatter = computed(() => new Intl.NumberFormat(locale.value, { maximumFractionDigits: 1 }))
const bucketStartFormatter = computed(() => new Intl.DateTimeFormat(locale.value, {
  month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false,
}))
const bucketEndFormatter = computed(() => new Intl.DateTimeFormat(locale.value, {
  hour: '2-digit', minute: '2-digit', hour12: false,
}))
const authStore = useAuthStore()
const appStore = useAppStore()
const report = ref<ModelStatusResponse | null>(null)
const customGroups = ref<UserCustomGroup[]>([])
const customGroupsReady = ref(false)
const customGroupsLoadFailed = ref(false)
const loading = ref(true)
const loadFailed = ref(false)
const captureMode = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('capture') === 'all'
const captureSearch = typeof window !== 'undefined' ? (new URLSearchParams(window.location.search).get('search') ?? '').trim() : ''
const modelSearch = ref(captureSearch)
const groupFilterStorageKey = 'model-status-group-filter'
const captureGroupFilter = typeof window !== 'undefined' ? (new URLSearchParams(window.location.search).get('group_id') ?? '').trim() : ''
const groupFilter = ref(captureMode ? captureGroupFilter : readStoredGroupFilter())
const visibleModelLimit = ref(40)
const loadingMoreModels = ref(false)
const loadMoreSentinel = ref<HTMLElement | null>(null)
const now = ref(Date.now())
const mobileBreakpoint = 767
const mobileHeaderHideThreshold = 0
const mobileHeaderHideDelta = 4
const backToTopThreshold = 320
const mobileStatusToolbarHeight = 86
const refreshIntervalSeconds = 30
const refreshCountdown = ref(refreshIntervalSeconds)
const mobileNavHidden = ref(false)
const showBackToTop = ref(false)
const activeGroupId = ref('')
const showGroupContext = ref(false)
const statusGroups = ref<HTMLElement | null>(null)
const healthBadgeClasses: Record<ModelStatusHealth, string> = {
  healthy: 'badge-success',
  degraded: 'badge-warning',
  unavailable: 'badge-danger',
  insufficient_data: 'badge-gray',
  no_data: 'badge-gray',
  unknown: 'badge-gray',
}
let request: AbortController | null = null
let customGroupsRequest: AbortController | null = null
let timer: ReturnType<typeof setInterval> | undefined
let refreshCountdownTimer: ReturnType<typeof setInterval> | undefined
let loadMoreObserver: IntersectionObserver | null = null
let loadMoreTimer: ReturnType<typeof setTimeout> | undefined
let groupContextObserver: IntersectionObserver | null = null
let disposed = false
let lastScrollY = 0

function readStoredGroupFilter(): string {
  try {
    return window.localStorage.getItem(groupFilterStorageKey) ?? ''
  } catch {
    return ''
  }
}

function persistGroupFilter(value: string) {
  try {
    if (value) {
      window.localStorage.setItem(groupFilterStorageKey, value)
    } else {
      window.localStorage.removeItem(groupFilterStorageKey)
    }
  } catch {
    // Storage can be unavailable in private browsing or restricted webviews.
  }
}

function isCustomGroupFilter(value: string): boolean {
  return value.startsWith('custom:')
}

function modelSourceKey(groupId: number, modelName: string): string {
  return `${groupId}:${modelName.trim().toLocaleLowerCase()}`
}

function authIdentity(): string {
  if (!authStore.isAuthenticated) return 'guest'
  return `user:${authStore.user?.id ?? 'unknown'}`
}

function validateGroupFilter(groups: ModelStatusResponse['groups']) {
  if (!groupFilter.value || isCustomGroupFilter(groupFilter.value)) return
  if (!groups.some(group => String(group.id) === groupFilter.value)) groupFilter.value = ''
}

function validateCustomGroupFilter() {
  if (!isCustomGroupFilter(groupFilter.value) || !customGroupsReady.value) return
  if (!selectedCustomGroup.value) groupFilter.value = ''
}

async function loadCustomGroups() {
  if (captureMode || !authStore.isAuthenticated || disposed) {
    customGroupsRequest?.abort()
    customGroupsRequest = null
    customGroups.value = []
    customGroupsReady.value = true
    customGroupsLoadFailed.value = false
    validateCustomGroupFilter()
    return
  }

  customGroupsRequest?.abort()
  const controller = new AbortController()
  const identity = authIdentity()
  customGroupsRequest = controller
  customGroups.value = []
  customGroupsReady.value = false
  customGroupsLoadFailed.value = false

  try {
    const groups = await customGroupsAPI.list({ signal: controller.signal })
    if (disposed || controller.signal.aborted || identity !== authIdentity()) return
    customGroups.value = groups.filter(group => group.status === 'active')
    customGroupsLoadFailed.value = false
    customGroupsReady.value = true
    validateCustomGroupFilter()
  } catch {
    if (controller.signal.aborted || disposed || identity !== authIdentity()) return
    customGroups.value = []
    customGroupsLoadFailed.value = true
    customGroupsReady.value = true
    validateCustomGroupFilter()
  } finally {
    if (customGroupsRequest === controller) customGroupsRequest = null
  }
}

// The backend keeps a completed report cached for five minutes. Allow a
// small buffer before warning so normal cache reuse is not reported as stale;
// the notice is for a genuinely stuck refresh (older than six minutes).
const staleReportThresholdMs = 6 * 60 * 1000
const stale = computed(() => !!report.value && !report.value.snapshot_at && now.value - Date.parse(report.value.generated_at) > staleReportThresholdMs)
const selectedBucket = ref<ModelStatusBucket | null>(null)
const bucketPopover = ref<HTMLElement | null>(null)
const bucketAnchorKey = ref('')
const bucketPosition = ref<CSSProperties>({ top: '0px', left: '0px', visibility: 'hidden' })
let bucketAnchor: HTMLElement | null = null
const activeCustomGroups = computed(() => customGroups.value.filter(group => group.status === 'active'))
const selectedCustomGroup = computed(() => {
  if (captureMode || !groupFilter.value.startsWith('custom:')) return null
  const id = Number(groupFilter.value.slice('custom:'.length))
  return activeCustomGroups.value.find(group => group.id === id) ?? null
})
const customGroupFilterPending = computed(() => isCustomGroupFilter(groupFilter.value) && !customGroupsReady.value)
const groupOptions = computed(() => [
  { value: '', label: t('modelStatus.allGroups'), platform: 'composite' },
  ...(report.value?.groups ?? []).map(group => ({ value: String(group.id), label: group.name, platform: group.platform })),
  ...(customGroupFilterPending.value ? [{ value: groupFilter.value, label: t('modelStatus.customGroupLoading'), platform: 'custom' }] : []),
  ...(!captureMode ? activeCustomGroups.value.map(group => ({
    value: `custom:${group.id}`,
    label: `${t('modelStatus.customGroupPrefix')}: ${group.name}`,
    platform: 'custom',
  })) : []),
])
const filteredGroups = computed(() => {
  const customSources = new Set(
    selectedCustomGroup.value?.models
      .filter(source => source.source_available !== false)
      .map(source => modelSourceKey(source.source_group_id, source.source_model)) ?? [],
  )
  const groups = (report.value?.groups ?? [])
    .filter(group => !groupFilter.value || customGroupFilterPending.value || selectedCustomGroup.value || String(group.id) === groupFilter.value)
    .map(group => {
      const models = selectedCustomGroup.value
        ? group.models.filter(model => customSources.has(modelSourceKey(group.id, model.name)))
        : group.models
      return models.length ? { ...group, models } : null
    })
    .filter((group): group is NonNullable<typeof group> => !!group)
  if (captureMode) {
    const query = normalizeModelStatusSearch(modelSearch.value)
    return groups.filter(group => !query || group.name.toLocaleLowerCase().includes(query) || group.models.some(model => `${model.name} ${model.platform}`.toLocaleLowerCase().includes(query)))
  }
  return filterModelStatusGroups(groups, modelSearch.value)
})
const filteredModelCount = computed(() => filteredGroups.value.reduce((count, group) => count + group.models.length, 0))
const emptyStateKey = computed(() => {
  if (!report.value?.groups.length) return 'modelStatus.noModels'
  if (selectedCustomGroup.value) return 'modelStatus.noCustomGroupMatches'
  if (normalizeModelStatusSearch(modelSearch.value)) return 'modelStatus.noSearchMatches'
  return 'modelStatus.noMatches'
})
const visibleGroups = computed(() => {
  if (captureMode) {
    return filteredGroups.value
      .filter(group => group.models.length)
      .map(group => ({ ...group, visibleModels: group.models }))
  }
  let remaining = visibleModelLimit.value
  return filteredGroups.value.flatMap(group => {
    const visibleModels = group.models.slice(0, remaining)
    remaining -= visibleModels.length
    return visibleModels.length ? [{ ...group, visibleModels }] : []
  })
})
const activeGroup = computed(() => visibleGroups.value.find(group => String(group.id) === activeGroupId.value) ?? visibleGroups.value[0] ?? null)
const renderedModelCount = computed(() => visibleGroups.value.reduce((count, group) => count + group.visibleModels.length, 0))
const hasMoreModels = computed(() => renderedModelCount.value < filteredModelCount.value)
type ModelStatusBucketOutcome = ModelStatusOutcome | 'degraded' | 'no-data'

function formatCount(value: number): string {
  return countFormatter.value.format(value)
}

function formatRate(metrics: ModelStatusMetrics): string {
  const value = metrics.success_rate
  return metrics.total === 0 || value == null ? '-' : `${rateFormatter.value.format(value)}%`
}

// The API metrics are the rolling 24-hour aggregate. Buckets are only the
// visual timeline and must not replace the card's overall success rate with
// the most recent (potentially very small) 30-minute sample.
function modelMetricsForDisplay(model: ModelStatusModel): ModelStatusMetrics {
  return model.metrics
}

function modelHealthForDisplay(model: ModelStatusModel): ModelStatusHealth {
  const metrics = modelMetricsForDisplay(model)
  if (metrics.total === 0) return 'no_data'
  if (!model.buckets?.length) return model.status
  const knownTotal = observedOutcomeTotal(metrics)
  if (knownTotal === 0) return 'unknown'
  // Match the backend health thresholds using the same bucket as the card metrics.
  if (knownTotal < 5 || metrics.success_rate === null) return 'insufficient_data'
  if (metrics.success_rate >= 99) return 'healthy'
  if (metrics.success_rate >= 80) return 'degraded'
  return 'unavailable'
}

function observedOutcomeTotal(metrics: ModelStatusModel['metrics']): number {
  return metrics.success + metrics.failure + metrics.empty
}

function hasObservedOutcomes(metrics: ModelStatusModel['metrics']): boolean {
  return observedOutcomeTotal(metrics) > 0
}

function healthLightStyle(metrics: ModelStatusModel['metrics']): Record<string, string> | undefined {
  const total = observedOutcomeTotal(metrics)
  if (total <= 0) return undefined
  const successRatio = Math.max(0, Math.min(1, metrics.success / total))
  return { '--health-hue': `${Math.round(successRatio * 120)}deg` }
}

function resultLabel(at: string, outcome: ModelStatusOutcome, statusCode?: number): string {
  const label = requestOutcomeLabel(outcome, statusCode)
  return `${label} · ${new Date(at).toLocaleString(locale.value, { hour12: false })}`
}

function modelBuckets(model: ModelStatusModel): ModelStatusBucket[] {
  return model.buckets ?? []
}

function bucketOutcome(bucket: ModelStatusBucket): ModelStatusBucketOutcome {
  if (bucket.total <= 0) return 'no-data'
  const knownTotal = bucket.success + bucket.failure + bucket.empty
  if (knownTotal <= 0) return 'unknown'
  if (bucket.success === 0 && bucket.failure === 0 && bucket.empty > 0) return 'empty'
  const successRate = bucket.success / knownTotal
  if (successRate > 0.8) return 'success'
  if (successRate >= 0.5) return 'degraded'
  return 'failure'
}

function bucketStatusLabel(bucket: ModelStatusBucket): string {
  const outcome = bucketOutcome(bucket)
  return outcome === 'no-data'
    ? t('modelStatus.noRequestsInBucket')
    : outcome === 'unknown'
      ? t('modelStatus.noRatedRequests')
      : outcome === 'degraded'
        ? t('modelStatus.bucketStatus.degraded')
        : t(`modelStatus.outcome.${outcome}`)
}

function bucketLabel(bucket: ModelStatusBucket): string {
  return `${formatRange(bucket.start_at, bucket.end_at)} · ${bucketStatusLabel(bucket)}`
}

function bucketKey(modelName: string, bucket: ModelStatusBucket): string {
  return `${modelName}:${bucket.start_at}`
}

function formatRange(start: string, end: string): string {
  return `${bucketStartFormatter.value.format(new Date(start))} - ${bucketEndFormatter.value.format(new Date(end))}`
}

function toggleBucket(event: MouseEvent, bucket: ModelStatusBucket, modelIdentity: string) {
  if (event.currentTarget === bucketAnchor) closeBucket()
  else void showBucket(event, bucket, modelIdentity)
}

async function showBucket(event: MouseEvent, bucket: ModelStatusBucket, modelIdentity: string) {
  const anchor = event.currentTarget as HTMLElement
  bucketAnchor = anchor
  bucketAnchorKey.value = bucketKey(modelIdentity, bucket)
  selectedBucket.value = bucket
  bucketPosition.value = { top: '0px', left: '0px', visibility: 'hidden' }
  await nextTick()
  if (anchor !== bucketAnchor || !bucketPopover.value) return

  const rect = anchor.getBoundingClientRect()
  const { width, height } = bucketPopover.value.getBoundingClientRect()
  const margin = 8
  const left = Math.max(margin, Math.min(rect.left + (rect.width - width) / 2, window.innerWidth - width - margin))
  const top = rect.bottom + height + margin <= window.innerHeight - margin
    ? rect.bottom + margin
    : Math.max(margin, rect.top - height - margin)
  bucketPosition.value = { top: `${top}px`, left: `${left}px`, visibility: 'visible' }
}

function closeBucket() {
  selectedBucket.value = null
  bucketAnchorKey.value = ''
  bucketAnchor = null
}

function handleBucketOutsideClick(event: PointerEvent) {
  if (bucketAnchor && event.target instanceof Node && !bucketAnchor.contains(event.target) && !bucketPopover.value?.contains(event.target)) {
    closeBucket()
  }
}

function handleBucketEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') closeBucket()
}

function requestOutcomeLabel(outcome: ModelStatusOutcome, statusCode?: number): string {
  if (outcome === 'unknown' && statusCode !== undefined && statusCode >= 400 && statusCode < 500) {
    return t('modelStatus.clientErrorExcluded')
  }
  return outcome === 'unknown' ? t('modelStatus.incompleteResult') : t(`modelStatus.outcome.${outcome}`)
}

function loadMoreModels() {
  if (loadingMoreModels.value || !hasMoreModels.value) return
  loadingMoreModels.value = true
  loadMoreTimer = setTimeout(() => {
    loadMoreTimer = undefined
    visibleModelLimit.value += 40
    loadingMoreModels.value = false
  }, 160)
}

function clearGroupFilter() {
  groupFilter.value = ''
}

async function loadReport() {
  if (request || disposed) return
  const controller = new AbortController()
  request = controller
  loading.value = true
  try {
    const data = await getModelStatus({ signal: controller.signal })
    if (disposed) return
    report.value = data
    loadFailed.value = false
    validateGroupFilter(data.groups)
  } catch {
    if (!controller.signal.aborted) loadFailed.value = true
  } finally {
    if (request === controller) request = null
    loading.value = false
    refreshCountdown.value = refreshIntervalSeconds
    now.value = Date.now()
  }
}

function refreshAll() {
  void loadReport()
  void loadCustomGroups()
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') refreshAll()
}

function updateMobileStatusChrome() {
  const scrollY = Math.max(0, window.scrollY || 0)
  const scrollDelta = scrollY - lastScrollY
  if (window.innerWidth > mobileBreakpoint) {
    mobileNavHidden.value = false
    showBackToTop.value = false
  } else {
    if (scrollY <= mobileHeaderHideThreshold) mobileNavHidden.value = false
    if (scrollDelta >= mobileHeaderHideDelta && scrollY > mobileHeaderHideThreshold) mobileNavHidden.value = true
    showBackToTop.value = scrollY > backToTopThreshold
  }
  document.documentElement.classList.toggle('model-status-mobile-header-hidden', mobileNavHidden.value)
  lastScrollY = scrollY
}

function handleWindowResize() {
  lastScrollY = window.scrollY || 0
  updateMobileStatusChrome()
  void nextTick().then(observeStatusGroups)
}

function scrollToTop() {
  const behavior = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth'
  window.scrollTo({ top: 0, behavior })
}

function observeStatusGroups() {
  groupContextObserver?.disconnect()
  groupContextObserver = null
  const headings = statusGroups.value?.querySelectorAll<HTMLElement>('[data-group-id]')
  if (!headings?.length) {
    activeGroupId.value = ''
    showGroupContext.value = false
    return
  }
  activeGroupId.value = headings[0].dataset.groupId ?? ''
  showGroupContext.value = false
  if (window.innerWidth > mobileBreakpoint || typeof IntersectionObserver === 'undefined') return
  groupContextObserver = new IntersectionObserver(entries => {
    if (!entries.length) return
    const passedHeadings = Array.from(headings)
      .filter(heading => heading.getBoundingClientRect().top <= mobileStatusToolbarHeight)
      .sort((left, right) => left.getBoundingClientRect().top - right.getBoundingClientRect().top)
    const current = passedHeadings[passedHeadings.length - 1] ?? headings[0]
    activeGroupId.value = current.dataset.groupId ?? activeGroupId.value
    showGroupContext.value = passedHeadings.length > 0
  }, { rootMargin: `-${mobileStatusToolbarHeight}px 0px -70% 0px`, threshold: [0, 1] })
  headings.forEach(heading => groupContextObserver?.observe(heading))
}

watch(groupFilter, value => {
  persistGroupFilter(value)
  visibleModelLimit.value = 40
})

watch(() => `${authStore.isAuthenticated}:${authStore.user?.id ?? ''}`, () => {
  void loadCustomGroups()
}, { flush: 'post' })

watch(visibleGroups, () => {
  closeBucket()
  void nextTick().then(observeStatusGroups)
}, { flush: 'post' })

watch(loadMoreSentinel, element => {
  loadMoreObserver?.disconnect()
  loadMoreObserver = null
  if (!element || typeof IntersectionObserver === 'undefined') return
  loadMoreObserver = new IntersectionObserver(entries => {
    if (entries.some(entry => entry.isIntersecting)) loadMoreModels()
  }, { rootMargin: '240px 0px' })
  loadMoreObserver.observe(element)
}, { flush: 'post' })

watch(modelSearch, value => {
  if (typeof window === 'undefined') return
  const url = new URL(window.location.href)
  const normalized = normalizeModelStatusSearch(value)
  if (normalized) url.searchParams.set('search', value.trim())
  else url.searchParams.delete('search')
  window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`)
})

onMounted(() => {
  void appStore.fetchPublicSettings().catch(() => {})
  refreshAll()
  refreshCountdownTimer = setInterval(() => {
    refreshCountdown.value = Math.max(0, refreshCountdown.value - 1)
  }, 1000)
  timer = setInterval(() => {
    now.value = Date.now()
    void loadReport()
  }, refreshIntervalSeconds * 1000)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  lastScrollY = window.scrollY || 0
  updateMobileStatusChrome()
  window.addEventListener('scroll', updateMobileStatusChrome, { passive: true })
  window.addEventListener('resize', handleWindowResize)
  window.addEventListener('resize', closeBucket)
  window.addEventListener('scroll', closeBucket, true)
  document.addEventListener('pointerdown', handleBucketOutsideClick)
  document.addEventListener('keydown', handleBucketEscape)
})

onBeforeUnmount(() => {
  disposed = true
  request?.abort()
  customGroupsRequest?.abort()
  loadMoreObserver?.disconnect()
  if (loadMoreTimer) clearTimeout(loadMoreTimer)
  clearInterval(timer)
  clearInterval(refreshCountdownTimer)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  groupContextObserver?.disconnect()
  window.removeEventListener('scroll', updateMobileStatusChrome)
  window.removeEventListener('resize', handleWindowResize)
  window.removeEventListener('resize', closeBucket)
  window.removeEventListener('scroll', closeBucket, true)
  document.removeEventListener('pointerdown', handleBucketOutsideClick)
  document.removeEventListener('keydown', handleBucketEscape)
  document.documentElement.classList.remove('model-status-mobile-header-hidden')
})
</script>

<style scoped>
.model-status-page { min-height: 100vh; min-width: 0; letter-spacing: 0; }
.status-guest { @apply bg-slate-50 text-slate-900 dark:bg-dark-950 dark:text-slate-100; }
.status-content { container: model-status / inline-size; max-width: 1280px; min-width: 0; margin: 0 auto; padding: 28px 24px; }
.status-content-embedded { max-width: none; margin: 0; padding: 0; }
.status-toolbar-shell { min-width: 0; }
.model-status-workspace { --status-surface: #fff; --status-surface-muted: #f8fafc; --status-border: #e2e8f0; }
.dark .model-status-workspace { --status-surface: #111827; --status-surface-muted: #0f172a; --status-border: #283548; }
.workspace-toolbar { position: relative; }
.status-page-title { margin: 0 0 16px; font-size: 21px; font-weight: 650; line-height: 1.3; }
.status-recovery-action { min-height: 44px; }
.status-recovery-action:focus-visible, .refresh-button:focus-visible { outline: 2px solid var(--brand-500); outline-offset: 3px; }
.status-toolbar-actions { display: inline-flex; flex-shrink: 0; align-items: center; gap: 12px; margin-left: auto; }
.model-count + .status-toolbar-actions { margin-left: 0; }
.status-home-link { @apply text-slate-600 hover:underline dark:text-slate-300; font-size: 12px; white-space: nowrap; }
.refresh-countdown { @apply text-slate-500 dark:text-dark-400; font-size: 12px; white-space: nowrap; font-variant-numeric: tabular-nums; }
.refresh-button { width: 44px; height: 44px; flex: 0 0 44px; border-color: var(--status-border); background: var(--status-surface); box-shadow: none; }
.recent-bar:focus-visible { outline: 2px solid var(--brand-500); outline-offset: 3px; }
.status-notice { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 12px 0; font-size: 13px; }
.status-notice-error { @apply text-red-700 dark:text-red-400; }
.status-notice-warning { @apply text-amber-700 dark:text-amber-400; }
.outcome-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.outcome-success { @apply bg-emerald-600 dark:bg-emerald-400; }
.outcome-failure { @apply bg-red-500 dark:bg-red-400; }
.outcome-empty { @apply bg-amber-500 dark:bg-amber-400; }
.status-filters { display: flex; align-items: center; gap: 10px; padding: 0 0 18px; }
.group-filter { width: 240px; min-width: 0; }
.model-search { display: flex; width: min(100%, 320px); min-height: 44px; align-items: center; gap: 8px; padding: 0 12px; color: var(--status-muted, #64748b); border: 1px solid var(--status-border); border-radius: 8px; background: var(--status-surface); }
.model-search:focus-within { outline: 2px solid color-mix(in srgb, var(--brand-500) 55%, transparent); outline-offset: 2px; }
.model-search input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; color: inherit; font-size: 13px; }
.model-search-clear { display: inline-flex; width: 28px; height: 28px; align-items: center; justify-content: center; border-radius: 6px; }
.model-search-clear:hover { background: var(--status-surface-muted); }
.group-filter :deep(.select-trigger) { min-height: 44px; }
.group-filter-mobile-icon { display: none; }
.group-context { display: none; }
.model-count { @apply text-slate-500 dark:text-dark-400; margin-left: auto; font-size: 12px; white-space: nowrap; }
.status-empty { @apply text-slate-500 dark:text-dark-400; display: flex; min-height: 200px; flex-direction: column; align-items: center; justify-content: center; gap: 12px; text-align: center; font-size: 14px; }
.workspace-empty-state { padding: 28px 16px; border: 1px dashed var(--status-border); border-radius: 14px; background: var(--status-surface-muted); }
.status-group { margin-bottom: 24px; }
.group-heading { display: flex; min-width: 0; align-items: center; margin: 0 0 8px; }
.group-title { min-width: 0; }
.group-heading h2 { @apply text-slate-700 dark:text-slate-200; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; font-weight: 650; line-height: 22px; }
.model-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1px; overflow: hidden; border: 1px solid var(--status-border); border-radius: 8px; background: var(--status-surface); }
.model-row { display: flex; min-width: 0; flex-direction: column; gap: 4px; padding: 10px 14px; border: 0; border-radius: 0; background: var(--status-surface); box-shadow: 0 0 0 1px var(--status-border); }
.load-more-models { display: flex; width: min(100%, 280px); min-height: 40px; margin: 4px auto 28px; align-items: center; justify-content: center; gap: 7px; }
.load-more-models.is-loading { cursor: wait; opacity: .78; }
.load-more-wrap { position: relative; min-height: 60px; }
.load-more-sentinel { position: absolute; right: 0; bottom: 0; left: 0; height: 1px; pointer-events: none; }
.model-identity, .model-meta, .recent-results { min-width: 0; }
.model-status-page .model-health.sr-only,
.model-status-page .health-light.sr-only,
.model-status-page .outcome-counts.sr-only {
  display: none !important;
  position: absolute !important;
  width: 1px !important;
  height: 1px !important;
  padding: 0 !important;
  margin: -1px !important;
  overflow: hidden !important;
  clip: rect(0, 0, 0, 0) !important;
  white-space: nowrap !important;
  border: 0 !important;
}
.model-row-header { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 18px; }
.model-identity-minimal { display: flex; min-width: 0; align-items: center; justify-content: flex-start; gap: 10px; }
.model-status-rate { @apply text-slate-500 dark:text-dark-300; min-width: 44px; text-align: right; font-size: 13px; font-variant-numeric: tabular-nums; }
.model-status-rate strong { @apply text-slate-500 dark:text-dark-300; font-weight: 500; }
.model-title { display: flex; min-height: 28px; gap: 10px; align-items: center; }
.model-logo { @apply rounded-lg bg-primary-50 dark:bg-primary-900/25; display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; flex: 0 0 36px; }
.model-logo :deep(.model-icon), .model-logo :deep(.model-icon-fallback) { flex-shrink: 0; }
.model-title h3 { font-size: 14px; font-weight: 600; line-height: 20px; overflow-wrap: anywhere; }
.model-meta { @apply text-slate-500 dark:text-dark-400; display: flex; min-width: 0; align-items: center; gap: 10px; margin-top: 6px; font-size: 12px; }
.outcome-counts { display: flex; min-width: 0; flex-wrap: wrap; justify-content: flex-end; gap: 4px 8px; margin-left: auto; font-size: 12px; }
.outcome-counts > span { display: inline-flex; align-items: center; gap: 4px; }
.model-health:not(.sr-only) { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 24px; margin-top: 8px; padding-left: 3px; }
.health-light { @apply bg-slate-400 text-slate-400 dark:bg-slate-500 dark:text-slate-500; width: 10px; height: 10px; flex: 0 0 10px; border-radius: 50%; box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 15%, transparent), 0 0 8px color-mix(in srgb, currentColor 20%, transparent); }
.dark .model-status-page .health-light[data-health="healthy"], .dark .model-status-page .health-light[data-health="degraded"], .dark .model-status-page .health-light[data-health="unavailable"] { box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 15%, transparent), 0 0 10px color-mix(in srgb, currentColor 34%, transparent); }
.health-light[data-health="healthy"] { @apply bg-emerald-500 text-emerald-500 dark:bg-emerald-400 dark:text-emerald-400; }
.health-light[data-health="degraded"] { @apply bg-amber-500 text-amber-500 dark:bg-amber-400 dark:text-amber-400; }
.health-light[data-health="unavailable"] { @apply bg-red-500 text-red-500 dark:bg-red-400 dark:text-red-400; }
.health-light-observed { background: hsl(var(--health-hue) 72% 48%); color: hsl(var(--health-hue) 72% 48%); }
.dark .model-status-page .health-light-observed { background: hsl(var(--health-hue) 78% 58%); color: hsl(var(--health-hue) 78% 58%); }
.health-badge { max-width: 100%; }
.health-badge-readable { overflow-wrap: anywhere; text-align: center; }
.recent-results { width: 100%; max-width: 100%; min-width: 0; }
.recent-heading { @apply text-slate-500 dark:text-dark-400; display: flex; justify-content: space-between; gap: 8px; font-size: 11px; line-height: 14px; }
.recent-heading-axis { width: 100%; }
.recent-heading-label { display: inline-flex; min-width: 0; align-items: center; gap: 5px; }
.recent-bars { position: relative; display: grid; grid-template-columns: repeat(48, minmax(0, 1fr)); width: 100%; gap: 2px; height: 16px; margin: 5px 0; overflow: visible; }
.recent-bars:not(.status-bucket-grid) { grid-template-columns: repeat(30, minmax(0, 1fr)); }
.status-bucket-grid { isolation: isolate; }
.recent-bar { position: relative; display: block; justify-self: center; min-width: 0; width: 6px; height: 16px; padding: 0; border: 0; border-radius: 0; cursor: pointer; touch-action: manipulation; }
.status-bucket-grid > .recent-bar { justify-self: stretch; width: 100%; }
.recent-placeholder { @apply bg-slate-300 dark:bg-slate-600; display: block; justify-self: center; min-width: 0; width: 6px; height: 16px; border-radius: 0; }
.recent-incomplete { @apply border border-amber-400 dark:border-amber-300; background: transparent; }
.recent-bar:focus-visible { outline: 2px solid var(--brand-500); outline-offset: 2px; }
.bucket-no-data { @apply bg-slate-300 text-slate-400 dark:bg-slate-600 dark:text-slate-500; }
.bucket-success { @apply bg-emerald-500 text-emerald-500 dark:bg-emerald-400 dark:text-emerald-400; }
.bucket-failure { @apply bg-red-500 text-red-500 dark:bg-red-400 dark:text-red-400; }
.bucket-degraded { @apply bg-amber-400 text-amber-400 dark:bg-amber-300 dark:text-amber-300; }
.bucket-empty { @apply bg-amber-400 text-amber-400 dark:bg-amber-300 dark:text-amber-300; }
.bucket-unknown { @apply bg-amber-400 text-amber-400 dark:bg-amber-300 dark:text-amber-300; }
.dark .model-status-page .bucket-success { box-shadow: 0 0 9px rgba(52, 211, 153, 0.3); }
.dark .model-status-page .bucket-failure { box-shadow: 0 0 9px rgba(248, 113, 113, 0.27); }
.dark .model-status-page .bucket-degraded { box-shadow: 0 0 9px rgba(251, 191, 36, 0.24); }
.dark .model-status-page .bucket-empty { box-shadow: 0 0 9px rgba(251, 191, 36, 0.24); }
.dark .model-status-page .bucket-unknown { box-shadow: 0 0 9px rgba(251, 191, 36, 0.18); }
.bucket-light:is(:hover, [aria-expanded="true"]) { background-color: color-mix(in srgb, currentColor 80%, white); }
.bucket-light.bucket-no-data:is(:hover, [aria-expanded="true"]) { @apply bg-slate-200 dark:bg-slate-500; }

.bucket-popover {
  @apply border border-slate-200 bg-white text-slate-900 shadow-lg dark:border-dark-600 dark:bg-dark-800 dark:text-white;
  position: fixed;
  z-index: 60;
  width: min(216px, calc(100vw - 16px));
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 12px;
}
.bucket-popover-time { @apply text-slate-500 dark:text-dark-300; margin: 0 0 6px; }
.bucket-popover-status { display: flex; align-items: center; gap: 6px; font-weight: 600; }
.bucket-popover-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.bucket-popover-counts { @apply border-slate-100 dark:border-dark-700; display: grid; grid-template-columns: repeat(2, 1fr); gap: 8px; border-top-width: 1px; margin: 8px 0 0; padding-top: 7px; }
.bucket-popover-counts > div { display: flex; justify-content: space-between; gap: 4px; }
.bucket-popover-counts dt { @apply text-slate-500 dark:text-dark-300; }
.bucket-popover-counts dd { margin: 0; font-weight: 600; font-variant-numeric: tabular-nums; }
.dark .model-status-page .model-title :deep(.model-icon path[fill="#000000"]),
.dark .model-status-page .model-title :deep(.model-icon path[fill="#16191E"]) { fill: #f1f5f9; }

@media (prefers-reduced-motion: reduce) {
  .model-row, .load-more-models { transition: none; }
  .load-more-models .animate-spin { animation: none; }
}

@container model-status (min-width: 1400px) {
  .model-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}

@container model-status (min-width: 1600px) {
  .model-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}

@container model-status (max-width: 1040px) {
  .model-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 767px) {
  .model-grid { grid-template-columns: minmax(0, 1fr); }
  .model-row { padding: 10px 12px; }
  .model-row-header { align-items: center; }
  .model-status-page .model-identity { flex-direction: row; align-items: center; gap: 10px; }
  .model-identity-minimal { width: 100%; }
  .recent-results { width: 100%; }
  .recent-bars { grid-template-columns: repeat(48, minmax(0, 1fr)); height: 16px; gap: 2px; margin: 4px 0; }
  .recent-bars:not(.status-bucket-grid) { grid-template-columns: repeat(30, minmax(0, 1fr)); }
  .recent-bar, .recent-placeholder { width: 5px; height: 16px; border-radius: 0; }
}

@container model-status (max-width: 640px) {
  .model-count { display: none; }
  .model-grid { grid-template-columns: minmax(0, 1fr); }
  .model-row { display: flex; gap: 8px; padding: 12px 14px; }
  .model-title { gap: 10px; }
  .recent-bars { grid-template-columns: repeat(48, minmax(0, 1fr)); height: 16px; gap: 2px; margin: 4px 0; }
  .recent-bars:not(.status-bucket-grid) { grid-template-columns: repeat(30, minmax(0, 1fr)); }
  .recent-bar, .recent-placeholder { width: 5px; height: 16px; border-radius: 0; }
  .model-health { margin-top: 6px; }
  .group-heading { padding: 7px 9px 6px; margin-bottom: 6px; }
  .group-title { width: 100%; }
}

@container model-status (max-width: 360px) {
  .recent-bars { gap: 2px; height: 16px; }
  .recent-bar, .recent-placeholder { height: 16px; }
}

@media (max-width: 640px) {
  .status-guest .status-content { padding: 12px; }
}

@media (max-width: 767px) {
  .status-toolbar-shell { position: sticky; top: 0; z-index: 20; background: var(--status-surface-muted); }
  .status-filters { display: grid; grid-template-columns: minmax(0, 1fr) 44px auto; gap: 8px; padding: 6px 0 8px; }
  .status-filters .model-search { width: 100%; min-width: 0; }
  .status-filters .group-filter { width: 44px; }
  .status-filters .group-filter :deep(.select-trigger) { width: 44px; min-height: 44px; justify-content: center; padding: 0; border-color: var(--status-border); border-radius: 8px; background: var(--status-surface); box-shadow: none; }
  .status-filters .group-filter :deep(.select-value) { flex: 0 0 auto; }
  .status-filters .group-filter :deep(.select-icon), .group-filter-selected-text { display: none; }
  .group-filter-mobile-icon { display: inline-flex; align-items: center; }
  .status-toolbar-actions { grid-area: 1 / 3; gap: 8px; }
  .refresh-countdown, .model-count { display: none; }
  .group-context { @apply text-slate-600 dark:text-slate-300; display: flex; grid-column: 1 / -1; min-width: 0; align-items: center; gap: 6px; font-size: 12px; }
  .group-context-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .model-status-page .model-identity {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }
  .model-status-page .model-title {
    grid-column: 1 / -1;
    min-height: 24px;
  }
  .model-status-page .model-meta {
    min-width: 0;
    margin-top: 0;
  }
  .model-status-page .model-health:not(.sr-only) {
    min-height: 24px;
    margin-top: 0;
    padding-left: 0;
    justify-content: flex-start;
    gap: 8px;
  }
  .model-status-page .health-light:not(.sr-only) {
    width: 12px;
    height: 12px;
    flex-basis: 12px;
    box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 16%, transparent), 0 0 10px color-mix(in srgb, currentColor 28%, transparent);
  }
  .model-status-page .health-badge {
    white-space: normal;
  }
  .back-to-top {
    position: fixed;
    right: max(16px, env(safe-area-inset-right));
    bottom: max(16px, env(safe-area-inset-bottom));
    z-index: 40;
    width: 44px;
    height: 44px;
    box-shadow: 0 10px 24px color-mix(in srgb, var(--brand-500) 28%, transparent);
  }
}
</style>
