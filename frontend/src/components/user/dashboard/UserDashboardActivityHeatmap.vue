<template>
  <section class="workspace-surface activity-heatmap" data-testid="activity-heatmap">
    <div class="activity-heatmap-header">
      <div class="activity-heatmap-metrics" role="group" :aria-label="t('dashboard.activityHeatmap.metricLabel')">
        <button
          v-for="option in metricOptions"
          :key="option.value"
          type="button"
          class="activity-heatmap-metric"
          :class="{ 'activity-heatmap-metric-active': metric === option.value }"
          :aria-pressed="metric === option.value"
          @click="metric = option.value"
        >
          {{ t(option.label) }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="activity-heatmap-state" aria-live="polite">
      <LoadingSpinner size="sm" />
      <span>{{ t('common.loading') }}</span>
    </div>
    <div v-else-if="error" class="activity-heatmap-state activity-heatmap-error" role="alert">
      <span>{{ t('dashboard.activityHeatmap.failedToLoad') }}</span>
      <button type="button" class="activity-heatmap-retry" @click="$emit('retry')">
        {{ t('common.retry') }}
      </button>
    </div>
    <div v-else class="activity-heatmap-scroll">
      <div class="activity-heatmap-track">
        <div class="activity-heatmap-canvas" :style="{ '--activity-week-count': weekCount }">
          <div class="activity-heatmap-months" aria-hidden="true">
            <span
              v-for="month in monthLabels"
              :key="`${month.label}-${month.weekIndex}`"
              class="activity-heatmap-month"
              :style="{ gridColumn: `${month.weekIndex + 1} / span ${month.span}` }"
            >
              {{ month.label }}
            </span>
          </div>
          <div class="activity-heatmap-body">
            <div class="activity-heatmap-weekdays" aria-hidden="true">
              <span v-for="(label, index) in weekdayLabels" :key="index" :class="{ 'activity-heatmap-weekday-hidden': index % 2 === 1 }">
                {{ label }}
              </span>
            </div>
            <div class="activity-heatmap-grid" :style="{ gridTemplateColumns: `repeat(${weekCount}, var(--activity-cell-size))` }">
              <div
                v-for="cell in cells"
                :key="cell.date"
                class="activity-heatmap-cell-wrap"
                :style="{ gridColumn: cell.weekIndex + 1, gridRow: cell.weekday + 1 }"
                @mouseleave="closeOnLeave($event, cell.date)"
                @focusout="closeOnLeave($event, cell.date)"
                @keydown.esc.stop="activeDate = null"
              >
                <button
                  type="button"
                  class="activity-heatmap-cell"
                  :class="`activity-heatmap-level-${cellLevel(cell)}`"
                  :aria-label="cellLabel(cell)"
                  @mouseenter="activeDate = cell.date"
                  @focus="activeDate = cell.date"
                  @click="activeDate = cell.date"
                />
                <div
                  v-if="activeDate === cell.date"
                  class="activity-heatmap-popover"
                  :class="`activity-heatmap-popover-${popoverPlacement(cell)}`"
                  role="status"
                >
                  <p class="activity-heatmap-popover-date">{{ cell.date }}</p>
                  <dl>
                    <div><dt>{{ t('dashboard.activityHeatmap.successRequests') }}</dt><dd>{{ formatNumber(cell.success_requests) }}</dd></div>
                    <div><dt>{{ t('dashboard.activityHeatmap.failedRequests') }}</dt><dd>{{ formatNumber(cell.failed_requests) }}</dd></div>
                    <div><dt>{{ t('dashboard.activityHeatmap.inputTokens') }}</dt><dd>{{ formatTokens(cell.input_tokens) }}</dd></div>
                    <div><dt>{{ t('dashboard.activityHeatmap.outputTokens') }}</dt><dd>{{ formatTokens(cell.output_tokens) }}</dd></div>
                    <div><dt>{{ t('dashboard.activityHeatmap.totalTokens') }}</dt><dd>{{ formatTokens(cell.total_tokens) }}</dd></div>
                    <div><dt>{{ t('dashboard.activityHeatmap.billedCost') }}</dt><dd>${{ formatCost(cell.billed_cost) }}</dd></div>
                    <div><dt>{{ t('dashboard.activityHeatmap.models') }}</dt><dd>{{ formatNumber(cell.model_count) }}</dd></div>
                  </dl>
                  <router-link
                    v-if="hasRetainedRecords(cell.date)"
                    class="activity-heatmap-link"
                    :to="{ path: '/usage', query: { start_date: cell.date, end_date: cell.date } }"
                  >
                    {{ t('dashboard.activityHeatmap.viewDay') }}
                  </router-link>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="activity-heatmap-legend">
          <span>{{ t('dashboard.activityHeatmap.less') }}</span>
          <span v-for="level in 5" :key="level" class="activity-heatmap-legend-cell" :class="`activity-heatmap-level-${level - 1}`" />
          <span>{{ t('dashboard.activityHeatmap.more') }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatCostFixed, formatNumberLocaleString, formatTokenCount } from '@/utils/format'
import { buildActivityHeatmapCells, getActivityHeatmapLevel, type ActivityHeatmapCell, type ActivityHeatmapDay } from './activityHeatmap'

type ActivityMetric = 'requests' | 'tokens' | 'cost'

const props = defineProps<{
  startDate: string
  endDate: string
  days: ActivityHeatmapDay[]
  loading: boolean
  error: boolean
}>()

defineEmits<{ retry: [] }>()

const { t } = useI18n()
const metric = ref<ActivityMetric>('requests')
const activeDate = ref<string | null>(null)
const retainedRecordDates = computed(() => new Set(props.days.map((day) => day.date)))

const hasRetainedRecords = (date: string): boolean => retainedRecordDates.value.has(date)

function closeOnLeave(event: MouseEvent | FocusEvent, date: string) {
  if (activeDate.value !== date) return
  const region = event.currentTarget
  if (region instanceof Node && event.relatedTarget instanceof Node && region.contains(event.relatedTarget)) return
  activeDate.value = null
}

const metricOptions: Array<{ value: ActivityMetric; label: string }> = [
  { value: 'requests', label: 'dashboard.activityHeatmap.metricRequests' },
  { value: 'tokens', label: 'dashboard.activityHeatmap.metricTokens' },
  { value: 'cost', label: 'dashboard.activityHeatmap.metricCost' },
]

const cells = computed(() => buildActivityHeatmapCells(props.startDate, props.endDate, props.days))
const weekCount = computed(() => Math.max(1, ...cells.value.map((cell) => cell.weekIndex + 1)))

const valueForMetric = (cell: ActivityHeatmapDay) => {
  if (metric.value === 'tokens') return cell.total_tokens
  if (metric.value === 'cost') return cell.billed_cost
  return cell.success_requests
}

const nonZeroValues = computed(() => cells.value.map(valueForMetric).filter((value) => value > 0))
const cellLevel = (cell: ActivityHeatmapDay) => getActivityHeatmapLevel(valueForMetric(cell), nonZeroValues.value)

const popoverPlacement = (cell: ActivityHeatmapCell) => {
  const edgeColumnCount = Math.min(3, Math.max(1, Math.floor(weekCount.value / 2)))
  const horizontal = cell.weekIndex < edgeColumnCount
    ? 'start'
    : cell.weekIndex >= weekCount.value - edgeColumnCount
      ? 'end'
      : 'center'
  const vertical = cell.weekday >= 3 ? 'above' : 'below'
  return `${vertical}-${horizontal}`
}

const monthLabels = computed(() => {
  const labels: Array<{ label: string; weekIndex: number; span: number }> = []
  let lastMonth = ''
  for (const cell of cells.value) {
    const month = cell.date.slice(0, 7)
    if (month === lastMonth) continue
    lastMonth = month
    const label = new Date(`${cell.date}T00:00:00Z`).toLocaleString(undefined, { month: 'short', timeZone: 'UTC' })
    const previous = labels[labels.length - 1]
    if (previous?.weekIndex === cell.weekIndex) {
      labels[labels.length - 1] = { label, weekIndex: cell.weekIndex, span: 1 }
    } else {
      labels.push({ label, weekIndex: cell.weekIndex, span: 1 })
    }
  }
  return labels.map((item, index) => ({ ...item, span: Math.max(1, (labels[index + 1]?.weekIndex ?? weekCount.value) - item.weekIndex) }))
})

const weekdayLabels = computed(() => [
  t('dashboard.activityHeatmap.weekdays.sun'),
  t('dashboard.activityHeatmap.weekdays.mon'),
  t('dashboard.activityHeatmap.weekdays.tue'),
  t('dashboard.activityHeatmap.weekdays.wed'),
  t('dashboard.activityHeatmap.weekdays.thu'),
  t('dashboard.activityHeatmap.weekdays.fri'),
  t('dashboard.activityHeatmap.weekdays.sat'),
])

const formatNumber = (value: number) => formatNumberLocaleString(value)
const formatTokens = (value: number) => formatTokenCount(value)
const formatCost = (value: number) => formatCostFixed(value)
const cellLabel = (cell: ActivityHeatmapDay) => `${cell.date}: ${formatNumber(cell.success_requests)} ${t('dashboard.activityHeatmap.successRequests')}`
</script>

<style scoped>
.activity-heatmap { --activity-cell-size: 0.78rem; --activity-cell-gap: 0.22rem; --activity-primary: #2563eb; --activity-primary-soft: rgb(37 99 235 / 12%); padding: 1rem; }
.dark .activity-heatmap { --activity-primary: #60a5fa; --activity-primary-soft: rgb(96 165 250 / 15%); }
.activity-heatmap-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.activity-heatmap-metrics { display: inline-flex; flex-wrap: wrap; gap: 0.25rem; }
.activity-heatmap-metric { border: 1px solid var(--workspace-rule); border-radius: 6px; background: var(--workspace-surface); padding: 0.35rem 0.55rem; color: var(--workspace-muted); font-size: 0.6875rem; transition: background-color 160ms ease, color 160ms ease, border-color 160ms ease; }
.activity-heatmap-metric:hover, .activity-heatmap-metric-active { border-color: var(--activity-primary); background: var(--activity-primary-soft); color: var(--activity-primary); }
.activity-heatmap-state { display: flex; min-height: 8rem; align-items: center; justify-content: center; gap: 0.5rem; color: var(--workspace-muted); font-size: 0.75rem; }
.activity-heatmap-error { flex-direction: column; }
.activity-heatmap-retry, .activity-heatmap-link { color: var(--activity-primary); font-size: 0.75rem; font-weight: 600; }
.activity-heatmap-scroll { overflow: visible; padding: 0.75rem 0 0.25rem; }
.activity-heatmap-track { width: max-content; margin: 0 auto; }
.activity-heatmap-canvas { width: max-content; min-width: calc(3rem + var(--activity-week-count) * (var(--activity-cell-size) + var(--activity-cell-gap))); }
.activity-heatmap-months, .activity-heatmap-grid { display: grid; column-gap: var(--activity-cell-gap); }
.activity-heatmap-months { margin-left: 2rem; grid-template-columns: repeat(var(--activity-week-count), var(--activity-cell-size)); min-height: 1.25rem; }
.activity-heatmap-month { overflow: hidden; color: var(--workspace-muted); font-size: 0.6875rem; white-space: nowrap; }
.activity-heatmap-body { display: flex; gap: 0.45rem; }
.activity-heatmap-weekdays { display: grid; grid-template-rows: repeat(7, var(--activity-cell-size)); row-gap: var(--activity-cell-gap); width: 1.55rem; color: var(--workspace-muted); font-size: 0.625rem; line-height: var(--activity-cell-size); text-align: right; }
.activity-heatmap-weekday-hidden { visibility: hidden; }
.activity-heatmap-grid { grid-template-rows: repeat(7, var(--activity-cell-size)); grid-auto-flow: column; row-gap: var(--activity-cell-gap); position: relative; }
.activity-heatmap-cell-wrap { position: relative; width: var(--activity-cell-size); height: var(--activity-cell-size); }
.activity-heatmap-cell, .activity-heatmap-legend-cell { display: block; width: var(--activity-cell-size); height: var(--activity-cell-size); border: 1px solid transparent; border-radius: 3px; background: var(--workspace-hover); }
.activity-heatmap-cell { cursor: pointer; }
.activity-heatmap-cell:hover, .activity-heatmap-cell:focus-visible { border-color: var(--workspace-ink); outline: none; }
.activity-heatmap-level-0 { background: color-mix(in srgb, var(--workspace-hover) 84%, var(--activity-primary)); }
.activity-heatmap-level-1 { background: color-mix(in srgb, var(--activity-primary-soft) 55%, var(--activity-primary)); }
.activity-heatmap-level-2 { background: color-mix(in srgb, var(--activity-primary-soft) 25%, var(--activity-primary)); }
.activity-heatmap-level-3 { background: color-mix(in srgb, var(--activity-primary) 72%, #0f3b78); }
.activity-heatmap-level-4 { background: color-mix(in srgb, var(--activity-primary) 58%, #062957); }
.activity-heatmap-popover { position: absolute; z-index: 20; top: calc(100% + 0.45rem); left: 50%; width: 12rem; transform: translateX(-50%); border: 1px solid var(--workspace-rule); border-radius: 7px; background: var(--workspace-surface); padding: 0.75rem; box-shadow: var(--workspace-shadow); color: var(--workspace-ink); }
.activity-heatmap-popover-above-start { top: auto; right: auto; bottom: calc(100% + 0.45rem); left: 0; transform: none; }
.activity-heatmap-popover-above-center { top: auto; right: auto; bottom: calc(100% + 0.45rem); left: 50%; transform: translateX(-50%); }
.activity-heatmap-popover-above-end { top: auto; right: 0; bottom: calc(100% + 0.45rem); left: auto; transform: none; }
.activity-heatmap-popover-below-start { right: auto; left: 0; transform: none; }
.activity-heatmap-popover-below-end { right: 0; left: auto; transform: none; }
/* Bridge the gap so the pointer can reach the record link without closing the popover. */
.activity-heatmap-popover::before { content: ''; position: absolute; right: 0; bottom: 100%; left: 0; height: 0.5rem; }
.activity-heatmap-popover-above-start::before,
.activity-heatmap-popover-above-center::before,
.activity-heatmap-popover-above-end::before { top: 100%; bottom: auto; }
.activity-heatmap-popover-date { margin-bottom: 0.45rem; font-size: 0.75rem; font-weight: 600; }
.activity-heatmap-popover dl { display: grid; gap: 0.25rem; margin: 0 0 0.55rem; font-size: 0.6875rem; }
.activity-heatmap-popover dl div { display: flex; justify-content: space-between; gap: 0.5rem; }
.activity-heatmap-popover dt { color: var(--workspace-muted); }
.activity-heatmap-popover dd { margin: 0; font-variant-numeric: tabular-nums; }
.activity-heatmap-link { display: inline-block; border-top: 1px solid var(--workspace-divider); padding-top: 0.45rem; }
.activity-heatmap-legend { display: flex; align-items: center; justify-content: flex-end; gap: 0.3rem; padding-top: 0.5rem; color: var(--workspace-muted); font-size: 0.625rem; }
@media (max-width: 639px) {
  .activity-heatmap { padding: 0.875rem; }
  .activity-heatmap-header { flex-direction: column; }
  .activity-heatmap-metrics { width: 100%; }
  .activity-heatmap-metric { flex: 1; }
  .activity-heatmap-track { margin: 0; }
  .activity-heatmap-scroll { overflow-x: auto; overflow-y: hidden; }
}
@media (prefers-reduced-motion: reduce) { .activity-heatmap-metric { transition: none; } }
</style>
