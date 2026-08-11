<template>
  <GroupBadge
    data-testid="available-channel-platform-badge"
    :name="platform"
    :platform="groupPlatform"
    :show-rate="false"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { GroupPlatform } from '@/types'
import GroupBadge from '@/components/common/GroupBadge.vue'

const props = defineProps<{ platform: string }>()
const groupPlatform = computed(() => props.platform as GroupPlatform)
</script>
