<template>
  <component :is="authStore.isAuthenticated ? AppLayout : 'div'" class="model-status-page" :class="{ 'status-guest': !authStore.isAuthenticated, 'status-group-context-active': showGroupContext }">
    <PlazaNavBar v-if="!authStore.isAuthenticated" login-redirect="/model-status" />
    <component :is="authStore.isAuthenticated ? 'section' : 'main'" class="status-content" :class="{ 'status-content-embedded': authStore.isAuthenticated }" :aria-busy="loading">
      <div class="status-toolbar-shell">
        <header class="page-header status-header">
        <div class="min-w-0">
          <h1 class="page-title flex items-center gap-2">
            <span class="status-title-icon"><Icon name="chart" size="md" /></span>
            {{ t('modelStatus.title') }}
          </h1>
        </div>
        <div class="status-header-actions">
          <span class="refresh-countdown" data-testid="refresh-countdown" aria-live="polite" aria-atomic="true">
            {{ loading ? t('modelStatus.refreshing') : t('modelStatus.autoRefreshIn', { seconds: refreshCountdown }) }}
          </span>
          <button
            class="btn btn-secondary btn-icon refresh-button"
            type="button"
            data-testid="refresh"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="loadReport"
          >
            <Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" />
          </button>
        </div>
        </header>

        <template v-if="report">
          <div class="status-filters" :class="{ 'group-context-active': showGroupContext }">
            <Select v-model="groupFilter" class="group-filter" :options="groupOptions" :aria-label="t('modelStatus.group')" mobile-sheet>
              <template #selected>
                <span class="group-filter-selected-text">{{ groupOptions.find(option => option.value === groupFilter)?.label ?? t('modelStatus.allGroups') }}</span>
                <span class="group-filter-mobile-icon" aria-hidden="true"><Icon name="filter" size="sm" /></span>
              </template>
            </Select>
            <div v-if="activeGroup && showGroupContext" class="group-context" aria-live="polite">
              <span class="group-context-accent" aria-hidden="true" />
              <span class="group-context-name">{{ activeGroup.name }}</span>
            </div>
            <span class="model-count">{{ t('modelStatus.modelCount', { count: filteredModelCount }) }}</span>
          </div>
        </template>
      </div>

      <div v-if="loadFailed" role="alert" class="status-notice status-notice-error">
        <Icon name="exclamationTriangle" size="md" class="shrink-0" />
        <span>{{ t(report ? 'modelStatus.refreshFailed' : 'modelStatus.loadFailed') }}</span>
        <button v-if="!report" data-testid="retry" type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="loadReport">
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

        <div v-if="!report.groups.length || !filteredGroups.length" class="status-empty">
          <Icon name="search" size="lg" class="text-gray-400" />
          <span>{{ t(report.groups.length ? 'modelStatus.noMatches' : 'modelStatus.noModels') }}</span>
        </div>

        <div v-if="visibleGroups.length" ref="statusGroups" class="status-groups">
          <section v-for="(group, groupIndex) in visibleGroups" :key="group.id" class="status-group" :aria-labelledby="`status-group-${group.id}`">
            <header class="group-heading" :data-group-id="group.id">
              <div class="group-title">
                <span class="group-accent" aria-hidden="true" />
                <h2 :id="`status-group-${group.id}`">{{ group.name }}</h2>
              </div>
            </header>
            <div class="model-grid">
          <article v-for="(model, modelIndex) in group.visibleModels" :key="`${model.platform}:${model.name}`" class="card model-row" data-testid="model-row">
            <div class="model-identity">
              <div class="model-title">
                <span class="model-logo"><ModelIcon :model="model.name" size="20px" /></span>
                <h3>{{ model.name }}</h3>
              </div>
              <div class="model-meta">
                <span class="model-rate">
                  <span>{{ t('modelStatus.successRate') }}</span>
                  <strong>{{ formatRate(model.metrics.success_rate) }}</strong>
                </span>
                <div class="outcome-counts" :aria-label="t('modelStatus.outcomeCounts')">
                  <span v-for="outcome in outcomes" :key="outcome" :title="t(`modelStatus.outcome.${outcome}`)">
                    <i class="outcome-dot" :class="`outcome-${outcome}`" aria-hidden="true" />
                    <span class="sr-only">{{ t(`modelStatus.outcome.${outcome}`) }}</span>{{ formatCount(model.metrics[outcome]) }}
                  </span>
                </div>
              </div>
              <div class="model-health">
                <span
                  class="health-light"
                  :class="{ 'health-light-observed': hasObservedOutcomes(model.metrics) }"
                  :data-health="model.status"
                  :style="healthLightStyle(model.metrics)"
                  aria-hidden="true"
                />
                <span class="badge health-badge" :class="healthBadgeClasses[model.status]">{{ t(`modelStatus.health.${model.status}`) }}</span>
              </div>
            </div>

            <div class="recent-results">
              <template v-if="model.buckets">
                <div class="recent-heading">
                  <span class="recent-heading-label">
                    <span>{{ t('modelStatus.fifteenMinuteBuckets') }}</span>
                  </span>
                  <span>{{ modelBuckets(model).length }}/{{ bucketCount }}</span>
                </div>
              <div class="recent-bars" :class="{ 'bucket-hint-active': showBucketHint && groupIndex === 0 && modelIndex === 0 }" :aria-label="t('modelStatus.fifteenMinuteBuckets')">
                <button
                  v-for="(bucket, index) in modelBuckets(model)"
                  :key="`${bucket.start_at}:${index}`"
                  type="button"
                  class="recent-bar bucket-light"
                  data-testid="status-bucket"
                  :class="[`bucket-${bucketOutcome(bucket)}`, { 'bucket-pressed': pressedBucketKey === bucketKey(`${group.id}:${model.platform}:${model.name}`, bucket) }]"
                  :data-outcome="bucketOutcome(bucket) === 'unknown' ? undefined : bucketOutcome(bucket)"
                  :title="bucketLabel(bucket)"
                  :aria-label="bucketLabel(bucket)"
                  @click="handleBucketClick(model.name, bucket, `${group.id}:${model.platform}:${model.name}`)"
                />
              </div>
              <div class="recent-heading"><span>{{ t('modelStatus.older') }}</span><span>{{ t('modelStatus.newer') }}</span></div>
              </template>
              <template v-else>
                <div class="recent-heading"><span>{{ t('modelStatus.recentRequests', { count: 30 }) }}</span><span>{{ model.recent?.length ?? 0 }}/30</span></div>
                <div class="recent-bars" :aria-label="t('modelStatus.recentRequests', { count: 30 })">
                  <span v-for="index in Math.max(0, 30 - (model.recent?.length ?? 0))" :key="`placeholder-${index}`" class="recent-placeholder" aria-hidden="true" />
                  <span v-for="(result, index) in model.recent ?? []" :key="`${result.at}:${index}`" class="recent-bar" :class="result.outcome === 'unknown' ? 'recent-incomplete' : `outcome-${result.outcome}`" :data-outcome="result.outcome === 'unknown' ? undefined : result.outcome" :title="resultLabel(result.at, result.outcome)" />
                </div>
                <div class="recent-heading"><span>{{ t('modelStatus.older') }}</span><span>{{ t('modelStatus.newer') }}</span></div>
              </template>
            </div>

          </article>
            </div>
          </section>
        </div>
        <div v-if="hasMoreModels" class="load-more-wrap">
          <button class="btn btn-secondary load-more-models" type="button" @click="loadMoreModels">
            {{ t('modelStatus.loadMoreModels') }}
          </button>
          <span ref="loadMoreSentinel" class="load-more-sentinel" aria-hidden="true" />
        </div>
      </template>
    </component>
    <button
      v-if="showBackToTop"
      class="btn btn-primary btn-icon back-to-top"
      type="button"
      data-testid="back-to-top"
      :title="t('modelStatus.backToTop')"
      :aria-label="t('modelStatus.backToTop')"
      @click="scrollToTop"
    >
      <Icon name="arrowUp" size="sm" aria-hidden="true" />
    </button>
    <BaseDialog
      :show="Boolean(selectedBucket)"
      :title="selectedModelName ? `${selectedModelName} · ${t('modelStatus.bucketDetails')}` : t('modelStatus.bucketDetails')"
      width="wide"
      :close-on-click-outside="true"
      @close="closeBucket"
    >
      <template v-if="selectedBucket">
        <div class="bucket-detail">
          <p class="bucket-detail-range">{{ formatRange(selectedBucket.start_at, selectedBucket.end_at) }}</p>
          <div class="bucket-detail-stats">
            <div><span>{{ t('modelStatus.requestTotal') }}</span><strong>{{ formatCount(selectedBucket.total) }}</strong></div>
            <div><span class="stat-success">{{ t('modelStatus.outcome.success') }}</span><strong>{{ formatCount(selectedBucket.success) }}</strong></div>
            <div><span class="stat-failure">{{ t('modelStatus.outcome.failure') }}</span><strong>{{ formatCount(selectedBucket.failure) }}</strong></div>
            <div><span class="stat-empty">{{ t('modelStatus.outcome.empty') }}</span><strong>{{ formatCount(selectedBucket.empty) }}</strong></div>
          </div>
          <div class="bucket-detail-list">
            <div class="bucket-detail-list-header"><span>{{ t('modelStatus.requestDetails') }}</span><span v-if="selectedBucket.total > selectedBucket.requests.length">{{ t('modelStatus.showingLatest', { count: selectedBucket.requests.length, total: selectedBucket.total }) }}</span></div>
            <div v-if="!selectedBucket.requests.length" class="bucket-detail-empty">{{ t('modelStatus.noRequestsInBucket') }}</div>
            <div v-for="(requestItem, index) in selectedBucket.requests" :key="`${requestItem.at}:${index}`" class="bucket-request-row">
              <div class="bucket-request-info">
                <span>{{ new Date(requestItem.at).toLocaleString(locale, { hour12: false }) }}</span>
                <span v-if="requestItem.status_code" class="bucket-request-status">{{ t('modelStatus.statusCode', { code: requestItem.status_code }) }}</span>
              </div>
              <span class="badge" :class="requestOutcomeClass(requestItem.outcome)">{{ requestOutcomeLabel(requestItem.outcome) }}</span>
            </div>
          </div>
        </div>
      </template>
    </BaseDialog>
  </component>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getModelStatus, type ModelStatusBucket, type ModelStatusHealth, type ModelStatusModel, type ModelStatusOutcome, type ModelStatusResponse } from '@/api/modelStatus'
import AppLayout from '@/components/layout/AppLayout.vue'
import PlazaNavBar from '@/components/modelPlaza/PlazaNavBar.vue'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const report = ref<ModelStatusResponse | null>(null)
const loading = ref(true)
const loadFailed = ref(false)
const groupFilterStorageKey = 'model-status-group-filter'
const groupFilter = ref(readStoredGroupFilter())
const visibleModelLimit = ref(40)
const loadMoreSentinel = ref<HTMLElement | null>(null)
const now = ref(Date.now())
const mobileBreakpoint = 767
const mobileHeaderHideThreshold = 0
const mobileHeaderHideDelta = 4
const backToTopThreshold = 320
const mobileStatusToolbarHeight = 86
const refreshIntervalSeconds = 30
const refreshCountdown = ref(refreshIntervalSeconds)
const bucketHintStorageKey = 'model-status-bucket-hint-seen'
const outcomes = ['success', 'failure', 'empty'] as const
const mobileNavHidden = ref(false)
const showBackToTop = ref(false)
const showBucketHint = ref(false)
const bucketHintPlayed = ref(false)
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
let timer: ReturnType<typeof setInterval> | undefined
let refreshCountdownTimer: ReturnType<typeof setInterval> | undefined
let loadMoreObserver: IntersectionObserver | null = null
let groupContextObserver: IntersectionObserver | null = null
let disposed = false
let lastScrollY = 0
let bucketHintTimer: ReturnType<typeof setTimeout> | undefined

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

const stale = computed(() => !!report.value && !report.value.snapshot_at && now.value - Date.parse(report.value.generated_at) > 90000)
const bucketCount = computed(() => report.value?.bucket_count ?? 20)
const selectedBucket = ref<ModelStatusBucket | null>(null)
const selectedModelName = ref('')
const pressedBucketKey = ref('')
let pressedBucketTimer: ReturnType<typeof setTimeout> | undefined
const groupOptions = computed(() => [
  { value: '', label: t('modelStatus.allGroups'), platform: 'composite' },
  ...(report.value?.groups ?? []).map(group => ({ value: String(group.id), label: group.name, platform: group.platform })),
])
const filteredGroups = computed(() => {
  return (report.value?.groups ?? [])
    .filter(group => !groupFilter.value || String(group.id) === groupFilter.value)
    .filter(group => group.models.length)
})
const filteredModelCount = computed(() => filteredGroups.value.reduce((count, group) => count + group.models.length, 0))
const visibleGroups = computed(() => {
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
type ModelStatusBucketOutcome = ModelStatusOutcome | 'degraded'

function formatCount(value: number): string {
  return new Intl.NumberFormat(locale.value).format(value)
}

function formatRate(value: number | null): string {
  return value === null ? '-' : `${new Intl.NumberFormat(locale.value, { maximumFractionDigits: 1 }).format(value)}%`
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

function resultLabel(at: string, outcome: ModelStatusOutcome): string {
  const label = outcome === 'unknown' ? t('modelStatus.incompleteResult') : t(`modelStatus.outcome.${outcome}`)
  return `${label} · ${new Date(at).toLocaleString(locale.value, { hour12: false })}`
}

function modelBuckets(model: ModelStatusModel): ModelStatusBucket[] {
  return model.buckets ?? []
}

function bucketOutcome(bucket: ModelStatusBucket): ModelStatusBucketOutcome {
  if (bucket.total <= 0) return 'unknown'
  const knownTotal = bucket.success + bucket.failure + bucket.empty
  if (knownTotal <= 0) return 'unknown'
  if (bucket.failure > 0) {
    return bucket.failure / knownTotal >= 0.5 ? 'failure' : 'degraded'
  }
  if (bucket.empty > 0) return 'empty'
  if (bucket.success > 0) return 'success'
  return 'unknown'
}

function bucketLabel(bucket: ModelStatusBucket): string {
  const outcome = bucketOutcome(bucket)
  const status = outcome === 'unknown'
    ? t(bucket.total > 0 ? 'modelStatus.incompleteResult' : 'modelStatus.noRequestsInBucket')
    : outcome === 'degraded'
      ? t('modelStatus.bucketStatus.degraded')
      : t(`modelStatus.outcome.${outcome}`)
  return `${formatRange(bucket.start_at, bucket.end_at)} · ${status} · ${formatCount(bucket.total)}`
}

function bucketKey(modelName: string, bucket: ModelStatusBucket): string {
  return `${modelName}:${bucket.start_at}`
}

function formatRange(start: string, end: string): string {
  const options: Intl.DateTimeFormatOptions = { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }
  return `${new Date(start).toLocaleString(locale.value, options)} - ${new Date(end).toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit', hour12: false })}`
}

function openBucket(modelName: string, bucket: ModelStatusBucket) {
  selectedModelName.value = modelName
  selectedBucket.value = bucket
}

function handleBucketClick(modelName: string, bucket: ModelStatusBucket, modelIdentity = modelName) {
  pressedBucketKey.value = bucketKey(modelIdentity, bucket)
  if (pressedBucketTimer) clearTimeout(pressedBucketTimer)
  pressedBucketTimer = setTimeout(() => {
    pressedBucketKey.value = ''
    pressedBucketTimer = undefined
  }, 420)
  openBucket(modelName, bucket)
}

function closeBucket() {
  selectedBucket.value = null
  selectedModelName.value = ''
}

function requestOutcomeClass(outcome: ModelStatusOutcome): string {
  return outcome === 'success' ? 'badge-success' : outcome === 'failure' ? 'badge-danger' : outcome === 'empty' ? 'badge-warning' : 'badge-gray'
}

function requestOutcomeLabel(outcome: ModelStatusOutcome): string {
  return outcome === 'unknown' ? t('modelStatus.incompleteResult') : t(`modelStatus.outcome.${outcome}`)
}

function loadMoreModels() {
  visibleModelLimit.value += 40
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
    triggerBucketHint(data)
    loadFailed.value = false
    if (groupFilter.value && !data.groups.some(group => String(group.id) === groupFilter.value)) groupFilter.value = ''
  } catch {
    if (!controller.signal.aborted) loadFailed.value = true
  } finally {
    if (request === controller) request = null
    loading.value = false
    refreshCountdown.value = refreshIntervalSeconds
    now.value = Date.now()
  }
}

function handleVisibilityChange() {
  if (document.visibilityState === 'visible') void loadReport()
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

function triggerBucketHint(data: ModelStatusResponse) {
  if (bucketHintPlayed.value || showBucketHint.value || !data.groups.some(group => group.models.length)) return
  bucketHintPlayed.value = true
  try {
    if (sessionStorage.getItem(bucketHintStorageKey)) return
    sessionStorage.setItem(bucketHintStorageKey, '1')
  } catch {
    // Storage may be unavailable in private browsing; the hint can still play once.
  }
  showBucketHint.value = true
  bucketHintTimer = setTimeout(() => {
    showBucketHint.value = false
    bucketHintTimer = undefined
  }, 1400)
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

watch(visibleGroups, () => {
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

onMounted(() => {
  void appStore.fetchPublicSettings().catch(() => {})
  void loadReport()
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
})

onBeforeUnmount(() => {
  disposed = true
  request?.abort()
  loadMoreObserver?.disconnect()
  if (pressedBucketTimer) clearTimeout(pressedBucketTimer)
  if (bucketHintTimer) clearTimeout(bucketHintTimer)
  clearInterval(timer)
  clearInterval(refreshCountdownTimer)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  groupContextObserver?.disconnect()
  window.removeEventListener('scroll', updateMobileStatusChrome)
  window.removeEventListener('resize', handleWindowResize)
  document.documentElement.classList.remove('model-status-mobile-header-hidden')
})
</script>

<style scoped>
.model-status-page { min-height: 100vh; min-width: 0; letter-spacing: 0; }
.status-guest { @apply bg-slate-50 text-slate-900 dark:bg-dark-950 dark:text-slate-100; }
.status-content { container: model-status / inline-size; max-width: 1280px; min-width: 0; margin: 0 auto; padding: 28px 24px; }
.status-content-embedded { max-width: none; margin: 0; padding: 0; }
.status-toolbar-shell { min-width: 0; }
.status-header { @apply rounded-2xl border border-primary-100/70 bg-white/80 px-4 py-3 shadow-sm dark:border-primary-500/15 dark:bg-dark-900/60; display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.dark .model-status-page .status-header { border-color: rgba(96, 165, 250, 0.22); background: linear-gradient(135deg, rgba(30, 64, 175, 0.18), rgba(8, 145, 178, 0.08)), rgba(2, 6, 23, 0.78); box-shadow: 0 10px 28px rgba(0, 0, 0, 0.22), 0 1px 0 rgba(255, 255, 255, 0.04) inset; }
.dark .model-status-page .status-header .page-title { color: #e0f2fe; text-shadow: 0 0 18px rgba(34, 211, 238, 0.14); }
.status-title-icon { @apply rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400; display: inline-flex; align-items: center; justify-content: center; width: 32px; height: 32px; flex: 0 0 32px; }
.status-header-actions { display: inline-flex; min-width: 0; align-items: center; gap: 10px; }
.refresh-countdown { @apply text-slate-500 dark:text-dark-400; min-width: 94px; font-size: 12px; text-align: right; white-space: nowrap; font-variant-numeric: tabular-nums; }
.refresh-button { width: 44px; height: 44px; flex: 0 0 44px; }
.dark .model-status-page .refresh-button { @apply bg-dark-800 text-slate-200 shadow-none; border-color: rgba(96, 165, 250, 0.2); }
.recent-bar:focus-visible { outline: 2px solid var(--brand-500); outline-offset: 3px; }
.status-notice { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 12px 0; font-size: 13px; }
.status-notice-error { @apply text-red-700 dark:text-red-400; }
.status-notice-warning { @apply text-amber-700 dark:text-amber-400; }
.outcome-dot { display: inline-block; width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.outcome-success { @apply bg-emerald-600 dark:bg-emerald-400; }
.outcome-failure { @apply bg-red-500 dark:bg-red-400; }
.outcome-empty { @apply bg-amber-500 dark:bg-amber-400; }
.status-filters { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; padding: 0 0 24px; }
.group-filter { width: 240px; min-width: 0; }
.group-filter :deep(.select-trigger) { min-height: 44px; }
.group-filter-mobile-icon { display: none; }
.group-context { display: none; }
.model-count { @apply text-slate-500 dark:text-dark-400; margin-left: auto; font-size: 12px; white-space: nowrap; }
.status-empty { @apply text-slate-500 dark:text-dark-400; display: flex; min-height: 200px; flex-direction: column; align-items: center; justify-content: center; gap: 12px; text-align: center; font-size: 14px; }
.status-group { margin-bottom: 32px; }
.group-heading { @apply rounded-xl border border-primary-100/70 bg-primary-50/45 dark:border-primary-500/15 dark:bg-primary-900/10; display: flex; min-width: 0; align-items: center; padding: 11px 12px 10px; margin: 0 0 16px; }
.group-title { display: flex; min-width: 0; flex: 1 1 auto; align-items: flex-start; gap: 9px; }
.group-accent { @apply bg-primary-500 dark:bg-primary-400; display: block; width: 3px; min-width: 3px; min-height: 24px; border-radius: 999px; margin-top: 1px; }
.group-heading h2 { @apply text-slate-900 dark:text-slate-100; min-width: 0; flex: 1 1 auto; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 16px; font-weight: 650; line-height: 24px; }
.model-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.model-row { @apply rounded-2xl; contain-intrinsic-size: 380px; content-visibility: auto; display: flex; min-width: 0; flex-direction: column; gap: 18px; padding: 20px; }
.dark .model-status-page .model-row { border-color: rgba(96, 165, 250, 0.2); background: linear-gradient(180deg, rgba(15, 23, 42, 0.88), rgba(2, 6, 23, 0.94)); box-shadow: 0 14px 34px rgba(0, 0, 0, 0.26), 0 1px 0 rgba(255, 255, 255, 0.035) inset; }
.load-more-models { display: flex; width: min(100%, 280px); margin: 4px auto 28px; justify-content: center; }
.load-more-wrap { position: relative; min-height: 60px; }
.load-more-sentinel { position: absolute; right: 0; bottom: 0; left: 0; height: 1px; pointer-events: none; }
.model-identity, .model-meta, .recent-results { min-width: 0; }
.model-title { display: flex; min-height: 44px; gap: 12px; align-items: center; }
.model-logo { @apply rounded-lg bg-primary-50 dark:bg-primary-900/25; display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; flex: 0 0 36px; }
.model-logo :deep(.model-icon), .model-logo :deep(.model-icon-fallback) { flex-shrink: 0; }
.model-title h3 { font-size: 15px; font-weight: 600; line-height: 22px; overflow-wrap: anywhere; }
.model-meta { @apply text-slate-500 dark:text-dark-400; display: flex; min-width: 0; align-items: center; gap: 10px; margin-top: 6px; font-size: 12px; }
.outcome-counts { display: flex; min-width: 0; flex-wrap: wrap; justify-content: flex-end; gap: 4px 8px; margin-left: auto; font-size: 12px; }
.outcome-counts > span { display: inline-flex; align-items: center; gap: 4px; }
.model-health { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 24px; margin-top: 8px; padding-left: 3px; }
.health-light { @apply bg-slate-400 text-slate-400 dark:bg-slate-500 dark:text-slate-500; width: 10px; height: 10px; flex: 0 0 10px; border-radius: 50%; box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 15%, transparent), 0 0 8px color-mix(in srgb, currentColor 20%, transparent); }
.dark .model-status-page .health-light[data-health="healthy"], .dark .model-status-page .health-light[data-health="degraded"], .dark .model-status-page .health-light[data-health="unavailable"] { box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 15%, transparent), 0 0 10px color-mix(in srgb, currentColor 34%, transparent); }
.health-light[data-health="healthy"] { @apply bg-emerald-500 text-emerald-500 dark:bg-emerald-400 dark:text-emerald-400; }
.health-light[data-health="degraded"] { @apply bg-amber-500 text-amber-500 dark:bg-amber-400 dark:text-amber-400; }
.health-light[data-health="unavailable"] { @apply bg-red-500 text-red-500 dark:bg-red-400 dark:text-red-400; }
.health-light-observed { background: hsl(var(--health-hue) 72% 48%); color: hsl(var(--health-hue) 72% 48%); }
.dark .model-status-page .health-light-observed { background: hsl(var(--health-hue) 78% 58%); color: hsl(var(--health-hue) 78% 58%); }
.health-badge { max-width: 100%; }
.model-rate { display: inline-flex; min-width: 0; align-items: baseline; gap: 4px; white-space: nowrap; font-variant-numeric: tabular-nums; }
.model-rate strong { @apply text-slate-950 dark:text-white; font-weight: 600; }
.recent-results { margin-top: auto; }
.recent-heading { @apply text-slate-500 dark:text-dark-400; display: flex; justify-content: space-between; gap: 8px; font-size: 11px; line-height: 16px; }
.recent-heading-label { display: inline-flex; min-width: 0; align-items: center; gap: 5px; }
.recent-bars { position: relative; display: grid; grid-template-columns: repeat(20, minmax(0, 1fr)); gap: 4px; height: 22px; margin: 8px 0; overflow: hidden; }
.bucket-hint-active::after { position: absolute; top: -4px; bottom: -4px; left: -32%; width: 32%; background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--brand-500) 48%, transparent), transparent); content: ''; pointer-events: none; transform: translateX(0); animation: bucket-hint-sweep 1.2s cubic-bezier(.2, .7, .25, 1) both; }
.recent-bar { position: relative; display: block; min-width: 0; width: 100%; height: 22px; padding: 0; border: 1px solid transparent; border-radius: 999px; cursor: pointer; touch-action: manipulation; transition: transform .18s ease, opacity .18s ease, box-shadow .18s ease, border-color .18s ease; }
.recent-bar::after { position: absolute; inset: -4px; border: 1px solid currentColor; border-radius: inherit; content: ''; opacity: 0; pointer-events: none; transform: scale(.78); transition: opacity .18s ease, transform .18s ease; }
.recent-placeholder { @apply bg-slate-200 dark:bg-dark-700; display: block; min-width: 0; height: 22px; border-radius: 999px; }
.recent-incomplete { @apply border border-slate-400 dark:border-dark-500; background: transparent; }
.recent-bar:hover { opacity: .96; transform: translateY(-2px); box-shadow: 0 4px 10px color-mix(in srgb, currentColor 28%, transparent), inset 0 1px 0 rgb(255 255 255 / 32%); }
.recent-bar:hover::after, .recent-bar:focus-visible::after { opacity: .62; transform: scale(1); }
.recent-bar:active { transform: scale(.9); box-shadow: 0 1px 3px color-mix(in srgb, currentColor 30%, transparent); }
.bucket-pressed { animation: bucket-pop .42s ease both; }
.recent-bar:focus-visible { outline: 2px solid var(--brand-500); outline-offset: 2px; }
.bucket-success { @apply bg-emerald-500 text-emerald-500 dark:bg-emerald-400 dark:text-emerald-400; }
.bucket-failure { @apply bg-red-500 text-red-500 dark:bg-red-400 dark:text-red-400; }
.bucket-degraded { @apply bg-orange-500 text-orange-500 dark:bg-orange-400 dark:text-orange-400; }
.bucket-empty { @apply bg-amber-400 text-amber-400 dark:bg-amber-300 dark:text-amber-300; }
.bucket-unknown { @apply bg-slate-200 text-slate-400 dark:bg-dark-700 dark:text-slate-500; }
.dark .model-status-page .bucket-success { box-shadow: 0 0 9px rgba(52, 211, 153, 0.3); }
.dark .model-status-page .bucket-failure { box-shadow: 0 0 9px rgba(248, 113, 113, 0.27); }
.dark .model-status-page .bucket-degraded { box-shadow: 0 0 9px rgba(251, 146, 60, 0.3); }
.dark .model-status-page .bucket-empty { box-shadow: 0 0 9px rgba(251, 191, 36, 0.24); }
.dark .model-status-page .bucket-unknown { background: #1e293b; box-shadow: none; }

@keyframes bucket-pop {
  0% { transform: scale(1); }
  38% { transform: scale(.86); }
  72% { transform: scale(1.06); }
  100% { transform: scale(1); }
}

@keyframes bucket-hint-sweep {
  from { transform: translateX(0); }
  to { transform: translateX(420%); }
}
.bucket-detail { display: flex; flex-direction: column; gap: 18px; }
.bucket-detail-range { @apply text-slate-500 dark:text-dark-300; margin: 0; font-size: 13px; }
.bucket-detail-stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.bucket-detail-stats > div { @apply border border-slate-200 bg-slate-50 dark:border-dark-700 dark:bg-dark-800/60; display: flex; min-width: 0; flex-direction: column; gap: 4px; border-radius: 10px; padding: 12px; }
.dark .model-status-page .bucket-detail-stats > div { border-color: rgba(96, 165, 250, 0.16); background: rgba(30, 41, 59, 0.7); }
.bucket-detail-stats span { @apply text-slate-500 dark:text-dark-300; font-size: 12px; }
.bucket-detail-stats strong { @apply text-slate-900 dark:text-white; font-size: 18px; font-weight: 600; }
.stat-success { @apply text-emerald-600 dark:text-emerald-400 !important; }
.stat-failure { @apply text-red-600 dark:text-red-400 !important; }
.stat-empty { @apply text-amber-600 dark:text-amber-400 !important; }
.bucket-detail-list { @apply border-t border-slate-200 dark:border-dark-700; max-height: 360px; overflow: auto; padding-top: 14px; }
.bucket-detail-list-header { @apply text-slate-500 dark:text-dark-300; display: flex; justify-content: space-between; gap: 12px; margin-bottom: 8px; font-size: 12px; }
.bucket-detail-empty { @apply text-slate-500 dark:text-dark-400; padding: 24px 0; text-align: center; font-size: 13px; }
.bucket-request-row { @apply border-b border-slate-100 dark:border-dark-700/70; display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 12px; padding: 9px 0; font-size: 12px; }
.bucket-request-info { display: flex; min-width: 0; flex: 1 1 auto; flex-wrap: wrap; align-items: center; gap: 4px 10px; }
.bucket-request-status { @apply text-slate-400 dark:text-dark-400; font-variant-numeric: tabular-nums; white-space: nowrap; }
.dark .model-status-page .model-title :deep(.model-icon path[fill="#000000"]),
.dark .model-status-page .model-title :deep(.model-icon path[fill="#16191E"]) { fill: #f1f5f9; }

@media (prefers-reduced-motion: reduce) {
  .model-row, .recent-bar { transition: none; }
  .recent-bar::after, .bucket-pressed, .bucket-hint-active::after { animation: none; transition: none; }
}

@container model-status (min-width: 1400px) {
  .model-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}

@container model-status (min-width: 1600px) {
  .model-grid { grid-template-columns: repeat(5, minmax(0, 1fr)); }
}

@container model-status (max-width: 1040px) {
  .model-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@container model-status (max-width: 640px) {
  .group-filter { width: 100%; max-width: none; flex: auto; }
  .model-count { display: none; }
  .model-grid { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .model-row { gap: 16px; padding: 16px; }
  .model-title { gap: 10px; }
  .model-logo { width: 34px; height: 34px; flex-basis: 34px; }
  .model-health { margin-top: 6px; }
  .group-heading { padding: 10px 11px 9px; margin-bottom: 14px; }
  .group-title { width: 100%; }
}

@container model-status (max-width: 360px) {
  .recent-bars { gap: 2px; height: 24px; }
  .recent-bar, .recent-placeholder { height: 24px; }
}

@media (max-width: 560px) {
  .bucket-detail-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 640px) {
  .status-guest .status-content { padding: 16px; }
  .status-header { margin-bottom: 16px; padding: 12px; }
}

@media (max-width: 767px) {
  .status-toolbar-shell {
    --status-toolbar-bleed: var(--app-content-padding-x, 24px);
    position: sticky;
    top: 0;
    z-index: 20;
    width: auto;
    margin-right: calc(-1 * var(--status-toolbar-bleed));
    margin-left: calc(-1 * var(--status-toolbar-bleed));
    padding: 0 var(--status-toolbar-bleed);
    background: linear-gradient(to bottom, #f8fafc 0%, #f8fafc 88%, rgb(248 250 252 / 0%) 100%);
    backdrop-filter: blur(14px);
  }
  .status-guest .status-toolbar-shell { --status-toolbar-bleed: 24px; }
  .status-header {
    width: auto;
    margin-right: 0;
    margin-left: 0;
    padding-right: 12px;
    padding-left: 12px;
    margin-bottom: 0;
    max-height: 6rem;
    overflow: hidden;
    border-right: 0;
    border-left: 0;
    border-radius: 0;
    transition: max-height 180ms ease, margin 180ms ease, padding 180ms ease, border-color 180ms ease, opacity 140ms ease, transform 180ms ease, visibility 180ms ease;
  }
  :global(html.model-status-mobile-header-hidden .model-status-page .status-header) {
    max-height: 0;
    min-height: 0;
    margin-top: 0;
    margin-bottom: 0;
    padding-top: 0;
    padding-bottom: 0;
    border-top-color: transparent;
    border-bottom-color: transparent;
    opacity: 0;
    pointer-events: none;
    transform: translateY(-12px);
    visibility: hidden;
  }
  .status-filters {
    max-height: 8rem;
    position: static;
    z-index: auto;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 8px;
    width: 100%;
    margin: 0 0 14px;
    padding: 6px 0 10px;
    border: 0;
    border-radius: 0;
    background: linear-gradient(to bottom, #f8fafc 0%, #f8fafc 78%, rgb(248 250 252 / 0%) 100%);
    backdrop-filter: blur(14px);
    box-shadow: none;
    transition: max-height 180ms ease, margin 180ms ease, padding 180ms ease, border-color 180ms ease, opacity 140ms ease, transform 180ms ease, visibility 180ms ease;
  }
  .status-filters .group-filter { width: 100%; max-width: none; }
  .status-filters.group-context-active {
    display: flex;
    align-items: center;
    flex-wrap: nowrap;
    gap: 8px;
    padding-top: 5px;
    padding-bottom: 7px;
    border-bottom: 1px solid rgba(203, 213, 225, 0.6);
    background: rgb(248 250 252 / 92%);
  }
  .status-filters.group-context-active .group-filter {
    width: 40px;
    flex: 0 0 40px;
  }
  .status-filters.group-context-active .group-filter :deep(.select-trigger) {
    width: 40px;
    height: 36px;
    min-height: 36px;
    justify-content: center;
    padding: 0;
    border-color: rgba(148, 163, 184, 0.38);
    border-radius: 10px;
    background: rgb(255 255 255 / 86%);
    box-shadow: none;
  }
  .status-filters.group-context-active .group-filter :deep(.select-trigger:hover),
  .status-filters.group-context-active .group-filter :deep(.select-trigger-open) {
    border-color: color-mix(in srgb, var(--brand-500) 42%, #cbd5e1);
    background: #fff;
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand-500) 10%, transparent);
  }
  .status-filters.group-context-active .group-filter :deep(.select-value) {
    flex: 0 0 auto;
  }
  .status-filters.group-context-active .group-filter :deep(.select-icon) {
    display: none;
  }
  .status-filters.group-context-active .group-filter-selected-text {
    display: none;
  }
  .status-filters.group-context-active .group-filter-mobile-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
  }
  .status-filters .group-filter :deep(.select-trigger) {
    height: 42px;
    min-height: 42px;
    border-color: rgba(203, 213, 225, 0.9);
    border-radius: 0.7rem;
    background: #fff;
    box-shadow: 0 1px 2px rgb(15 23 42 / 4%);
  }
  .status-filters .group-filter :deep(.select-trigger-open) {
    border-color: var(--brand-border-strong);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--brand-500) 18%, transparent);
  }
  .group-context {
    display: flex;
    flex: 1 1 auto;
    min-width: 0;
    align-items: center;
    gap: 6px;
    min-height: 20px;
    overflow: hidden;
    padding: 0 3px;
  }
  .group-context-accent {
    @apply bg-primary-500 dark:bg-primary-400;
    width: 2px;
    height: 15px;
    flex: 0 0 2px;
    border-radius: 999px;
  }
  .group-context-name {
    @apply text-slate-700 dark:text-slate-200;
    display: block;
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
    font-weight: 550;
  }
  .dark .model-status-page .status-filters {
    background: linear-gradient(to bottom, #0f172a 0%, #0f172a 78%, rgb(15 23 42 / 0%) 100%);
    box-shadow: none;
  }
  .dark .status-filters.group-context-active {
    border-bottom-color: rgba(51, 65, 85, 0.72);
    background: rgb(15 23 42 / 88%);
  }
  .dark .status-filters.group-context-active .group-filter :deep(.select-trigger) {
    border-color: rgba(100, 116, 139, 0.5);
    background: rgb(15 23 42 / 82%);
  }
  .model-status-page .group-heading {
    position: relative;
    z-index: 0;
    min-height: 24px;
    margin: 14px 0 10px;
    padding: 0 2px;
    border: 0;
    border-radius: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
  }
  .status-group-context-active .group-heading {
    visibility: hidden;
  }
  .model-status-page .group-heading .group-title {
    gap: 7px;
  }
  .model-status-page .group-heading .group-accent {
    width: 2px;
    min-width: 2px;
    min-height: 18px;
    margin-top: 0;
  }
  .model-status-page .group-heading h2 {
    @apply text-slate-500 dark:text-dark-400;
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
  }
  .dark .model-status-page .status-filters .group-filter :deep(.select-trigger) {
    border-color: rgba(51, 65, 85, 0.95);
    background: rgba(15, 23, 42, 0.75);
    box-shadow: 0 1px 2px rgb(0 0 0 / 16%);
  }
  .model-status-page .model-identity {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    column-gap: 10px;
    row-gap: 4px;
    align-items: center;
  }
  .model-status-page .model-title {
    grid-column: 1 / -1;
    min-height: 38px;
  }
  .model-status-page .model-meta {
    min-width: 0;
    margin-top: 0;
  }
  .model-status-page .model-health {
    grid-column: 2;
    min-height: 24px;
    margin-top: 0;
    padding-left: 0;
    justify-content: flex-end;
    gap: 8px;
  }
  .model-status-page .health-light {
    width: 12px;
    height: 12px;
    flex-basis: 12px;
    box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 16%, transparent), 0 0 10px color-mix(in srgb, currentColor 28%, transparent);
  }
  .model-status-page .health-badge {
    white-space: nowrap;
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
