<template>
  <div v-if="entry.status === 'idle'" class="mt-0.5 text-xs">
    <button
      type="button"
      class="text-primary-600 underline decoration-dashed underline-offset-2 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
      @click="handleFetch"
    >
      {{ t('usage.ipGeo.fetch') }}
    </button>
  </div>

  <div
    v-else-if="entry.status === 'loading'"
    class="mt-0.5 flex items-center gap-1 text-xs text-gray-400 dark:text-gray-500"
  >
    <svg class="h-3 w-3 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
      />
    </svg>
    {{ t('usage.ipGeo.fetching') }}
  </div>

  <div v-else-if="entry.status === 'success'" class="mt-0.5 text-xs">
    <div class="flex items-center gap-1">
      <button
        type="button"
        class="truncate text-gray-500 underline decoration-dotted underline-offset-2 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400"
        :title="tooltipText"
        :aria-expanded="detailsOpen"
        @click="detailsOpen = !detailsOpen"
      >
        {{ entry.label }}
      </button>
      <button
        type="button"
        class="text-gray-400 hover:text-primary-600 dark:hover:text-primary-400"
        :title="t('usage.ipGeo.refreshTitle')"
        @click="handleRefresh"
      >
        <Icon name="refresh" size="xs" />
      </button>
    </div>
    <p v-if="detailsOpen" class="mt-1 whitespace-pre-line break-words text-gray-500 dark:text-gray-400">{{ tooltipText || t('usage.ipGeo.offlineHint') }}</p>
    <a
      v-if="provider"
      :href="provider.url"
      target="_blank"
      rel="noopener noreferrer"
      referrerpolicy="no-referrer"
      class="mt-0.5 inline-block text-[10px] text-gray-400 hover:underline dark:text-gray-500"
    >{{ provider.label }}</a>
  </div>

  <div v-else-if="entry.status === 'error'" class="mt-0.5 text-xs">
    <button
      type="button"
      class="text-red-600 underline decoration-dashed underline-offset-2 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
      @click="handleFetch"
    >
      {{ t('usage.ipGeo.failed') }}
    </button>
  </div>

  <div v-else class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
    {{ t('usage.ipGeo.private') }}
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { fetchOne, getEntry } from '@/utils/ipGeoLookup'

const props = defineProps<{ ip: string }>()
const { t } = useI18n()

const entry = computed(() => getEntry(props.ip))
const detailsOpen = ref(false)
watch(() => props.ip, () => { detailsOpen.value = false })

const provider = computed(() => {
  const source = entry.value.detail?.source
  if (source === 'dbip') return { url: 'https://db-ip.com', label: t('usage.ipGeo.dbipAttribution') }
  if (source === 'maxmind') return { url: 'https://www.maxmind.com', label: t('usage.ipGeo.maxmindAttribution') }
  return null
})

const tooltipText = computed(() => {
  const detail = entry.value.detail
  if (!detail) return ''
  const lines = [
    detail.organization ? `${t('usage.ipGeo.detailOrg')}: ${detail.organization}` : '',
    detail.timezone ? `${t('usage.ipGeo.detailTimezone')}: ${detail.timezone}` : '',
    detail.accuracy != null ? `${t('usage.ipGeo.detailAccuracy')}: ${detail.accuracy}km` : '',
    detail.latitude && detail.longitude
      ? `${t('usage.ipGeo.detailCoordinates')}: ${detail.latitude}, ${detail.longitude}`
      : '',
  ].filter(Boolean)
  return lines.join('\n')
})

const handleFetch = () => {
  void fetchOne(props.ip)
}

const handleRefresh = () => {
  void fetchOne(props.ip, true)
}

</script>
