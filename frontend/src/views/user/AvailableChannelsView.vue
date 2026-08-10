<template>
  <AppLayout>
    <TablePageLayout>
      <template #table>
        <AvailableChannelCatalog
          :channels="filteredCatalog"
          :model-entries="modelEntries"
          :loading="loading"
          :refreshing="loading && channels.length > 0"
          :rate-fallback="rateFallback"
          :empty-kind="channels.length > 0 && filteredCatalog.length === 0 ? 'no-results' : 'no-data'"
        >
          <template #toolbar="{ selectedChannel, headingId }">
            <AvailableChannelsToolbar
              v-model:search="searchQuery"
              v-model:platform="platformFilter"
              v-model:priced-only="pricedOnly"
              :platforms="availablePlatforms"
              :loading="loading"
              :channel-name="selectedChannel?.name"
              :channel-description="selectedChannel?.description"
              :channel-platforms="selectedChannel?.platforms"
              :group-count="selectedChannel?.groupCount"
              :model-count="selectedChannel?.modelCount"
              :heading-id="headingId"
              @refresh="loadChannels"
            />
          </template>
        </AvailableChannelCatalog>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import AvailableChannelCatalog from '@/components/channels/AvailableChannelCatalog.vue'
import AvailableChannelsToolbar from '@/components/channels/AvailableChannelsToolbar.vue'
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
