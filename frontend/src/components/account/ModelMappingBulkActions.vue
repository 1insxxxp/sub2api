<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { generateModelMappingsFromWhitelist, prependModelMappingPrefix, type ModelMappingEntry } from '@/composables/useModelWhitelist'
const props = defineProps<{ modelValue: ModelMappingEntry[]; allowedModels: string[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: ModelMappingEntry[]): void }>()
const { t } = useI18n(); const appStore = useAppStore(); const prefix = ref('')
const invalid = () => prefix.value.includes('*')
function generate() { if (!props.allowedModels.length) return appStore.showError(t('admin.accounts.modelMappingWhitelistEmpty')); if (!prefix.value.trim()) return appStore.showError(t('admin.accounts.modelMappingPrefixEmpty')); if (invalid()) return appStore.showError(t('admin.accounts.wildcardOnlyAtEnd')); const result = generateModelMappingsFromWhitelist(props.allowedModels, prefix.value, props.modelValue); emit('update:modelValue', result); appStore.showInfo(t('admin.accounts.modelMappingsGenerated', { count: result.length - props.modelValue.length })) }
function prepend() { if (!prefix.value.trim()) return appStore.showError(t('admin.accounts.modelMappingPrefixEmpty')); if (invalid()) return appStore.showError(t('admin.accounts.targetNoWildcard')); const result = prependModelMappingPrefix(props.modelValue, prefix.value); emit('update:modelValue', result); appStore.showInfo(t('admin.accounts.modelMappingPrefixesApplied', { count: result.filter((m, i) => m.from !== props.modelValue[i]?.from).length })) }
</script>
<template>
  <div class="mb-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
    <label class="mb-2 block text-xs text-gray-500">{{ t('admin.accounts.modelMappingPrefix') }}</label>
    <p class="mb-2 text-xs text-gray-500">{{ t('admin.accounts.modelMappingBulkHint') }}</p>
    <div class="flex flex-wrap gap-2"><input v-model="prefix" class="input min-w-[12rem] flex-1" :placeholder="t('admin.accounts.modelMappingPrefixPlaceholder')"/><button type="button" class="btn btn-secondary text-sm" @click="generate">{{ t('admin.accounts.generateMappingsFromWhitelist') }}</button><button type="button" class="btn btn-secondary text-sm" @click="prepend">{{ t('admin.accounts.prependMappingPrefix') }}</button></div>
  </div>
</template>
