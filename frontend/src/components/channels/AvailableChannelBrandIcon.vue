<script setup lang="ts">
import { computed } from 'vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformGroupBadgeClass } from '@/utils/platformColors'
import { resolveAvailableChannelBrand } from './availableChannelBrand'

const props = defineProps<{ platform: string }>()
const brand = computed(() => resolveAvailableChannelBrand(props.platform))
const themeClass = computed(() => platformGroupBadgeClass(brand.value.platform))
</script>

<template>
  <span
    data-testid="brand-icon"
    :data-brand="brand.key"
    :aria-label="brand.label"
    class="inline-flex size-11 shrink-0 items-center justify-center rounded-2xl shadow-sm ring-1 ring-inset ring-black/5 dark:ring-white/10"
    :class="themeClass"
  >
    <PlatformIcon :platform="brand.platform" size="xl" />
  </span>
</template>
