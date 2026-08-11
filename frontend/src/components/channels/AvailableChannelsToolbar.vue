<template>
  <div
    data-testid="available-channels-toolbar"
    class="min-w-0 overflow-hidden rounded-2xl border border-slate-200/90 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800"
  >
    <div
      v-if="channelName"
      data-testid="channel-context"
      class="flex min-w-0 flex-col gap-3 px-4 py-4 sm:px-5 lg:flex-row lg:items-center lg:justify-between"
    >
      <div class="min-w-0 flex-1">
        <h2
          :id="headingId"
          data-testid="channel-detail-name"
          class="min-w-0 text-lg font-semibold leading-6 text-slate-900 [overflow-wrap:anywhere] dark:text-white"
        >
          {{ channelName }}
        </h2>
        <p
          v-if="channelDescription"
          data-testid="channel-description"
          class="mt-1 min-w-0 break-words text-sm leading-5 text-slate-500 dark:text-slate-400"
        >
          {{ channelDescription }}
        </p>
      </div>

      <div class="flex min-w-0 shrink-0 flex-wrap items-center gap-2 lg:justify-end">
        <span
          v-for="item in channelPlatforms"
          :key="item"
          class="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-1 text-xs font-medium text-slate-600 dark:border-dark-500 dark:bg-dark-700 dark:text-slate-300"
        >
          <PlatformIcon :platform="asPlatform(item)" size="sm" />
          {{ item }}
        </span>
        <span class="h-4 w-px bg-slate-200 dark:bg-dark-500" aria-hidden="true" />
        <span class="rounded-lg bg-slate-100 px-2.5 py-1 text-xs font-medium tabular-nums text-slate-600 dark:bg-dark-700 dark:text-slate-300">
          {{ t('availableChannels.catalog.groupsCount', { count: groupCount }) }}
        </span>
        <span class="rounded-lg bg-slate-100 px-2.5 py-1 text-xs font-medium tabular-nums text-slate-600 dark:bg-dark-700 dark:text-slate-300">
          {{ t('availableChannels.catalog.modelsCount', { count: modelCount }) }}
        </span>
      </div>
    </div>

    <div
      data-testid="channel-filter-row"
      class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 border-slate-100 bg-slate-50/55 p-3 dark:border-dark-600 dark:bg-dark-900/35 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,13rem)_auto_auto] lg:px-4"
      :class="channelName ? 'border-t' : ''"
    >
    <div
      data-testid="channel-search-shell"
      class="relative col-span-3 min-w-0 sm:col-span-1"
    >
      <Icon
        name="search"
        size="md"
        class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 dark:text-slate-500"
      />
      <input
        data-testid="channel-search"
        :value="search"
        type="search"
        :placeholder="t('availableChannels.searchPlaceholder')"
        class="input h-11 w-full min-w-0 rounded-xl border-slate-200 bg-white pl-10 pr-3 text-sm shadow-none transition-colors placeholder:text-slate-400 hover:border-slate-300 focus:border-primary-300 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-dark-500"
        @input="emit('update:search', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <Select
      data-testid="platform-filter"
      class="platform-filter min-w-0"
      :model-value="platform"
      :options="platformOptions"
      :searchable="false"
      :aria-label="t('availableChannels.catalog.platformFilter')"
      @update:model-value="updatePlatform"
    />

    <label
      class="group inline-flex min-h-11 shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap rounded-xl border border-slate-200 bg-white px-3 text-xs font-semibold text-slate-600 transition-colors hover:border-primary-200 hover:text-primary-700 focus-within:ring-2 focus-within:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-800 dark:text-slate-300 dark:hover:border-primary-500/40 dark:hover:text-primary-300 sm:text-sm"
    >
      <input
        data-testid="priced-only-filter"
        :checked="pricedOnly"
        type="checkbox"
        class="peer sr-only"
        @change="emit('update:pricedOnly', ($event.target as HTMLInputElement).checked)"
      />
      <span
        aria-hidden="true"
        class="relative h-5 w-9 shrink-0 rounded-full bg-slate-300 transition-colors after:absolute after:left-0.5 after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-white after:shadow-sm after:transition-transform peer-checked:bg-primary-500 peer-checked:after:translate-x-4 motion-reduce:transition-none motion-reduce:after:transition-none dark:bg-dark-500"
      />
      <span aria-hidden="true" class="hidden max-[420px]:inline">¥</span>
      <span class="max-[420px]:sr-only">{{ t('availableChannels.catalog.pricedOnly') }}</span>
    </label>

    <button
      data-testid="channel-refresh"
      type="button"
      class="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-600 shadow-sm transition-colors hover:border-primary-200 hover:bg-primary-50 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-60 motion-reduce:transition-none dark:border-dark-600 dark:bg-dark-800 dark:text-slate-300 dark:hover:border-primary-500/40 dark:hover:bg-primary-500/10 dark:hover:text-primary-300"
      :disabled="loading"
      :title="t('common.refresh', 'Refresh')"
      :aria-label="t('common.refresh', 'Refresh')"
      :aria-busy="loading"
      @click="emit('refresh')"
    >
      <Icon name="refresh" size="md" :class="loading ? 'animate-spin motion-reduce:animate-none' : ''" />
    </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupPlatform } from '@/types'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  search: string
  platform: string
  pricedOnly: boolean
  platforms: string[]
  loading?: boolean
  channelName?: string
  channelDescription?: string
  channelPlatforms?: string[]
  groupCount?: number
  modelCount?: number
  headingId?: string
}>(), {
  loading: false,
  channelName: '',
  channelDescription: '',
  channelPlatforms: () => [],
  groupCount: 0,
  modelCount: 0,
  headingId: undefined,
})

const emit = defineEmits<{
  'update:search': [value: string]
  'update:platform': [value: string]
  'update:pricedOnly': [value: boolean]
  refresh: []
}>()

const { t } = useI18n()

const platformOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('availableChannels.catalog.allPlatforms') },
  ...props.platforms.map(value => ({ value, label: value })),
])

function updatePlatform(value: SelectOption['value']) {
  emit('update:platform', typeof value === 'string' ? value : '')
}

function asPlatform(value: string): GroupPlatform {
  return value as GroupPlatform
}
</script>

<style scoped>
.platform-filter :deep(.select-trigger) {
  @apply h-11 min-w-0 rounded-xl border-slate-200 bg-white px-3 shadow-none dark:border-dark-600 dark:bg-dark-800;
}
</style>
