<script setup lang="ts">
import { computed, getCurrentInstance, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AvailableChannelModelPrice from './AvailableChannelModelPrice.vue'
import type { CatalogModelListEntry, CatalogModelOffering } from './availableChannelCatalog'
import { formatCatalogMoney, summarizeOfferingPrice } from './availableChannelPriceDisplay'

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
    <div class="hidden min-w-0 border-b border-gray-200 px-4 py-2 text-xs font-semibold text-gray-500 dark:border-dark-600 dark:text-gray-400 xl:grid xl:grid-cols-[minmax(0,1.7fr)_minmax(120px,0.8fr)_minmax(130px,1fr)_minmax(84px,0.5fr)] xl:gap-4">
      <span>{{ t('availableChannels.catalog.modelColumn') }}</span><span>{{ t('availableChannels.catalog.officialPrice') }}</span><span>{{ t('availableChannels.catalog.sitePrice') }}</span><span>{{ t('availableChannels.catalog.offeringsColumn') }}</span>
    </div>
    <div class="space-y-3 xl:space-y-2">
      <article v-for="entry in entries" :key="entry.key" data-testid="model-list-row" class="min-w-0 overflow-hidden rounded-2xl border border-gray-200/90 bg-white shadow-sm transition-shadow motion-reduce:transition-none hover:shadow-md dark:border-dark-600 dark:bg-dark-800 xl:grid xl:grid-cols-[minmax(0,1.7fr)_minmax(120px,0.8fr)_minmax(130px,1fr)_minmax(84px,0.5fr)] xl:items-start xl:gap-4 xl:p-3">
        <header class="min-w-0 px-4 pb-2 pt-4 sm:px-5 xl:p-3">
          <component :is="headingLevel === 'h2' ? 'h3' : 'h4'" class="break-words text-sm font-semibold leading-5 text-gray-900 [overflow-wrap:anywhere] dark:text-white">{{ entry.name }}</component>
          <div class="mt-2 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
            <span v-for="platform in entry.platforms" :key="platform" class="max-w-full break-words rounded-md border border-primary-200 bg-primary-50 px-2 py-0.5 text-primary-700 [overflow-wrap:anywhere] dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200">{{ platform }}</span>
            <span>{{ t('availableChannels.catalog.channelsCount', { count: entry.channelCount }) }}</span><span>·</span><span>{{ t('availableChannels.catalog.groupsCount', { count: entry.groupCount }) }}</span>
          </div>
        </header>
        <template v-if="representative(entry)">
          <div data-testid="representative-official" class="px-4 pb-3 sm:px-5 xl:p-3"><span class="text-xs text-gray-500 dark:text-gray-400 xl:hidden">{{ t('availableChannels.catalog.officialPrice') }} · {{ t('availableChannels.catalog.representativePrice') }}</span><p class="break-words font-mono text-sm font-semibold tabular-nums text-gray-800 dark:text-gray-100">{{ price(representative(entry)!, '$') }}</p></div>
          <div data-testid="representative-site" class="px-4 pb-3 sm:px-5 xl:p-3"><span class="text-xs text-primary-600 dark:text-primary-300 xl:hidden">{{ t('availableChannels.catalog.sitePrice') }} · {{ t('availableChannels.catalog.representativePrice') }}</span><p class="break-words font-mono text-sm font-bold tabular-nums text-primary-700 dark:text-primary-200">{{ price(representative(entry)!, '¥') }}</p></div>
        </template>
        <div v-else class="px-4 pb-3 text-sm text-amber-700 sm:px-5 xl:p-3">{{ t('availableChannels.catalog.unpriced') }}</div>
        <button type="button" data-testid="model-offering-toggle" class="min-h-11 w-full border-t border-gray-100 px-4 py-2 text-left text-sm font-medium text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:border-dark-600 dark:text-primary-300 xl:rounded-xl xl:border xl:px-3" :aria-expanded="isExpanded(entry.key)" :aria-controls="detailsId(entry.key)" @click="toggle(entry.key)" @keydown.enter.prevent="toggle(entry.key)" @keydown.space.prevent="toggle(entry.key)">{{ isExpanded(entry.key) ? t('availableChannels.catalog.hideOfferings') : (entry.offerings.length > 1 ? t('availableChannels.catalog.moreOfferings', { count: entry.offerings.length - 1 }) : t('availableChannels.catalog.showOfferings')) }}</button>
        <div v-if="isExpanded(entry.key)" :id="detailsId(entry.key)" data-testid="offering-details" class="col-span-full border-t border-gray-100 bg-gray-50/50 px-3 py-3 dark:border-dark-600 dark:bg-dark-900/20 sm:px-5">
          <div class="space-y-3">
            <section v-for="offering in entry.offerings" :key="offering.key" class="min-w-0 rounded-xl border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800">
              <div class="mb-3 flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1"><strong class="break-words text-sm [overflow-wrap:anywhere]">{{ offering.channelName }}</strong><span class="break-words text-xs text-gray-500 [overflow-wrap:anywhere]">{{ offering.groupName }} · {{ offering.platform }}</span></div>
              <AvailableChannelModelPrice :model="offering.model" />
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
