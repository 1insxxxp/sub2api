<template>
  <label class="flex min-h-11 cursor-pointer items-start gap-3">
    <input
      v-model="enabled"
      type="checkbox"
      class="mt-1 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
      :aria-label="t('admin.groups.inputTokenProtection.enable')"
      :aria-describedby="id + '-hint'"
      :data-testid="id"
    />
    <span class="min-w-0">
      <span class="block text-sm font-medium text-gray-800 dark:text-gray-100">
        {{ t('admin.groups.inputTokenProtection.enable') }}
      </span>
      <span :id="id + '-hint'" class="mt-1 block text-xs leading-5 text-gray-500 dark:text-gray-400">
        {{ t('admin.groups.inputTokenProtection.hint') }}
      </span>
    </span>
  </label>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ id: string }>()
const enabled = defineModel<boolean>({ default: false })
const { t } = useI18n()
</script>
