<template>
  <article
    class="w-full overflow-hidden rounded-2xl border border-gray-200/90 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800"
  >
    <header class="flex flex-wrap items-start justify-between gap-3 px-4 pb-3 pt-4 sm:px-5">
      <div class="min-w-0">
        <h3 class="break-words text-sm font-semibold leading-5 text-gray-900 dark:text-white">
          {{ model.name }}
        </h3>
        <div class="mt-2 flex flex-wrap items-center gap-2">
          <span
            class="rounded-md border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-300"
          >
            {{ billingModeLabel }}
          </span>
          <span
            v-if="model.intervals.length > 0"
            class="rounded-md border border-amber-200 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300"
          >
            {{ t(`${catalogKey}.tieredPricing`) }} · {{ t(`${catalogKey}.startingFrom`) }}
          </span>
        </div>
      </div>

      <div class="shrink-0 text-right">
        <p class="text-[11px] leading-4 text-gray-500 dark:text-gray-400">
          {{ t(`${catalogKey}.effectiveRate`) }}
        </p>
        <p class="font-mono text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-100">
          {{ formatRate(model.normalRate) }}×
        </p>
      </div>
    </header>

    <div
      v-if="hasDisplayPricing"
      data-testid="price-comparison"
      class="grid grid-cols-2 gap-2 px-3 pb-3 sm:gap-3 sm:px-5 lg:grid-cols-2"
      :aria-hidden="expanded && model.intervals.length > 0 ? 'true' : undefined"
    >
      <section
        data-testid="official-price"
        class="min-w-0 rounded-xl border border-gray-200 bg-gray-50/90 p-3 dark:border-dark-500 dark:bg-dark-700/70"
      >
        <h4 class="text-xs font-semibold text-gray-500 dark:text-gray-300">
          {{ t(`${catalogKey}.officialPrice`) }}
        </h4>
        <div class="mt-2 space-y-2.5">
          <div v-for="metric in primaryMetrics" :key="metric.key" class="min-w-0">
            <p v-if="metric.label" class="text-[11px] text-gray-500 dark:text-gray-400">
              {{ metric.label }}
            </p>
            <p
              class="mt-0.5 break-words font-mono text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-100"
            >
              {{ formatOfficial(metric.value?.official ?? null, metric.scale) }}
              <span class="text-[10px] font-medium text-gray-400 dark:text-gray-500">
                {{ metric.unit }}
              </span>
            </p>
          </div>
        </div>
      </section>

      <section
        data-testid="site-price"
        class="min-w-0 rounded-xl border border-primary-200 bg-primary-50/70 p-3 dark:border-primary-500/30 dark:bg-primary-500/10"
      >
        <h4 class="text-xs font-semibold text-primary-700 dark:text-primary-200">
          {{ t(`${catalogKey}.sitePrice`) }}
        </h4>
        <div class="mt-2 space-y-2.5">
          <div v-for="metric in primaryMetrics" :key="metric.key" class="min-w-0">
            <p v-if="metric.label" class="text-[11px] text-primary-700/70 dark:text-primary-200/70">
              {{ metric.label }}
            </p>
            <template v-if="metric.value?.peakSite != null">
              <p class="mt-0.5 text-[10px] font-medium text-primary-700/70 dark:text-primary-200/70">
                {{ t(`${catalogKey}.regularPrice`) }}
              </p>
              <p
                class="break-words font-mono text-sm font-bold tabular-nums text-primary-700 dark:text-primary-200"
              >
                {{ formatCny(metric.value.site, metric.scale) }}
                <span class="text-[10px] font-medium opacity-70">{{ metric.unit }}</span>
              </p>
              <p class="mt-1.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
                {{ t(`${catalogKey}.peakPrice`) }}
              </p>
              <p
                class="break-words font-mono text-sm font-bold tabular-nums text-amber-700 dark:text-amber-300"
              >
                {{ formatCny(metric.value.peakSite, metric.scale) }}
                <span class="text-[10px] font-medium opacity-70">{{ metric.unit }}</span>
              </p>
            </template>
            <p
              v-else
              class="mt-0.5 break-words font-mono text-sm font-bold tabular-nums text-primary-700 dark:text-primary-200"
            >
              {{ formatCny(metric.value?.site ?? null, metric.scale) }}
              <span class="text-[10px] font-medium opacity-70">{{ metric.unit }}</span>
            </p>
          </div>
        </div>
      </section>
    </div>

    <div
      v-else
      data-testid="unpriced-state"
      class="mx-4 mb-4 rounded-xl border border-dashed border-amber-300 bg-amber-50 px-4 py-3 text-sm font-medium text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300 sm:mx-5"
    >
      {{ t(`${catalogKey}.unpriced`) }}
    </div>

    <template v-if="hasDetails">
      <div
        v-if="expanded"
        :id="detailRegionId"
        data-testid="price-details"
        class="border-t border-gray-100 bg-gray-50/50 px-3 py-3 dark:border-dark-600 dark:bg-dark-900/20 sm:px-5"
      >
        <div v-if="model.intervals.length > 0" class="space-y-3">
          <section
            v-for="interval in model.intervals"
            :key="interval.key"
            data-testid="pricing-tier"
            class="rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800"
          >
            <h5 class="text-xs font-semibold text-gray-800 dark:text-gray-100">
              {{ intervalLabel(interval) }}
            </h5>
            <div class="mt-3 space-y-2">
              <DetailPriceRow
                v-for="metric in priceMetrics(interval.prices)"
                :key="metric.key"
                :label="metric.label"
                :value="metric.value"
                :scale="metric.scale"
                :unit="metric.unit"
              />
            </div>
          </section>
        </div>

        <div v-else class="space-y-2">
          <DetailPriceRow
            v-for="metric in detailMetrics"
            :key="metric.key"
            :label="metric.label"
            :value="metric.value"
            :scale="metric.scale"
            :unit="metric.unit"
          />
        </div>
      </div>

      <button
        type="button"
        class="flex min-h-11 w-full items-center justify-center gap-2 border-t border-gray-100 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700 dark:hover:text-primary-300"
        :aria-expanded="expanded"
        :aria-controls="detailRegionId"
        @click="expanded = !expanded"
      >
        {{ expanded ? t(`${catalogKey}.hideDetails`) : t(`${catalogKey}.showDetails`) }}
        <svg
          class="h-4 w-4 transition-transform"
          :class="expanded ? 'rotate-180' : ''"
          viewBox="0 0 20 20"
          fill="none"
          aria-hidden="true"
        >
          <path d="m5 7.5 5 5 5-5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>
    </template>
  </article>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'
import { formatScaled } from '@/utils/pricing'
import type {
  CatalogModelEntry,
  CatalogPriceCollection,
  CatalogPriceValue,
  CatalogPricingInterval,
} from './availableChannelCatalog'

const catalogKey = 'availableChannels.catalog'
const perMillion = 1_000_000
const { t } = useI18n()

const props = defineProps<{
  model: CatalogModelEntry
}>()

interface PriceMetric {
  key: string
  label: string
  value: CatalogPriceValue | null
  scale: number
  unit: string
}

function preciseNumber(value: number, scale: number, minimumFractionDigits = 2): string {
  let result = (value * scale).toPrecision(10).replace(/\.?0+$/, '')
  if (!result.includes('e')) {
    const decimalIndex = result.indexOf('.')
    const digits = decimalIndex === -1 ? 0 : result.length - decimalIndex - 1
    if (digits < minimumFractionDigits) {
      result = (decimalIndex === -1 ? `${result}.` : result)
        + '0'.repeat(minimumFractionDigits - digits)
    }
  }
  return result
}

function formatCny(value: number | null, scale: number): string {
  return value == null ? '-' : `¥${preciseNumber(value, scale)}`
}

function formatOfficial(value: number | null, scale: number): string {
  return formatScaled(value, scale, 2)
}

function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return Number(value.toPrecision(6)).toString()
}

function safeId(value: string): string {
  const readable = value
    .toLocaleLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 48) || 'model'
  let hash = 2166136261
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return `available-channel-model-price-${readable}-${(hash >>> 0).toString(36)}`
}

function hasValue(value: CatalogPriceValue | null): boolean {
  return value != null
    && (value.official != null || value.site != null || value.peakSite != null)
}

function hasCollectionValue(prices: CatalogPriceCollection): boolean {
  return Object.values(prices).some(hasValue)
}

function metricsForCollection(
  prices: CatalogPriceCollection,
  includeExtended: boolean,
): PriceMetric[] {
  const tokenMetrics: PriceMetric[] = [
    {
      key: 'input',
      label: t(`${catalogKey}.input`),
      value: prices.input,
      scale: perMillion,
      unit: t(`${catalogKey}.perMillion`),
    },
    {
      key: 'output',
      label: t(`${catalogKey}.output`),
      value: prices.output,
      scale: perMillion,
      unit: t(`${catalogKey}.perMillion`),
    },
  ]
  if (!includeExtended) return tokenMetrics
  return tokenMetrics.concat([
    {
      key: 'cache-write',
      label: t(`${catalogKey}.cacheWrite`),
      value: prices.cacheWrite,
      scale: perMillion,
      unit: t(`${catalogKey}.perMillion`),
    },
    {
      key: 'cache-read',
      label: t(`${catalogKey}.cacheRead`),
      value: prices.cacheRead,
      scale: perMillion,
      unit: t(`${catalogKey}.perMillion`),
    },
    {
      key: 'image-input',
      label: t(`${catalogKey}.imageInput`),
      value: prices.imageInput,
      scale: perMillion,
      unit: t(`${catalogKey}.perMillion`),
    },
    {
      key: 'image-output',
      label: t(`${catalogKey}.imageOutput`),
      value: prices.imageOutput,
      scale: perMillion,
      unit: t(`${catalogKey}.perMillion`),
    },
  ])
}

function requestMetric(prices: CatalogPriceCollection, unit: string): PriceMetric[] {
  return [{ key: 'request', label: '', value: prices.request, scale: 1, unit }]
}

function priceMetrics(prices: CatalogPriceCollection): PriceMetric[] {
  if (props.model.billingMode === BILLING_MODE_PER_REQUEST) {
    return requestMetric(prices, t(`${catalogKey}.perRequest`)).filter((metric) => hasValue(metric.value))
  }
  if (props.model.billingMode === BILLING_MODE_IMAGE) {
    return requestMetric(prices, t(`${catalogKey}.perImage`)).filter((metric) => hasValue(metric.value))
  }
  if (props.model.billingMode === BILLING_MODE_TOKEN) {
    return metricsForCollection(prices, true).filter((metric) => hasValue(metric.value))
  }
  if (hasValue(prices.request)) {
    return requestMetric(prices, t(`${catalogKey}.perRequest`))
  }
  return metricsForCollection(prices, true).filter((metric) => hasValue(metric.value))
}

const expanded = ref(false)
const detailRegionId = computed(() => safeId(props.model.key))
const primaryPrices = computed(() => props.model.intervals[0]?.prices ?? props.model.prices)
const hasDisplayPricing = computed(() =>
  hasCollectionValue(primaryPrices.value)
  || props.model.intervals.some((interval) => hasCollectionValue(interval.prices)),
)
const billingModeLabel = computed(() => {
  const mode = props.model.billingMode
  if (mode === BILLING_MODE_TOKEN || mode === BILLING_MODE_PER_REQUEST || mode === BILLING_MODE_IMAGE) {
    return t(`${catalogKey}.billingMode.${mode}`)
  }
  return t(`${catalogKey}.billingMode.unknown`)
})
const primaryMetrics = computed<PriceMetric[]>(() => {
  const prices = primaryPrices.value
  if (props.model.billingMode === BILLING_MODE_PER_REQUEST) {
    return requestMetric(prices, t(`${catalogKey}.perRequest`))
  }
  if (props.model.billingMode === BILLING_MODE_IMAGE) {
    return requestMetric(prices, t(`${catalogKey}.perImage`))
  }
  if (props.model.billingMode === BILLING_MODE_TOKEN) {
    return metricsForCollection(prices, false)
  }
  if (hasValue(prices.request)) {
    return requestMetric(prices, t(`${catalogKey}.perRequest`))
  }
  return metricsForCollection(prices, false).filter((metric) => hasValue(metric.value))
})
const detailMetrics = computed(() => metricsForCollection(props.model.prices, true)
  .slice(2)
  .filter((metric) => hasValue(metric.value)))
const hasDetails = computed(() => props.model.intervals.length > 0 || detailMetrics.value.length > 0)

function intervalLabel(interval: CatalogPricingInterval): string {
  if (interval.tierLabel) return interval.tierLabel
  const number = new Intl.NumberFormat('en-US')
  if (interval.maxTokens == null) {
    return t(`${catalogKey}.rangeFrom`, { min: number.format(interval.minTokens) })
  }
  return t(`${catalogKey}.rangeBetween`, {
    min: number.format(interval.minTokens),
    max: number.format(interval.maxTokens),
  })
}

const DetailPriceRow = defineComponent({
  name: 'DetailPriceRow',
  props: {
    label: { type: String, required: true },
    value: { type: Object as PropType<CatalogPriceValue | null>, default: null },
    scale: { type: Number, required: true },
    unit: { type: String, required: true },
  },
  setup(rowProps) {
    return () => h('div', {
      class: 'rounded-lg border border-gray-200/80 bg-white p-3 dark:border-dark-500 dark:bg-dark-800',
    }, [
      rowProps.label ? h('p', {
        class: 'mb-2 text-xs font-medium text-gray-700 dark:text-gray-200',
      }, rowProps.label) : null,
      h('div', { class: 'grid grid-cols-2 gap-3' }, [
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'text-[10px] text-gray-500 dark:text-gray-400' }, t(`${catalogKey}.officialPrice`)),
          h('p', {
            class: 'mt-0.5 break-words font-mono text-xs font-semibold tabular-nums text-gray-800 dark:text-gray-100',
          }, `${formatOfficial(rowProps.value?.official ?? null, rowProps.scale)} ${rowProps.unit}`),
        ]),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'text-[10px] text-primary-700 dark:text-primary-200' }, t(`${catalogKey}.sitePrice`)),
          rowProps.value?.peakSite != null
            ? h('div', [
                h('p', { class: 'mt-1 text-[10px] text-primary-700/70 dark:text-primary-200/70' }, t(`${catalogKey}.regularPrice`)),
                h('p', {
                  class: 'break-words font-mono text-xs font-bold tabular-nums text-primary-700 dark:text-primary-200',
                }, `${formatCny(rowProps.value.site, rowProps.scale)} ${rowProps.unit}`),
                h('p', { class: 'mt-1 text-[10px] text-amber-700 dark:text-amber-300' }, t(`${catalogKey}.peakPrice`)),
                h('p', {
                  class: 'break-words font-mono text-xs font-bold tabular-nums text-amber-700 dark:text-amber-300',
                }, `${formatCny(rowProps.value.peakSite, rowProps.scale)} ${rowProps.unit}`),
              ])
            : h('p', {
                class: 'mt-0.5 break-words font-mono text-xs font-bold tabular-nums text-primary-700 dark:text-primary-200',
              }, `${formatCny(rowProps.value?.site ?? null, rowProps.scale)} ${rowProps.unit}`),
        ]),
      ]),
    ])
  },
})
</script>
