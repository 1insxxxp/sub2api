<template>
  <div
    data-testid="available-channels-toolbar"
    class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 rounded-2xl border border-slate-200/90 bg-white p-3 shadow-sm dark:border-dark-600 dark:bg-dark-800 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,13rem)_auto_auto] lg:p-4"
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
        class="input h-11 w-full min-w-0 rounded-xl border-slate-200 bg-slate-50/70 pl-10 pr-3 text-sm shadow-none transition-colors placeholder:text-slate-400 hover:border-slate-300 focus:bg-white dark:border-dark-600 dark:bg-dark-900/70 dark:hover:border-dark-500 dark:focus:bg-dark-900"
        @input="emit('update:search', ($event.target as HTMLInputElement).value)"
      />
    </div>

    <select
      data-testid="platform-filter"
      :value="platform"
      class="input h-11 w-full min-w-0 rounded-xl border-slate-200 bg-slate-50/70 px-3 text-sm shadow-none transition-colors hover:border-slate-300 focus:bg-white dark:border-dark-600 dark:bg-dark-900/70 dark:hover:border-dark-500 dark:focus:bg-dark-900"
      :aria-label="t('availableChannels.catalog.platformFilter')"
      @change="emit('update:platform', ($event.target as HTMLSelectElement).value)"
    >
      <option value="">{{ t('availableChannels.catalog.allPlatforms') }}</option>
      <option v-for="item in platforms" :key="item" :value="item">{{ item }}</option>
    </select>

    <label
      class="group inline-flex min-h-11 shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap rounded-xl border border-slate-200 bg-slate-50/70 px-3 text-xs font-semibold text-slate-600 transition-colors hover:border-slate-300 hover:bg-white focus-within:ring-2 focus-within:ring-primary-500/40 dark:border-dark-600 dark:bg-dark-900/70 dark:text-slate-300 dark:hover:border-dark-500 dark:hover:bg-dark-900 sm:text-sm"
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
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

withDefaults(defineProps<{
  search: string
  platform: string
  pricedOnly: boolean
  platforms: string[]
  loading?: boolean
}>(), {
  loading: false,
})

const emit = defineEmits<{
  'update:search': [value: string]
  'update:platform': [value: string]
  'update:pricedOnly': [value: boolean]
  refresh: []
}>()

const { t } = useI18n()
</script>
