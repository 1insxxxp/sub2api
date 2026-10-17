<template>
  <section data-testid="empty-response-cap-rules" class="min-w-0 space-y-1 text-xs">
    <p class="font-medium text-gray-700 dark:text-gray-200">{{ t('usage.emptyResponse.capTitle') }}</p>
    <p class="leading-5 text-gray-500 dark:text-gray-400">{{ t('usage.emptyResponse.capHint') }}</p>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>
