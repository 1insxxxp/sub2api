<script setup lang="ts">
import { computed, getCurrentInstance, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AvailableChannelModelPrice from './AvailableChannelModelPrice.vue'
import AvailableChannelBrandIcon from './AvailableChannelBrandIcon.vue'
import type { CatalogModelListEntry, CatalogModelOffering, CatalogPriceValue } from './availableChannelCatalog'
import { compareOfferingPrice, formatCatalogMoney, summarizeOfferingPrice } from './availableChannelPriceDisplay'

const props = withDefaults(defineProps<{ entries: CatalogModelListEntry[]; headingLevel?: 'h2' | 'h3' }>(), { headingLevel: 'h2' })
const { t } = useI18n()
const expanded = ref(new Set<string>())
const uid = `available-model-list-${getCurrentInstance()?.uid ?? 0}`

const activeKeys = computed(() => new Set(props.entries.map((entry) => entry.key)))
function isExpanded(key: string) { return expanded.value.has(key) }
function toggle(key: string) {
  const next = new Set(expanded.value)
  next.has(key) ? next.delete(key) : next.add(key)
  expanded.value = next
}
function representative(entry: CatalogModelListEntry): CatalogModelOffering | null {
  return entry.offerings.find((offering) => offering.hasPricing) ?? entry.offerings[0] ?? null
}
function price(offering: CatalogModelOffering, currency: '$' | '¥') {
  const summary = summarizeOfferingPrice(offering)
  if (summary.kind === 'tiered') return t('availableChannels.catalog.tieredSummary')
  if (summary.kind === 'unpriced') return t('availableChannels.catalog.unpriced')
  const value = currency === '$' ? summary.value?.official : summary.value?.site
  const formatted = formatCatalogMoney(value ?? null, summary.scale, currency)
  return formatted === '-' ? t('availableChannels.catalog.unpriced') : `${formatted} ${t(`availableChannels.catalog.${summary.unitKey}`)}`
}
function comparison(offering: CatalogModelOffering) { return compareOfferingPrice(offering) }
function platform(entry: CatalogModelListEntry) { return entry.platforms[0] || 'AI' }
function dimensionValue(value: CatalogPriceValue, currency: '$' | '¥', scale: number) {
  return formatCatalogMoney(currency === '$' ? value.official : value.site, scale, currency)
}
function priceDimensions(offering: CatalogModelOffering) {
  const tokenDimensions = [
    ['input', offering.prices.input], ['output', offering.prices.output],
    ['cacheWrite', offering.prices.cacheWrite], ['cacheRead', offering.prices.cacheRead],
    ['imageInput', offering.prices.imageInput], ['imageOutput', offering.prices.imageOutput],
  ] as const
  const rows: Array<{ label: string; value: CatalogPriceValue; scale: number }> = tokenDimensions
    .filter((entry): entry is [typeof entry[0], CatalogPriceValue] => entry[1] != null && (entry[1].official != null || entry[1].site != null))
    .map(([label, value]) => ({ label, value, scale: 1_000_000 }))
  if (offering.prices.request && (offering.prices.request.official != null || offering.prices.request.site != null)) {
    rows.push({ label: offering.model.billingMode === 'image' ? 'priceImage' : 'priceRequest', value: offering.prices.request, scale: 1 })
  }
  return rows.slice(0, 4)
}
function detailsId(key: string) {
  let hash = 2166136261
  for (let index = 0; index < key.length; index += 1) {
    hash ^= key.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  // Keep the exact encoded key alongside the readable hash. The hash keeps IDs
  // compact for normal keys, while the encoded key makes collisions impossible
  // even for adversarial model names that share an FNV-1a digest.
  return `${uid}-details-${(hash >>> 0).toString(36)}-${encodeURIComponent(key)}`
}

// Keep only still-present stable keys after parent list refresh/reorder.
watch(activeKeys, (keys) => { expanded.value = new Set([...expanded.value].filter((key) => keys.has(key))) }, { immediate: true })
</script>

<template>
  <section v-if="entries.length > 0" data-testid="available-model-list" class="min-w-0">
    <component :is="headingLevel" class="sr-only">{{ t('availableChannels.catalog.modelColumn') }}</component>
    <div data-testid="model-card-grid" class="grid min-w-0 grid-cols-1 gap-4 md:grid-cols-2 2xl:grid-cols-3">
      <article v-for="entry in entries" :key="entry.key" data-testid="model-card" class="group relative min-w-0 overflow-hidden rounded-2xl border border-gray-200/90 bg-white shadow-sm transition duration-200 motion-reduce:transition-none hover:-translate-y-0.5 hover:border-primary-200 hover:shadow-lg dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-500/40">
        <div class="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-primary-500 via-cyan-400 to-emerald-400 opacity-80" />
        <header class="flex min-w-0 items-start gap-3 px-4 pb-3 pt-5 sm:px-5">
          <AvailableChannelBrandIcon :platform="platform(entry)" />
          <div class="min-w-0 flex-1">
            <component :is="headingLevel === 'h2' ? 'h3' : 'h4'" class="break-words text-[15px] font-bold leading-5 text-gray-950 [overflow-wrap:anywhere] dark:text-white">{{ entry.name }}</component>
            <div class="mt-1.5 flex min-w-0 flex-wrap items-center gap-1.5">
              <span v-for="platformName in entry.platforms" :key="platformName" class="max-w-full break-words rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 [overflow-wrap:anywhere] dark:bg-dark-700 dark:text-gray-300">{{ platformName }}</span>
            </div>
          </div>
        </header>

        <template v-if="representative(entry)">
          <div class="mx-4 rounded-2xl border border-emerald-100 bg-gradient-to-br from-emerald-50 via-white to-cyan-50 p-3.5 dark:border-emerald-500/20 dark:from-emerald-500/10 dark:via-dark-800 dark:to-cyan-500/10 sm:mx-5">
            <div class="flex flex-wrap items-start justify-between gap-2">
              <div data-testid="representative-site">
                <span class="text-[11px] font-semibold uppercase tracking-wide text-emerald-700 dark:text-emerald-300">{{ t('availableChannels.catalog.sitePrice') }}</span>
                <p data-testid="primary-site-price" class="mt-0.5 break-words font-mono text-xl font-black leading-tight tabular-nums text-emerald-700 [overflow-wrap:anywhere] dark:text-emerald-300">{{ price(representative(entry)!, '¥') }}</p>
              </div>
              <span v-if="comparison(representative(entry)!).savingsPercent != null" data-testid="savings-badge" class="rounded-full bg-emerald-600 px-2.5 py-1 text-xs font-bold text-white shadow-sm">{{ t('availableChannels.catalog.savePercent', { count: comparison(representative(entry)!).savingsPercent! }) }}</span>
            </div>
            <div data-testid="representative-official" class="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('availableChannels.catalog.officialPrice') }}</span>
              <span data-testid="primary-official-price" class="font-mono tabular-nums" :class="comparison(representative(entry)!).savingsPercent != null ? 'line-through decoration-gray-400' : 'font-semibold text-gray-700 dark:text-gray-300'">{{ price(representative(entry)!, '$') }}</span>
              <span class="rounded-md bg-white/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-500 shadow-sm dark:bg-dark-700">{{ t('availableChannels.catalog.representativePrice') }}</span>
            </div>
            <div v-if="priceDimensions(representative(entry)!).length" class="mt-3 space-y-1.5 border-t border-emerald-100 pt-3 text-xs dark:border-emerald-500/20">
              <div class="grid grid-cols-[minmax(0,1fr)_auto_auto] gap-2 text-[10px] font-medium text-gray-400"><span /><span>{{ t('availableChannels.catalog.officialPrice') }}</span><span>{{ t('availableChannels.catalog.sitePrice') }}</span></div>
              <div v-for="dimension in priceDimensions(representative(entry)!)" :key="dimension.label" data-testid="price-dimension-row" class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2">
                <span class="truncate text-gray-600 dark:text-gray-300">{{ t(`availableChannels.catalog.${dimension.label}`) }}</span>
                <span class="font-mono tabular-nums text-gray-400">{{ dimensionValue(dimension.value, '$', dimension.scale) }}</span>
                <span class="font-mono font-bold tabular-nums text-emerald-700 dark:text-emerald-300">{{ dimensionValue(dimension.value, '¥', dimension.scale) }}</span>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="mx-4 rounded-2xl border border-dashed border-amber-200 bg-amber-50 px-4 py-6 text-center text-sm font-semibold text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300 sm:mx-5">{{ t('availableChannels.catalog.unpriced') }}</div>

        <div class="mt-4 flex flex-wrap items-center gap-x-2 gap-y-1 px-4 text-xs text-gray-500 dark:text-gray-400 sm:px-5">
          <span>{{ t('availableChannels.catalog.channelsCount', { count: entry.channelCount }) }}</span><span aria-hidden="true">·</span><span>{{ t('availableChannels.catalog.groupsCount', { count: entry.groupCount }) }}</span><span aria-hidden="true">·</span><span>{{ entry.offerings.length }} {{ t('availableChannels.catalog.offeringsColumn') }}</span>
        </div>
        <button type="button" data-testid="model-offering-toggle" class="mt-3 min-h-11 w-full border-t border-gray-100 px-4 py-2.5 text-left text-sm font-semibold text-primary-600 transition-colors hover:bg-primary-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 motion-reduce:transition-none dark:border-dark-600 dark:text-primary-300 dark:hover:bg-primary-500/10 sm:px-5" :aria-expanded="isExpanded(entry.key)" :aria-controls="detailsId(entry.key)" @click="toggle(entry.key)" @keydown.enter.prevent="toggle(entry.key)" @keydown.space.prevent="toggle(entry.key)">{{ isExpanded(entry.key) ? t('availableChannels.catalog.hideOfferings') : (entry.offerings.length > 1 ? t('availableChannels.catalog.moreOfferings', { count: entry.offerings.length - 1 }) : t('availableChannels.catalog.showOfferings')) }}</button>
        <div v-if="isExpanded(entry.key)" :id="detailsId(entry.key)" data-testid="offering-details" class="border-t border-gray-100 bg-gray-50/70 px-3 py-3 dark:border-dark-600 dark:bg-dark-900/20 sm:px-5">
          <div class="space-y-3">
            <section v-for="offering in entry.offerings" :key="offering.key" class="min-w-0 rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800">
              <div class="mb-3 flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1"><strong class="break-words text-sm [overflow-wrap:anywhere]">{{ offering.channelName }}</strong><span class="break-words text-xs text-gray-500 [overflow-wrap:anywhere]">{{ offering.groupName }} · {{ offering.platform }}</span></div>
              <AvailableChannelModelPrice :model="offering.model" details-expanded />
            </section>
          </div>
        </div>
      </article>
    </div>
  </section>
  <div v-else data-testid="model-list-empty" class="rounded-2xl border border-dashed border-gray-300 bg-white px-6 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400">
    {{ t('availableChannels.catalog.noModels') }}
  </div>
</template>
