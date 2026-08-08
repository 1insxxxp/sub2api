<template>
  <section
    data-testid="channel-group"
    class="min-w-0 overflow-hidden rounded-2xl border border-l-4 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800"
    :class="accentClass"
  >
    <button
      type="button"
      data-testid="group-toggle"
      class="flex min-h-11 w-full min-w-0 items-start justify-between gap-3 px-4 py-4 text-left transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:hover:bg-dark-700/60 sm:px-5 lg:hidden"
      :aria-expanded="expanded"
      :aria-controls="bodyId"
      @click="expanded = !expanded"
    >
      <span class="min-w-0 flex-1">
        <GroupBadge
          :name="group.name"
          :platform="asPlatform(group.platform)"
          :subscription-type="asSubscriptionType(group.subscriptionType)"
          :rate-multiplier="group.defaultRate"
          :user-rate-multiplier="group.userRate"
          :always-show-rate="true"
          :wrap-name="true"
        />

        <span class="mt-2 flex flex-wrap items-center gap-1.5">
          <span
            class="rounded-md border px-2 py-0.5 text-xs font-medium"
            :class="group.isExclusive
              ? 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-300'
              : 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-300'"
          >
            {{ group.isExclusive ? t(`${catalogKey}.exclusiveGroup`) : t(`${catalogKey}.publicGroup`) }}
          </span>
          <span
            v-if="group.subscriptionType === 'subscription'"
            class="rounded-md border border-sky-200 bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-300"
          >
            {{ t(`${catalogKey}.subscriptionGroup`) }}
          </span>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t(`${catalogKey}.modelsCount`, { count: group.modelCount }) }}
          </span>
          <span
            v-if="peakWindow"
            class="rounded-md border border-amber-200 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300"
          >
            {{ peakWindow }}
          </span>
        </span>
      </span>

      <svg
        class="mt-1 h-5 w-5 shrink-0 text-gray-400 transition-transform lg:hidden"
        :class="expanded ? 'rotate-180' : ''"
        viewBox="0 0 20 20"
        fill="none"
        aria-hidden="true"
      >
        <path d="m5 7.5 5 5 5-5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>

    <header
      data-testid="group-desktop-header"
      class="hidden min-w-0 items-start justify-between gap-3 px-5 py-4 lg:flex"
    >
      <span class="min-w-0 flex-1">
        <GroupBadge
          :name="group.name"
          :platform="asPlatform(group.platform)"
          :subscription-type="asSubscriptionType(group.subscriptionType)"
          :rate-multiplier="group.defaultRate"
          :user-rate-multiplier="group.userRate"
          :always-show-rate="true"
          :wrap-name="true"
        />

        <span class="mt-2 flex flex-wrap items-center gap-1.5">
          <span
            class="rounded-md border px-2 py-0.5 text-xs font-medium"
            :class="group.isExclusive
              ? 'border-violet-200 bg-violet-50 text-violet-700 dark:border-violet-500/30 dark:bg-violet-500/10 dark:text-violet-300'
              : 'border-gray-200 bg-gray-50 text-gray-600 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-300'"
          >
            {{ group.isExclusive ? t(`${catalogKey}.exclusiveGroup`) : t(`${catalogKey}.publicGroup`) }}
          </span>
          <span
            v-if="group.subscriptionType === 'subscription'"
            class="rounded-md border border-sky-200 bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-300"
          >
            {{ t(`${catalogKey}.subscriptionGroup`) }}
          </span>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t(`${catalogKey}.modelsCount`, { count: group.modelCount }) }}
          </span>
          <span
            v-if="peakWindow"
            class="rounded-md border border-amber-200 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300"
          >
            {{ peakWindow }}
          </span>
        </span>
      </span>
    </header>

    <div
      :id="bodyId"
      data-testid="group-body"
      class="gap-4 border-t border-gray-100 bg-gray-50/50 p-4 dark:border-dark-600 dark:bg-dark-900/20 sm:p-5 lg:grid 2xl:grid-cols-2"
      :class="expanded ? 'grid' : 'hidden'"
    >
      <div
        data-testid="desktop-price-columns"
        class="col-span-full hidden min-w-0 grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(88px,0.5fr)_minmax(72px,0.4fr)] gap-4 border-b border-gray-200 px-3 pb-2 text-xs font-semibold text-gray-500 dark:border-dark-600 dark:text-gray-400 lg:grid"
        aria-hidden="true"
      >
        <span>{{ t(`${catalogKey}.modelColumn`) }}</span>
        <span>{{ t(`${catalogKey}.officialPrice`) }}</span>
        <span class="text-primary-700 dark:text-primary-200">{{ t(`${catalogKey}.sitePrice`) }}</span>
        <span>{{ t(`${catalogKey}.effectiveRate`) }}</span>
        <span>{{ t(`${catalogKey}.detailsColumn`) }}</span>
      </div>
      <AvailableChannelModelPrice
        v-for="model in group.models"
        :key="model.key"
        :model="model"
      />
      <div
        v-if="group.models.length === 0"
        data-testid="group-empty"
        class="col-span-full rounded-xl border border-dashed border-gray-300 bg-white px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-500 dark:bg-dark-800 dark:text-gray-400"
      >
        {{ t(`${catalogKey}.noModelsInGroup`) }}
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import GroupBadge from '@/components/common/GroupBadge.vue'
import AvailableChannelModelPrice from './AvailableChannelModelPrice.vue'
import type { CatalogGroupEntry } from './availableChannelCatalog'

const catalogKey = 'availableChannels.catalog'
const props = withDefaults(defineProps<{
  group: CatalogGroupEntry
  defaultExpanded?: boolean
}>(), {
  defaultExpanded: false,
})

const { t } = useI18n()
const appStore = useAppStore()
const expanded = ref(props.defaultExpanded)
const instanceUid = getCurrentInstance()?.uid ?? 0

function safeId(value: string): string {
  const normalized = value
    .toLocaleLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return normalized || 'group'
}

function asPlatform(value: string): GroupPlatform {
  return value as GroupPlatform
}

function asSubscriptionType(value: string): SubscriptionType {
  return value as SubscriptionType
}

const bodyId = computed(() => `available-channel-group-${safeId(props.group.key)}-${instanceUid}`)

const peakWindow = computed(() => {
  const peak = props.group.peak
  if (!peak) return ''
  return formatPeakRateWindow(
    {
      peak_rate_enabled: true,
      peak_start: peak.start,
      peak_end: peak.end,
      peak_rate_multiplier: peak.factor,
    },
    serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset),
  )
})

const accentClass = computed(() => {
  switch (props.group.platform) {
    case 'anthropic':
      return 'border-l-orange-400'
    case 'gemini':
      return 'border-l-sky-400'
    case 'antigravity':
      return 'border-l-violet-400'
    case 'grok':
      return 'border-l-zinc-500'
    case 'composite':
      return 'border-l-cyan-400'
    default:
      return 'border-l-emerald-400'
  }
})
</script>
