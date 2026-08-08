<template>
  <article
    data-testid="model-price-row"
    class="w-full min-w-0 overflow-hidden rounded-2xl border border-gray-200/90 bg-white shadow-sm [contain-intrinsic-size:auto_180px] [content-visibility:auto] dark:border-dark-600 dark:bg-dark-800 xl:grid xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(88px,0.5fr)_minmax(72px,0.4fr)] xl:items-start xl:gap-4 xl:p-3"
  >
    <header class="flex flex-wrap items-start justify-between gap-3 px-4 pb-3 pt-4 sm:px-5 xl:contents">
      <div class="min-w-0 xl:col-start-1 xl:row-start-1 xl:p-3">
        <h4 class="break-words text-sm font-semibold leading-5 text-gray-900 [overflow-wrap:anywhere] dark:text-white">
          {{ model.name }}
        </h4>
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

      <div
        data-testid="effective-rate"
        class="min-w-0 shrink-0 text-right xl:col-start-4 xl:row-start-1 xl:p-3 xl:text-left"
      >
        <p class="text-xs leading-4 text-gray-600 dark:text-gray-300">
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
      class="grid grid-cols-2 gap-2 px-3 pb-3 sm:gap-3 sm:px-5 xl:contents"
      :aria-hidden="expanded && model.intervals.length > 0 ? 'true' : undefined"
    >
      <section
        data-testid="official-price"
        class="min-w-0 rounded-xl border border-gray-200 bg-gray-50/90 p-3 dark:border-dark-500 dark:bg-dark-700/70 xl:col-start-2 xl:row-start-1"
      >
        <h5 class="text-xs font-semibold text-gray-600 dark:text-gray-300 xl:sr-only">
          {{ t(`${catalogKey}.officialPrice`) }}
        </h5>
        <div class="mt-2 space-y-2.5 xl:mt-0">
          <div v-for="metric in primaryMetrics" :key="metric.key" class="min-w-0">
            <p v-if="metric.label" class="text-xs text-gray-600 dark:text-gray-300">
              {{ metric.label }}
            </p>
            <p
              class="mt-0.5 break-words font-mono text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-100"
            >
              {{ formatOfficial(metric.value?.official ?? null, metric.scale) }}
              <span class="text-xs font-medium text-gray-600 dark:text-gray-300">
                {{ metric.unit }}
              </span>
            </p>
          </div>
        </div>
      </section>

      <section
        data-testid="site-price"
        class="min-w-0 rounded-xl border border-primary-200 bg-primary-50/70 p-3 dark:border-primary-500/30 dark:bg-primary-500/10 xl:col-start-3 xl:row-start-1"
      >
        <h5 class="text-xs font-semibold text-primary-700 dark:text-primary-200 xl:sr-only">
          {{ t(`${catalogKey}.sitePrice`) }}
        </h5>
        <div class="mt-2 space-y-2.5 xl:mt-0">
          <div v-for="metric in primaryMetrics" :key="metric.key" class="min-w-0">
            <p v-if="metric.label" class="text-xs text-primary-700 dark:text-primary-200">
              {{ metric.label }}
            </p>
            <template v-if="metric.value?.peakSite != null">
              <p class="mt-0.5 text-xs font-medium text-primary-700 dark:text-primary-200">
                {{ t(`${catalogKey}.regularPrice`) }}
              </p>
              <p
                class="break-words font-mono text-sm font-bold tabular-nums text-primary-700 dark:text-primary-200"
              >
                {{ formatCny(metric.value.site, metric.scale) }}
                <span class="text-xs font-medium text-primary-700 dark:text-primary-200">{{ metric.unit }}</span>
              </p>
              <p class="mt-1.5 text-xs font-medium text-amber-700 dark:text-amber-300">
                {{ t(`${catalogKey}.peakPrice`) }}
              </p>
              <p
                class="break-words font-mono text-sm font-bold tabular-nums text-amber-700 dark:text-amber-300"
              >
                {{ formatCny(metric.value.peakSite, metric.scale) }}
                <span class="text-xs font-medium text-amber-700 dark:text-amber-300">{{ metric.unit }}</span>
              </p>
            </template>
            <p
              v-else
              class="mt-0.5 break-words font-mono text-sm font-bold tabular-nums text-primary-700 dark:text-primary-200"
            >
              {{ formatCny(metric.value?.site ?? null, metric.scale) }}
              <span class="text-xs font-medium text-primary-700 dark:text-primary-200">{{ metric.unit }}</span>
            </p>
          </div>
        </div>
      </section>
    </div>

    <div
      v-else
      data-testid="unpriced-state"
      class="mx-4 mb-4 min-w-0 rounded-xl border border-dashed border-amber-300 bg-amber-50 px-4 py-3 text-sm font-medium text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300 sm:mx-5 xl:col-span-2 xl:col-start-2 xl:row-start-1 xl:m-0 xl:flex xl:min-h-full xl:items-center"
    >
      {{ t(`${catalogKey}.unpriced`) }}
    </div>

    <template v-if="hasDetails">
      <button
        type="button"
        data-testid="price-detail-toggle"
        class="flex min-h-11 w-full min-w-0 items-center justify-center gap-2 border-t border-gray-100 px-4 py-2 text-sm font-medium text-gray-600 transition-colors motion-reduce:transition-none hover:bg-gray-50 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700 dark:hover:text-primary-300 xl:col-start-5 xl:row-start-1 xl:min-h-full xl:rounded-xl xl:border xl:px-2 xl:py-3"
        :aria-expanded="expanded"
        :aria-controls="detailRegionId"
        @click="expanded = !expanded"
      >
        <span class="xl:hidden">
          {{ expanded ? t(`${catalogKey}.hideDetails`) : t(`${catalogKey}.showDetails`) }}
        </span>
        <span class="hidden xl:inline">{{ t(`${catalogKey}.detailsColumn`) }}</span>
        <svg
          class="h-4 w-4 transition-transform motion-reduce:transition-none"
          :class="expanded ? 'rotate-180' : ''"
          viewBox="0 0 20 20"
          fill="none"
          aria-hidden="true"
        >
          <path d="m5 7.5 5 5 5-5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </button>

      <div
        v-if="expanded"
        :id="detailRegionId"
        data-testid="price-details"
        class="border-t border-gray-100 bg-gray-50/50 px-3 py-3 dark:border-dark-600 dark:bg-dark-900/20 sm:px-5 xl:col-span-5 xl:col-start-1 xl:row-start-2 xl:rounded-xl xl:border"
      >
        <div class="space-y-3">
          <section
            v-for="section in detailSections"
            :key="section.key"
            :data-testid="section.isTier ? 'pricing-tier' : undefined"
            class="rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800"
          >
            <h5 v-if="section.label" class="text-xs font-semibold text-gray-800 dark:text-gray-100">
              {{ section.label }}
            </h5>
            <div
              v-if="section.metrics.length > 0"
              :class="section.label ? 'mt-3' : ''"
              class="space-y-2"
            >
              <div
                v-for="metric in section.metrics"
                :key="metric.key"
                class="rounded-lg border border-gray-200/80 bg-white p-3 dark:border-dark-500 dark:bg-dark-800"
              >
                <p v-if="metric.label" class="mb-2 text-xs font-medium text-gray-700 dark:text-gray-200">
                  {{ metric.label }}
                </p>
                <div class="grid grid-cols-2 gap-3">
                  <div class="min-w-0">
                    <p class="text-xs text-gray-600 dark:text-gray-300">
                      {{ t(`${catalogKey}.officialPrice`) }}
                    </p>
                    <p class="mt-0.5 break-words font-mono text-xs font-semibold tabular-nums text-gray-800 dark:text-gray-100">
                      {{ formatOfficial(metric.value?.official ?? null, metric.scale) }} {{ metric.unit }}
                    </p>
                  </div>
                  <div class="min-w-0">
                    <p class="text-xs text-primary-700 dark:text-primary-200">
                      {{ t(`${catalogKey}.sitePrice`) }}
                    </p>
                    <template v-if="metric.value?.peakSite != null">
                      <p class="mt-1 text-xs text-primary-700 dark:text-primary-200">
                        {{ t(`${catalogKey}.regularPrice`) }}
                      </p>
                      <p class="break-words font-mono text-xs font-bold tabular-nums text-primary-700 dark:text-primary-200">
                        {{ formatCny(metric.value.site, metric.scale) }} {{ metric.unit }}
                      </p>
                      <p class="mt-1 text-xs text-amber-700 dark:text-amber-300">
                        {{ t(`${catalogKey}.peakPrice`) }}
                      </p>
                      <p class="break-words font-mono text-xs font-bold tabular-nums text-amber-700 dark:text-amber-300">
                        {{ formatCny(metric.value.peakSite, metric.scale) }} {{ metric.unit }}
                      </p>
                    </template>
                    <p
                      v-else
                      class="mt-0.5 break-words font-mono text-xs font-bold tabular-nums text-primary-700 dark:text-primary-200"
                    >
                      {{ formatCny(metric.value?.site ?? null, metric.scale) }} {{ metric.unit }}
                    </p>
                  </div>
                </div>
              </div>
            </div>
            <div
              v-else
              data-testid="tier-unpriced"
              class="mt-3 rounded-lg border border-dashed border-amber-300 bg-amber-50 px-3 py-2 text-xs font-medium text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300"
            >
              {{ t(`${catalogKey}.unpriced`) }}
            </div>
          </section>
        </div>
      </div>

    </template>
  </article>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'
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
const instanceUid = getCurrentInstance()?.uid ?? 0

interface PriceMetric {
  key: string
  label: string
  value: CatalogPriceValue | null
  scale: number
  unit: string
}

function formatMoney(
  value: number | null,
  scale: number,
  symbol: '$' | '¥',
  minimumFractionDigits = 2,
): string {
  if (value == null) return '-'
  const scaled = value * scale
  if (!Number.isFinite(scaled)) return '-'
  if (scaled === 0) return `${symbol}0.${'0'.repeat(minimumFractionDigits)}`

  const [rawMantissa, rawExponent] = scaled.toPrecision(10).split('e')
  let mantissa = rawMantissa.includes('.')
    ? rawMantissa.replace(/0+$/, '').replace(/\.$/, '')
    : rawMantissa
  if (rawExponent != null) {
    const exponent = Number(rawExponent)
    return `${symbol}${mantissa}e${exponent >= 0 ? '+' : ''}${exponent}`
  }

  if (minimumFractionDigits > 0) {
    const decimalIndex = mantissa.indexOf('.')
    const digits = decimalIndex === -1 ? 0 : mantissa.length - decimalIndex - 1
    if (digits < minimumFractionDigits) {
      mantissa = (decimalIndex === -1 ? `${mantissa}.` : mantissa)
        + '0'.repeat(minimumFractionDigits - digits)
    }
  }
  return `${symbol}${mantissa}`
}

function formatCny(value: number | null, scale: number): string {
  return formatMoney(value, scale, '¥')
}

function formatOfficial(value: number | null, scale: number): string {
  return formatMoney(value, scale, '$')
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

function allMeaningfulMetrics(prices: CatalogPriceCollection): PriceMetric[] {
  return metricsForCollection(prices, true)
    .concat(requestMetric(prices, t(`${catalogKey}.perRequest`)))
    .filter((metric) => hasValue(metric.value))
}

function primaryMetricsForCollection(prices: CatalogPriceCollection): PriceMetric[] {
  if (props.model.billingMode === BILLING_MODE_PER_REQUEST) {
    const request = requestMetric(prices, t(`${catalogKey}.perRequest`))
      .filter((metric) => hasValue(metric.value))
    return request.length > 0 ? request : allMeaningfulMetrics(prices)
  }
  if (props.model.billingMode === BILLING_MODE_IMAGE) {
    const request = requestMetric(prices, t(`${catalogKey}.perImage`))
      .filter((metric) => hasValue(metric.value))
    return request.length > 0 ? request : allMeaningfulMetrics(prices)
  }
  if (props.model.billingMode === BILLING_MODE_TOKEN) {
    const token = metricsForCollection(prices, false).filter((metric) => hasValue(metric.value))
    return token.length > 0 ? token : allMeaningfulMetrics(prices)
  }
  if (hasValue(prices.request)) {
    return requestMetric(prices, t(`${catalogKey}.perRequest`))
  }
  return allMeaningfulMetrics(prices)
}

function priceMetrics(prices: CatalogPriceCollection): PriceMetric[] {
  const primary = primaryMetricsForCollection(prices)
  const primaryKeys = new Set(primary.map((metric) => metric.key))
  return primary.concat(
    allMeaningfulMetrics(prices).filter((metric) => !primaryKeys.has(metric.key)),
  )
}

const expanded = ref(false)
const detailRegionId = computed(() => `${safeId(props.model.key)}-${instanceUid.toString(36)}`)
const primaryMetrics = computed<PriceMetric[]>(() => {
  const candidates = props.model.intervals
    .map((interval) => interval.prices)
    .concat(props.model.prices)
  for (const prices of candidates) {
    const metrics = primaryMetricsForCollection(prices)
    if (metrics.length > 0) return metrics
  }
  return []
})
const hasDisplayPricing = computed(() => primaryMetrics.value.length > 0)
const billingModeLabel = computed(() => {
  const mode = props.model.billingMode
  if (mode === BILLING_MODE_TOKEN || mode === BILLING_MODE_PER_REQUEST || mode === BILLING_MODE_IMAGE) {
    return t(`${catalogKey}.billingMode.${mode}`)
  }
  return t(`${catalogKey}.billingMode.unknown`)
})
const detailMetrics = computed(() => metricsForCollection(props.model.prices, true)
  .slice(2)
  .filter((metric) => hasValue(metric.value))
  .filter((metric) => !primaryMetrics.value.some((primary) => primary.key === metric.key)))

interface DetailSection {
  key: string
  label: string | null
  isTier: boolean
  metrics: PriceMetric[]
}

const detailSections = computed<DetailSection[]>(() => {
  if (props.model.intervals.length > 0) {
    return props.model.intervals
      .map((interval) => ({
        key: interval.key,
        label: intervalLabel(interval),
        isTier: true,
        metrics: priceMetrics(interval.prices),
      }))
  }
  return detailMetrics.value.length > 0
    ? [{ key: 'extended', label: null, isTier: false, metrics: detailMetrics.value }]
    : []
})
const hasDetails = computed(() => {
  if (props.model.intervals.length > 0) return true
  const isAdditionalMetric = (key: string, value: CatalogPriceValue | null) =>
    hasValue(value) && !primaryMetrics.value.some((primary) => primary.key === key)
  const prices = props.model.prices
  return isAdditionalMetric('cache-write', prices.cacheWrite)
    || isAdditionalMetric('cache-read', prices.cacheRead)
    || isAdditionalMetric('image-input', prices.imageInput)
    || isAdditionalMetric('image-output', prices.imageOutput)
})

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

</script>
