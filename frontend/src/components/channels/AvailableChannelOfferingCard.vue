<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CatalogModelOffering, CatalogPriceCollection, CatalogPriceValue, CatalogPricingInterval } from './availableChannelCatalog'
import { formatCatalogMoney } from './availableChannelPriceDisplay'

const props = defineProps<{ offering: CatalogModelOffering }>()
const { t } = useI18n()
const catalogKey = 'availableChannels.catalog'

interface Metric { key: string; label: string; value: CatalogPriceValue; scale: number; unit: string }
function hasValue(value: CatalogPriceValue | null): value is CatalogPriceValue {
  return value != null && (value.official != null || value.site != null || value.peakSite != null)
}
function metrics(prices: CatalogPriceCollection): Metric[] {
  const token = [
    ['input', prices.input], ['output', prices.output], ['cacheWrite', prices.cacheWrite],
    ['cacheRead', prices.cacheRead], ['imageInput', prices.imageInput], ['imageOutput', prices.imageOutput],
  ] as const
  const result: Metric[] = token.filter((item): item is [typeof item[0], CatalogPriceValue] => hasValue(item[1]))
    .map(([key, value]) => ({ key, label: t(`${catalogKey}.${key}`), value, scale: 1_000_000, unit: t(`${catalogKey}.perMillion`) }))
  if (hasValue(prices.request)) {
    const image = props.offering.model.billingMode === 'image'
    result.push({ key: 'request', label: t(`${catalogKey}.${image ? 'priceImage' : 'priceRequest'}`), value: prices.request, scale: 1, unit: t(`${catalogKey}.${image ? 'perImage' : 'perRequest'}`) })
  }
  return result
}
function money(value: number | null, scale: number, symbol: '$' | '¥') { return formatCatalogMoney(value, scale, symbol) }
function rate(value: number) { return Number.isFinite(value) ? Number(value.toPrecision(6)).toString() : '-' }
function tierLabel(interval: CatalogPricingInterval) {
  if (interval.tierLabel) return interval.tierLabel
  const min = new Intl.NumberFormat('en-US').format(interval.minTokens)
  if (interval.maxTokens == null) return t(`${catalogKey}.rangeFrom`, { min })
  return t(`${catalogKey}.rangeBetween`, { min, max: new Intl.NumberFormat('en-US').format(interval.maxTokens) })
}
const directMetrics = computed(() => metrics(props.offering.prices))
const tiers = computed(() => props.offering.intervals.map(interval => ({ ...interval, label: tierLabel(interval), metrics: metrics(interval.prices) })))
const hasAnyPrice = computed(() => directMetrics.value.length > 0 || tiers.value.some(tier => tier.metrics.length > 0))
</script>

<template>
  <article data-testid="flat-offering-card" class="min-w-0 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-500 dark:bg-dark-800 sm:p-5">
    <header class="flex min-w-0 flex-wrap items-start justify-between gap-3 border-b border-gray-100 pb-3 dark:border-dark-600">
      <div class="min-w-0 flex-1">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <strong class="break-words text-sm font-bold text-gray-900 [overflow-wrap:anywhere] dark:text-white">{{ offering.channelName }}</strong>
          <span class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ offering.platform }}</span>
        </div>
        <p class="mt-1 break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-gray-400">{{ offering.groupName }}</p>
      </div>
      <div class="shrink-0 rounded-xl bg-primary-50 px-3 py-1.5 text-right dark:bg-primary-500/10">
        <span class="block text-[10px] text-primary-600 dark:text-primary-300">{{ t(`${catalogKey}.effectiveRate`) }}</span>
        <strong class="font-mono text-sm tabular-nums text-primary-700 dark:text-primary-200">{{ rate(offering.model.normalRate) }}×</strong>
      </div>
    </header>

    <div v-if="directMetrics.length" class="mt-3 grid min-w-0 grid-cols-2 gap-2 lg:grid-cols-4">
      <section v-for="metric in directMetrics" :key="metric.key" data-testid="offering-price-cell" class="min-w-0 rounded-xl bg-gray-50 p-3 dark:bg-dark-700/60">
        <h5 class="truncate text-xs font-semibold text-gray-700 dark:text-gray-200">{{ metric.label }}</h5>
        <div class="mt-2 space-y-1.5">
          <p class="min-w-0 text-[10px] text-gray-400">{{ t(`${catalogKey}.officialPrice`) }} <span class="break-all font-mono text-xs font-semibold tabular-nums text-gray-600 dark:text-gray-300">{{ money(metric.value.official, metric.scale, '$') }}</span></p>
          <p class="min-w-0 text-[10px] text-primary-500">{{ t(`${catalogKey}.sitePrice`) }} <span class="break-all font-mono text-xs font-bold tabular-nums text-primary-700 dark:text-primary-200">{{ money(metric.value.site, metric.scale, '¥') }}</span></p>
          <p class="break-words text-[10px] text-gray-400 [overflow-wrap:anywhere]">{{ metric.unit }}</p>
        </div>
      </section>
    </div>

    <div v-for="tier in tiers" :key="tier.key" data-testid="pricing-tier-flat" class="mt-3 rounded-xl bg-amber-50/70 p-3 dark:bg-amber-500/10">
      <h5 class="text-xs font-bold text-amber-800 dark:text-amber-300">{{ tier.label }}</h5>
      <div v-if="tier.metrics.length" class="mt-2 grid min-w-0 grid-cols-2 gap-2 lg:grid-cols-4">
        <section v-for="metric in tier.metrics" :key="metric.key" data-testid="offering-price-cell" class="min-w-0 rounded-lg bg-white/80 p-2.5 dark:bg-dark-800/70">
          <p class="text-xs font-semibold text-gray-700 dark:text-gray-200">{{ metric.label }}</p>
          <p class="mt-1 break-all font-mono text-[11px] text-gray-500">{{ money(metric.value.official, metric.scale, '$') }} → <strong class="text-primary-700 dark:text-primary-200">{{ money(metric.value.site, metric.scale, '¥') }}</strong></p>
          <p class="mt-1 text-[10px] text-gray-400">{{ metric.unit }}</p>
        </section>
      </div>
      <p v-else class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t(`${catalogKey}.unpriced`) }}</p>
    </div>

    <div v-if="!hasAnyPrice" data-testid="offering-unpriced" class="mt-3 rounded-xl border border-dashed border-amber-300 bg-amber-50 px-3 py-3 text-center text-sm font-medium text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300">{{ t(`${catalogKey}.unpriced`) }}</div>
  </article>
</template>
