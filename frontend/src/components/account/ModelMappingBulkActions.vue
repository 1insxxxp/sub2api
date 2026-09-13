<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  generateModelMappingsFromWhitelist,
  prependModelMappingPrefix,
  type ModelMappingEntry
} from '@/composables/useModelWhitelist'

const props = defineProps<{ modelValue: ModelMappingEntry[]; allowedModels: string[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: ModelMappingEntry[]): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const prefix = ref('')

const hasInvalidWildcard = () => prefix.value.includes('*')

function generate() {
  if (!props.allowedModels.length) {
    appStore.showError(t('admin.accounts.modelMappingWhitelistEmpty'))
    return
  }
  if (!prefix.value.trim()) {
    appStore.showError(t('admin.accounts.modelMappingPrefixEmpty'))
    return
  }
  if (hasInvalidWildcard()) {
    appStore.showError(t('admin.accounts.wildcardOnlyAtEnd'))
    return
  }
  const result = generateModelMappingsFromWhitelist(props.allowedModels, prefix.value, props.modelValue)
  emit('update:modelValue', result)
  appStore.showInfo(t('admin.accounts.modelMappingsGenerated', { count: result.length - props.modelValue.length }))
}

function prepend() {
  if (!prefix.value.trim()) {
    appStore.showError(t('admin.accounts.modelMappingPrefixEmpty'))
    return
  }
  if (hasInvalidWildcard()) {
    appStore.showError(t('admin.accounts.targetNoWildcard'))
    return
  }
  const result = prependModelMappingPrefix(props.modelValue, prefix.value)
  emit('update:modelValue', result)
  const changedCount = result.filter((mapping, index) => mapping.from !== props.modelValue[index]?.from).length
  appStore.showInfo(t('admin.accounts.modelMappingPrefixesApplied', { count: changedCount }))
}
</script>
<template>
  <div class="mb-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
    <label class="mb-2 block text-xs text-gray-500">{{ t('admin.accounts.modelMappingPrefix') }}</label>
    <p class="mb-2 text-xs text-gray-500">{{ t('admin.accounts.modelMappingBulkHint') }}</p>
    <p v-if="prefix.includes('*')" class="text-xs text-rose-600">{{ t('admin.accounts.wildcardOnlyAtEnd') }}</p>
    <div class="flex flex-wrap gap-2">
      <input
        v-model="prefix"
        data-testid="model-mapping-prefix"
        type="text"
        class="input min-w-[12rem] flex-1"
        :placeholder="t('admin.accounts.modelMappingPrefixPlaceholder')"
      />
      <button
        type="button"
        data-testid="generate-mappings"
        class="btn btn-secondary text-sm"
        :disabled="!allowedModels.length || !prefix.trim() || hasInvalidWildcard()"
        @click="generate"
      >
        {{ t('admin.accounts.generateMappingsFromWhitelist') }}
      </button>
      <button
        type="button"
        data-testid="prepend-prefix"
        class="btn btn-secondary text-sm"
        :disabled="!prefix.trim() || hasInvalidWildcard()"
        @click="prepend"
      >
        {{ t('admin.accounts.prependMappingPrefix') }}
      </button>
    </div>
  </div>
</template>
