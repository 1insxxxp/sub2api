<script setup lang="ts">
import { computed } from 'vue'
import { resolveAvailableChannelBrand } from './availableChannelBrand'

const props = defineProps<{ platform: string }>()
const brand = computed(() => resolveAvailableChannelBrand(props.platform))
</script>

<template>
  <span
    data-testid="brand-icon"
    :data-brand="brand.key"
    :aria-label="brand.label"
    class="inline-flex size-11 shrink-0 items-center justify-center rounded-2xl border shadow-sm"
    :class="[brand.surfaceClass, brand.borderClass, brand.accentClass]"
  >
    <svg v-if="brand.key === 'openai'" viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="1.9" aria-hidden="true"><path d="M12 3.2a4.4 4.4 0 0 1 4.2 3.1 4.4 4.4 0 0 1 2.6 7.7 4.4 4.4 0 0 1-6.8 5.1A4.4 4.4 0 0 1 5.8 17a4.4 4.4 0 0 1-.6-8.1A4.4 4.4 0 0 1 12 3.2Z"/><path d="m8.2 8.6 3.8-2.2 3.8 2.2v4.5L12 15.3 8.2 13.1Z"/></svg>
    <svg v-else-if="brand.key === 'anthropic'" viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="2" aria-hidden="true"><path d="m5 19 6.7-14L19 19M8 13.2h8"/><path d="M18 5v8"/></svg>
    <svg v-else-if="brand.key === 'gemini'" viewBox="0 0 24 24" class="size-6" fill="currentColor" aria-hidden="true"><path d="M12 2c.7 5.9 4.1 9.3 10 10-5.9.7-9.3 4.1-10 10-.7-5.9-4.1-9.3-10-10 5.9-.7 9.3-4.1 10-10Z"/></svg>
    <svg v-else-if="brand.key === 'grok'" viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M5 5h14L9 19H5l10-14M14 12l5 7"/></svg>
    <svg v-else viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true"><path d="M12 3 4.5 7.3v8.6L12 21l7.5-5.1V7.3Z"/><path d="m8.5 14.5 2.2-5h2.6l2.2 5M9.3 12.7h5.4"/></svg>
  </span>
</template>
