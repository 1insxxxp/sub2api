<template>
  <div
    data-test="group-option-layout"
    class="group-option relative min-w-0 flex-1 overflow-hidden px-3 py-3 text-left"
    :class="{ 'group-option-selected': selected }"
    :style="{ '--group-accent': tag ? groupTagColor(tag, tagColor) : '#94A3B8' }"
  >
    <div class="flex min-w-0 items-start gap-2.5">
      <span class="group-option-platform flex h-6 w-6 shrink-0 items-center justify-center rounded-md" :class="platformBadgeLightClass(platform)" :title="platformLabel(platform)">
        <PlatformIcon :platform="platform" size="sm" />
      </span>
      <span data-test="group-option-name" class="min-w-0 flex-1 whitespace-normal text-[13px] font-semibold leading-5 text-gray-900 [overflow-wrap:anywhere] dark:text-gray-100" :title="name">{{ name }}</span>
      <GroupTagBadge :tag="tag" :color="tagColor" variant="soft" class="group-option-tag" />
    </div>
    <p
      v-if="description"
      data-test="group-option-description"
      class="mt-1.5 whitespace-pre-line pl-[34px] text-xs leading-5 text-gray-500 [overflow-wrap:anywhere] dark:text-gray-400 line-clamp-3"
      :title="description"
    >{{ description }}</p>
    <div class="group-option-meta mt-2 ml-[34px] flex min-h-5 flex-wrap items-center gap-x-3 gap-y-1.5 pt-2">
      <span v-if="subscriptionType === 'subscription'" class="inline-flex items-center gap-1 text-xs text-cyan-700 dark:text-cyan-400"><Icon name="badge" size="xs" />{{ t('groups.subscription') }}</span>
      <span v-if="rateMultiplier !== undefined" data-test="group-option-rate" class="inline-flex items-baseline gap-1.5 whitespace-nowrap text-xs">
        <span class="text-gray-400 dark:text-gray-500">{{ t('admin.groups.rateLabel') }}</span>
        <span class="font-semibold tabular-nums text-gray-800 dark:text-gray-200">
          <template v-if="hasCustomRate">
            <span class="mr-1.5 font-normal text-gray-400 line-through">{{ formatVisibleRateMultiplier(rateMultiplier) }}x</span>
            <span class="text-emerald-700 dark:text-emerald-400">{{ formatVisibleRateMultiplier(userRateMultiplier) }}x</span>
          </template>
          <template v-else>{{ formatVisibleRateMultiplier(rateMultiplier) }}x</template>
        </span>
      </span>
      <span v-if="hasPeakRate" class="min-w-0 text-xs leading-5 text-amber-700 [overflow-wrap:anywhere] dark:text-amber-400" :title="peakRateTitle">{{ peakRateText }}</span>
      <span v-if="showCheckmark" class="ml-auto flex h-5 w-5 shrink-0 items-center justify-center rounded-full" :class="selected ? 'bg-primary-500 text-white shadow-sm shadow-primary-500/20 ring-2 ring-white dark:ring-white/10' : 'invisible'" aria-hidden="true">
        <Icon name="check" size="xs" :stroke-width="2.5" />
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformIcon from './PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupTagBadge from './GroupTagBadge.vue'
import type { SubscriptionType, GroupPlatform, GroupTag } from '@/types'
import { formatVisibleRateMultiplier } from '@/utils/formatters'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import { groupTagColor } from '@/utils/groupTag'

const { t } = useI18n()

interface Props {
  name: string
  tag?: GroupTag
  tagColor?: string
  platform: GroupPlatform
  subscriptionType?: SubscriptionType
  rateMultiplier?: number
  userRateMultiplier?: number | null
  peakRateEnabled?: boolean
  peakStart?: string
  peakEnd?: string
  peakRateMultiplier?: number
  description?: string | null
  selected?: boolean
  showCheckmark?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  subscriptionType: 'standard',
  selected: false,
  showCheckmark: true,
  userRateMultiplier: null,
  peakRateEnabled: false
})

// Whether user has a custom rate different from default
const hasCustomRate = computed(() => {
  return (
    props.userRateMultiplier !== null &&
    props.userRateMultiplier !== undefined &&
    props.rateMultiplier !== undefined &&
    props.userRateMultiplier !== props.rateMultiplier
  )
})

const appStore = useAppStore()

const hasPeakRate = computed(() => {
  return Boolean(props.peakRateEnabled && props.peakStart && props.peakEnd)
})

const peakRateText = computed(() => {
  return formatPeakRateWindow(
    {
      peak_rate_enabled: props.peakRateEnabled,
      peak_start: props.peakStart,
      peak_end: props.peakEnd,
      peak_rate_multiplier: props.peakRateMultiplier
    },
    serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
  )
})

const peakRateTitle = computed(() => {
  return t('common.peakRateTooltip', { window: peakRateText.value })
})

</script>

<style scoped>
.group-option {
  width: 100%;
  border: 1px solid rgb(148 163 184 / 24%);
  border-radius: 0.5rem;
  background-color: rgb(255 255 255 / 96%);
  background-image: linear-gradient(125deg, rgb(255 255 255 / 82%) 15%, transparent 65%, color-mix(in srgb, var(--group-accent) 5%, transparent));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 4px rgb(15 23 42 / 4%);
  transition: border-color 180ms ease, background-color 180ms ease, box-shadow 180ms ease;
}

.group-option-platform {
  border: 1px solid rgb(255 255 255 / 80%);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 75%), 0 1px 3px rgb(15 23 42 / 8%);
}

.group-option-meta { border-top: 1px solid rgb(148 163 184 / 12%); }

.group-option-selected {
  @apply border-primary-300 bg-primary-50/60;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 95%), 0 0 0 1px rgb(59 130 246 / 6%), 0 2px 5px rgb(59 130 246 / 8%);
}

.dark .group-option {
  border-color: rgb(255 255 255 / 11%);
  background-color: #27292e;
  background-image: linear-gradient(125deg, rgb(255 255 255 / 4%), transparent 65%, color-mix(in srgb, var(--group-accent) 5%, transparent));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%), 0 2px 5px rgb(0 0 0 / 12%);
}

.dark .group-option-platform {
  border-color: rgb(255 255 255 / 10%);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 8%), 0 1px 3px rgb(0 0 0 / 12%);
}

.dark .group-option-meta { border-color: rgb(255 255 255 / 7%); }

.dark .group-option-selected {
  @apply border-primary-400/60;
  background-color: #262e3a;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 8%), 0 0 0 1px rgb(96 165 250 / 8%), 0 2px 5px rgb(0 0 0 / 14%);
}

@media (hover: hover) {
  .group-option:hover {
    border-color: color-mix(in srgb, var(--group-accent) 32%, #d1d5db);
    box-shadow: inset 0 1px 0 #fff, 0 3px 8px rgb(15 23 42 / 7%);
  }
  .group-option-selected:hover { @apply border-primary-400; }
  .dark .group-option:hover {
    border-color: color-mix(in srgb, var(--group-accent) 32%, #52525b);
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 8%), 0 3px 8px rgb(0 0 0 / 16%);
  }
  .dark .group-option-selected:hover { @apply border-primary-400; }
}

.select-option-focused .group-option,
:focus-visible > .group-option {
  @apply border-primary-400;
  outline: 2px solid rgb(59 130 246 / 28%);
  outline-offset: -2px;
}

.group-option-tag {
  max-width: min(36%, 8rem);
  margin: -0.75rem -0.75rem 0 0;
  padding: 0.25rem 0.625rem;
  border-radius: 0 0 0 0.5rem;
}

@media (prefers-reduced-motion: reduce) {
  .group-option { transition: none; }
}
</style>
