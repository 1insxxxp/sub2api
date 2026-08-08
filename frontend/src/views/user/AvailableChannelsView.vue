<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('availableChannels.searchPlaceholder')"
                class="input pl-10"
              />
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <select v-model="platformFilter" class="input min-w-32" :aria-label="t('availableChannels.catalog.platformFilter')">
                <option value="">{{ t('availableChannels.catalog.allPlatforms') }}</option>
                <option v-for="platform in availablePlatforms" :key="platform" :value="platform">{{ platform }}</option>
              </select>
              <label class="inline-flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
                <input v-model="pricedOnly" type="checkbox" class="checkbox" />
                {{ t('availableChannels.catalog.pricedOnly') }}
              </label>
            </div>
          </div>

          <div class="flex w-full flex-shrink-0 flex-wrap items-center justify-end gap-3 lg:w-auto">
            <button
              @click="loadChannels"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh', 'Refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <AvailableChannelCatalog
          :channels="filteredCatalog"
          :model-entries="modelEntries"
          :loading="loading"
          :refreshing="loading && channels.length > 0"
          :rate-fallback="rateFallback"
          :empty-kind="channels.length > 0 && filteredCatalog.length === 0 ? 'no-results' : 'no-data'"
        />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AvailableChannelCatalog from '@/components/channels/AvailableChannelCatalog.vue'
import { buildAvailableChannelCatalog, buildAvailableChannelModelList, filterAvailableChannelCatalog } from '@/components/channels/availableChannelCatalog'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const platformFilter = ref('')
const pricedOnly = ref(false)

const priceCnyMultiplier = computed(() => {
  const value = Number(appStore.cachedPublicSettings?.available_channels_price_cny_multiplier)
  return Number.isFinite(value) && value > 0 ? value : 0
})

const rateFallback = ref(false)

/**
 * 搜索过滤：
 * - 命中渠道名/描述 → 整个渠道（所有 platforms）都保留
 * - 否则按 platform/group/model 维度在 sections 里过滤，保留有匹配的 section
 * - 所有 sections 都不匹配时，渠道本身被过滤掉
 */
const catalog = computed(() => buildAvailableChannelCatalog(channels.value, userGroupRates.value, priceCnyMultiplier.value))
const filteredCatalog = computed(() => filterAvailableChannelCatalog(catalog.value, {
  search: searchQuery.value,
  platform: platformFilter.value,
  pricedOnly: pricedOnly.value,
}))
const modelEntries = computed(() => buildAvailableChannelModelList(filteredCatalog.value))
const availablePlatforms = computed(() => [...new Set(catalog.value.flatMap(channel => channel.platforms))].sort())

async function loadChannels() {
  loading.value = true
  try {
    // 渠道列表和用户专属倍率并发拉取。专属倍率失败不阻塞渠道展示——
    // 失败时只是无法渲染专属倍率角标，降级为仅显示默认倍率。
    rateFallback.value = false
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        rateFallback.value = true
        return {} as Record<number, number>
      }),
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadChannels)
</script>
