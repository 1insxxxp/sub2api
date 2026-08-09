<template>
  <div
    data-testid="catalog-scroll-host"
    class="table-wrapper min-h-0 xl:h-full xl:[container-type:size]"
  >
    <section
      v-if="loading && channels.length === 0"
      data-testid="catalog-loading"
      class="xl:grid xl:grid-cols-[280px_minmax(0,1fr)] xl:gap-6"
      :aria-label="t(`${catalogKey}.loading`)"
      aria-busy="true"
    >
      <p class="sr-only">{{ t(`${catalogKey}.loading`) }}</p>
      <aside
        data-testid="catalog-loading-rail"
        class="hidden space-y-3 rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800 xl:block"
      >
        <div v-for="item in 5" :key="item" class="h-20 animate-pulse rounded-xl bg-gray-100 motion-reduce:animate-none dark:bg-dark-700" />
      </aside>
      <div data-testid="catalog-loading-detail" class="space-y-4">
        <div class="h-32 animate-pulse rounded-2xl bg-gray-100 motion-reduce:animate-none dark:bg-dark-700" />
        <div class="h-64 animate-pulse rounded-2xl bg-gray-100 motion-reduce:animate-none dark:bg-dark-700" />
      </div>
    </section>

    <section
      v-else-if="channels.length === 0"
      data-testid="catalog-empty"
      class="rounded-2xl border border-dashed border-gray-300 bg-white px-6 py-16 text-center text-sm text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400"
    >
      {{ t(emptyKind === 'no-results' ? `${catalogKey}.noMatchingResults` : `${catalogKey}.noChannels`) }}
    </section>

    <section
      v-else
      data-testid="channel-catalog-layout"
      class="relative min-w-0 space-y-4 xl:grid xl:grid-cols-[280px_minmax(0,1fr)] xl:items-start xl:gap-6 xl:space-y-0"
      :aria-busy="refreshing"
    >
      <div
        v-if="rateFallback"
        data-testid="rate-fallback-warning"
        class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300 xl:col-span-2"
        role="status"
      >
        {{ t(`${catalogKey}.rateFallback`) }}
      </div>

      <div
        v-if="refreshing"
        data-testid="refreshing-indicator"
        class="pointer-events-none absolute right-3 top-3 z-10 rounded-full border border-primary-200 bg-white/95 px-3 py-1 text-xs font-medium text-primary-700 shadow-sm dark:border-primary-500/30 dark:bg-dark-800/95 dark:text-primary-200"
        role="status"
        aria-live="polite"
      >
        {{ t(`${catalogKey}.refreshing`) }}
      </div>

      <AvailableChannelPicker v-model="selectedChannelKey" :channels="channels" />

      <aside
        data-testid="channel-navigation"
        class="hidden rounded-2xl border border-gray-200 bg-white p-3 shadow-sm dark:border-dark-600 dark:bg-dark-800 xl:sticky xl:top-4 xl:block xl:max-h-[calc(100cqh-2rem)] xl:overflow-y-auto"
      >
        <div
          ref="channelListboxRef"
          role="listbox"
          :aria-label="t(`${catalogKey}.channelNavigation`)"
          class="space-y-2"
        >
          <button
            v-for="channel in channels"
            :key="channel.key"
            :ref="(element) => setNavButtonRef(channel.key, element)"
            type="button"
            role="option"
            data-testid="channel-nav-item"
            class="w-full min-w-0 rounded-xl border px-3 py-3 text-left transition-colors motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
            :class="channel.key === selectedChannelKey
              ? 'border-primary-300 bg-primary-50 text-primary-800 dark:border-primary-500/40 dark:bg-primary-500/10 dark:text-primary-100'
              : 'border-transparent text-gray-700 hover:border-gray-200 hover:bg-gray-50 dark:text-gray-200 dark:hover:border-dark-500 dark:hover:bg-dark-700'"
            :aria-selected="channel.key === selectedChannelKey"
            :tabindex="channel.key === selectedChannelKey ? 0 : -1"
            :aria-controls="detailRegionId"
            @click="selectedChannelKey = channel.key"
            @keydown="handleNavKeydown($event, channel.key)"
          >
            <span class="block min-w-0 break-words text-sm font-semibold [overflow-wrap:anywhere]">
              {{ channel.name }}
            </span>
            <span class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="platform in channel.platforms"
                :key="platform"
                class="inline-flex items-center gap-1 rounded-md bg-gray-100 px-1.5 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
              >
                <PlatformIcon :platform="asPlatform(platform)" size="xs" />
                {{ platform }}
              </span>
            </span>
            <span class="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t(`${catalogKey}.groupsCount`, { count: channel.groupCount }) }}</span>
              <span>{{ t(`${catalogKey}.modelsCount`, { count: channel.modelCount }) }}</span>
            </span>
          </button>
        </div>
      </aside>

      <section
        v-if="selectedChannel || modelEntries"
        :id="detailRegionId"
        data-testid="channel-detail"
        class="min-w-0 space-y-4"
        :aria-labelledby="detailHeadingId"
      >
        <header class="min-w-0 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-600 dark:bg-dark-800 sm:p-5">
          <div v-if="selectedChannel" class="flex min-w-0 flex-wrap items-start justify-between gap-4">
            <div class="min-w-0 flex-1">
              <h2
                :id="detailHeadingId"
                data-testid="channel-detail-name"
                class="min-w-0 text-lg font-semibold leading-7 text-gray-900 [overflow-wrap:anywhere] dark:text-white"
              >
                {{ selectedChannel.name }}
              </h2>
              <p
                v-if="selectedChannel.description"
                data-testid="channel-description"
                class="mt-1 min-w-0 break-words text-sm leading-6 text-gray-500 dark:text-gray-400"
              >
                {{ selectedChannel.description }}
              </p>
            </div>
            <div class="flex shrink-0 flex-wrap justify-end gap-2">
              <span
                v-for="platform in selectedChannel.platforms"
                :key="platform"
                class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs font-medium text-gray-600 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-300"
              >
                <PlatformIcon :platform="asPlatform(platform)" size="sm" />
                {{ platform }}
              </span>
            </div>
          </div>
          <div v-if="selectedChannel" class="mt-4 flex flex-wrap gap-2">
            <span class="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              {{ t(`${catalogKey}.groupsCount`, { count: selectedChannel.groupCount }) }}
            </span>
            <span class="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              {{ t(`${catalogKey}.modelsCount`, { count: selectedChannel.modelCount }) }}
            </span>
          </div>
        </header>

        <AvailableChannelModelList
          v-if="modelEntries"
          :entries="visibleModelEntries"
          class="rounded-2xl"
        />
        <AvailableChannelGroupSection
          v-else
          v-for="(group, index) in selectedChannel?.groups ?? []"
          :key="group.key"
          :group="group"
          :default-expanded="index === 0"
        />
      </section>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, nextTick, ref, watch, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import type { GroupPlatform } from '@/types'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import AvailableChannelGroupSection from './AvailableChannelGroupSection.vue'
import AvailableChannelPicker from './AvailableChannelPicker.vue'
import AvailableChannelModelList from './AvailableChannelModelList.vue'
import { projectModelEntriesForChannel, type CatalogModelListEntry } from './availableChannelCatalog'
import type { CatalogChannelEntry } from './availableChannelCatalog'

const catalogKey = 'availableChannels.catalog'
const props = withDefaults(defineProps<{
  channels: CatalogChannelEntry[]
  loading: boolean
  refreshing: boolean
  rateFallback: boolean
  emptyKind?: 'no-data' | 'no-results'
  modelEntries?: CatalogModelListEntry[]
}>(), {
  emptyKind: 'no-data',
})

const { t } = useI18n()
const selectedChannelKey = ref<string | null>(null)
const navButtonRefs = new Map<string, HTMLButtonElement>()
const channelListboxRef = ref<HTMLElement | null>(null)
const instanceUid = getCurrentInstance()?.uid ?? 0
const detailRegionId = `available-channel-detail-${instanceUid}`
const detailHeadingId = `${detailRegionId}-heading`

const selectedChannel = computed(() => (
  props.channels.find((channel) => channel.key === selectedChannelKey.value) ?? null
))
const visibleModelEntries = computed(() => {
  if (!props.modelEntries) return []
  return projectModelEntriesForChannel(props.modelEntries, selectedChannelKey.value)
})

watch(
  () => props.channels.map((channel) => channel.key),
  async (keys) => {
    const focusWasInListbox = channelListboxRef.value?.contains(document.activeElement) ?? false
    let selectionChanged = false
    if (selectedChannelKey.value == null || !keys.includes(selectedChannelKey.value)) {
      selectedChannelKey.value = keys[0] ?? null
      selectionChanged = true
    }
    if (selectionChanged && focusWasInListbox && selectedChannelKey.value != null) {
      await nextTick()
      navButtonRefs.get(selectedChannelKey.value)?.focus()
    }
  },
  { immediate: true },
)

function asPlatform(value: string): GroupPlatform {
  return value as GroupPlatform
}

function setNavButtonRef(
  key: string,
  element: Element | ComponentPublicInstance | null,
): void {
  if (element instanceof HTMLButtonElement) {
    navButtonRefs.set(key, element)
  } else {
    navButtonRefs.delete(key)
  }
}

async function selectAndFocus(index: number): Promise<void> {
  const channel = props.channels[index]
  if (!channel) return
  selectedChannelKey.value = channel.key
  await nextTick()
  navButtonRefs.get(channel.key)?.focus()
}

function handleNavKeydown(event: KeyboardEvent, currentKey: string): void {
  const currentIndex = props.channels.findIndex((channel) => channel.key === currentKey)
  if (currentIndex < 0) return

  let nextIndex: number | null = null
  switch (event.key) {
    case 'ArrowDown':
      nextIndex = Math.min(currentIndex + 1, props.channels.length - 1)
      break
    case 'ArrowUp':
      nextIndex = Math.max(currentIndex - 1, 0)
      break
    case 'Home':
      nextIndex = 0
      break
    case 'End':
      nextIndex = props.channels.length - 1
      break
    default:
      return
  }

  event.preventDefault()
  void selectAndFocus(nextIndex)
}
</script>
