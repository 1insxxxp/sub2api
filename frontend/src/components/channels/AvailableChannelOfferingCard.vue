<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CatalogModelOffering, CatalogPriceCollection, CatalogPriceValue, CatalogPricingInterval } from './availableChannelCatalog'
import { formatCatalogMoney, formatCatalogMoneyRange, formatCatalogValueRange } from './availableChannelPriceDisplay'
import { formatAvailableChannelRate } from './availableChannelRateDisplay'
import AvailableChannelPlatformBadge from './AvailableChannelPlatformBadge.vue'

const props = defineProps<{ offering: CatalogModelOffering }>()
const { t } = useI18n()
const catalogKey = 'availableChannels.catalog'

interface Metric { key: string; label: string; value: CatalogPriceValue; scale: number; unit: string }
function hasValue(value: CatalogPriceValue | null): value is CatalogPriceValue {
  return value != null && (value.official != null
    || value.site != null
    || value.siteMax != null
    || value.peakSite != null
    || value.peakSiteMax != null)
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
function officialMoney(value: number | null, scale: number) { return formatCatalogMoney(value, scale, '$') }
function siteMoney(value: CatalogPriceValue, scale: number) { return formatCatalogMoneyRange(value.site, value.siteMax, scale, '¥') }
function peakMoney(value: CatalogPriceValue, scale: number) { return formatCatalogMoneyRange(value.peakSite, value.peakSiteMax, scale, '¥') }
function hasPeak(value: CatalogPriceValue) { return value.peakSite != null || value.peakSiteMax != null }
function tierLabel(interval: CatalogPricingInterval) {
  if (interval.tierLabel) return interval.tierLabel
  const min = new Intl.NumberFormat('en-US').format(interval.minTokens)
  if (interval.maxTokens == null) return t(`${catalogKey}.rangeFrom`, { min })
  return t(`${catalogKey}.rangeBetween`, { min, max: new Intl.NumberFormat('en-US').format(interval.maxTokens) })
}
const directMetrics = computed(() => metrics(props.offering.prices))
const tiers = computed(() => props.offering.intervals.map(interval => ({ ...interval, label: tierLabel(interval), metrics: metrics(interval.prices) })))
const hasAnyPrice = computed(() => directMetrics.value.length > 0 || tiers.value.some(tier => tier.metrics.length > 0))
const effectiveRateRange = computed(() => {
  const minimum = props.offering.model.effectiveRate
  const maximum = props.offering.model.effectiveRateMax ?? minimum
  return formatCatalogValueRange(
    minimum,
    maximum,
    value => `${formatAvailableChannelRate(value)}×`,
  )
})
</script>

<template>
  <article data-testid="flat-offering-card" class="offering-row min-w-0 overflow-hidden rounded-xl border border-slate-200 bg-white dark:border-dark-600 dark:bg-dark-800">
    <header class="grid min-w-0 gap-3 px-3 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
      <div class="min-w-0 flex-1">
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <h4 class="break-words text-sm font-bold text-gray-900 [overflow-wrap:anywhere] dark:text-white">{{ offering.channelName }}</h4>
          <AvailableChannelPlatformBadge :platform="offering.platform" />
        </div>
        <p class="mt-1 break-words text-xs text-gray-500 [overflow-wrap:anywhere] dark:text-gray-400">{{ offering.groupName }}</p>
      </div>
      <div class="grid min-w-0 grid-cols-2 divide-x divide-slate-200 overflow-hidden rounded-lg border border-slate-200 bg-slate-50 text-right dark:divide-dark-500 dark:border-dark-500 dark:bg-dark-700/50">
        <div class="px-2.5 py-1.5">
          <span class="block text-[10px] text-slate-500 dark:text-slate-400">{{ t(`${catalogKey}.groupRate`) }}</span>
          <strong class="font-mono text-sm tabular-nums text-slate-700 dark:text-slate-200">{{ formatAvailableChannelRate(offering.model.normalRate) }}×</strong>
        </div>
        <div class="px-2.5 py-1.5">
          <span class="block text-[10px] text-primary-600 dark:text-primary-300">{{ t(`${catalogKey}.effectiveRate`) }}</span>
          <strong class="font-mono text-sm tabular-nums text-primary-700 dark:text-primary-200">{{ effectiveRateRange }}</strong>
        </div>
      </div>
    </header>

    <div v-if="directMetrics.length" data-testid="offering-price-table" class="divide-y divide-slate-100 border-t border-slate-200 px-3 pb-1 dark:divide-dark-600 dark:border-dark-600">
      <div data-testid="offering-price-heading" class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] gap-2 py-2 text-[10px] font-medium text-slate-400">
        <span />
        <span>{{ t(`${catalogKey}.officialPrice`) }}</span>
        <span>{{ t(`${catalogKey}.sitePrice`) }}</span>
      </div>
      <section v-for="metric in directMetrics" :key="metric.key" data-testid="offering-price-cell" class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-start gap-2 py-2">
        <div class="min-w-0">
          <h5 class="truncate text-xs font-semibold text-slate-700 dark:text-slate-200">{{ metric.label }}</h5>
          <p class="mt-0.5 break-words text-[10px] text-slate-400 [overflow-wrap:anywhere]">{{ metric.unit }}</p>
        </div>
        <span class="break-words font-mono text-xs font-semibold tabular-nums text-slate-500 [overflow-wrap:anywhere] dark:text-slate-300">{{ officialMoney(metric.value.official, metric.scale) }}</span>
        <div class="min-w-0 text-right">
          <strong class="break-words font-mono text-xs tabular-nums text-primary-700 [overflow-wrap:anywhere] dark:text-primary-200">{{ siteMoney(metric.value, metric.scale) }}</strong>
          <p v-if="hasPeak(metric.value)" class="mt-0.5 break-words text-[10px] text-amber-600 [overflow-wrap:anywhere] dark:text-amber-300">{{ t(`${catalogKey}.peakPrice`) }} {{ peakMoney(metric.value, metric.scale) }}</p>
        </div>
      </section>
    </div>

    <div v-for="tier in tiers" :key="tier.key" data-testid="pricing-tier-flat" class="border-t border-amber-200/70 bg-amber-50/60 px-3 py-3 dark:border-amber-500/20 dark:bg-amber-500/10">
      <h5 class="text-xs font-bold text-amber-800 dark:text-amber-300">{{ tier.label }}</h5>
      <div v-if="tier.metrics.length" class="mt-2 divide-y divide-amber-200/60 dark:divide-amber-500/20">
        <section v-for="metric in tier.metrics" :key="metric.key" data-testid="offering-price-cell" class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-start gap-2 py-2">
          <div class="min-w-0">
            <p class="truncate text-xs font-semibold text-slate-700 dark:text-slate-200">{{ metric.label }}</p>
            <p class="mt-0.5 text-[10px] text-slate-400">{{ metric.unit }}</p>
          </div>
          <span class="font-mono text-[11px] tabular-nums text-slate-500">{{ officialMoney(metric.value.official, metric.scale) }}</span>
          <div class="min-w-0 text-right">
            <strong class="break-words font-mono text-[11px] tabular-nums text-primary-700 [overflow-wrap:anywhere] dark:text-primary-200">{{ siteMoney(metric.value, metric.scale) }}</strong>
            <p v-if="hasPeak(metric.value)" class="mt-0.5 break-words text-[10px] text-amber-700 [overflow-wrap:anywhere] dark:text-amber-300">{{ t(`${catalogKey}.peakPrice`) }} {{ peakMoney(metric.value, metric.scale) }}</p>
          </div>
        </section>
      </div>
      <p v-else class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t(`${catalogKey}.unpriced`) }}</p>
    </div>

    <div v-if="!hasAnyPrice" data-testid="offering-unpriced" class="border-t border-dashed border-amber-300 bg-amber-50 px-3 py-3 text-center text-sm font-medium text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300">{{ t(`${catalogKey}.unpriced`) }}</div>
  </article>
</template>
