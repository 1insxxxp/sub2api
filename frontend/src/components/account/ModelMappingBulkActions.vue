<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  generateModelMappingsFromWhitelist,
  prependModelMappingPrefix,
  rebuildModelMappingSourcesFromTargets,
  type ModelMappingEntry
} from '@/composables/useModelWhitelist'

const props = defineProps<{ modelValue: ModelMappingEntry[]; allowedModels: string[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: ModelMappingEntry[]): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const prefix = ref('')
const removePrefix = ref('')
const removeSuffix = ref('')

const cleanupPreview = computed(() => {
  if (!removePrefix.value.trim() && !removeSuffix.value.trim()) {
    return { mappings: props.modelValue, changedCount: 0, collisionCount: 0 }
  }
  return rebuildModelMappingSourcesFromTargets(props.modelValue, prefix.value, removePrefix.value, removeSuffix.value)
})

const cleanupPreviewRows = computed(() => cleanupPreview.value.mappings.flatMap((mapping, index) => {
  const original = props.modelValue[index]
  return original && original.from !== mapping.from ? [{ index, from: mapping.from, before: original.from, after: mapping.from }] : []
}))

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
    appStore.showError(t('admin.accounts.wildcardOnlyAtEnd'))
    return
  }
  const result = prependModelMappingPrefix(props.modelValue, prefix.value)
  emit('update:modelValue', result)
  const changedCount = result.filter((mapping, index) => mapping.from !== props.modelValue[index]?.from).length
  appStore.showInfo(t('admin.accounts.modelMappingPrefixesApplied', { count: changedCount }))
}
function cleanup() {
  if (!prefix.value.trim()) {
    appStore.showError(t('admin.accounts.modelMappingCleanupPrefixEmpty'))
    return
  }
  if (!removePrefix.value.trim() && !removeSuffix.value.trim()) {
    appStore.showError(t('admin.accounts.modelMappingCleanupEmpty'))
    return
  }
  const result = rebuildModelMappingSourcesFromTargets(props.modelValue, prefix.value, removePrefix.value, removeSuffix.value)
  emit('update:modelValue', result.mappings)
  appStore.showInfo(t('admin.accounts.modelMappingCleanupApplied', { count: result.changedCount, collisions: result.collisionCount }))
}
</script>
<template>
  <div class="mb-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
    <label for="model-mapping-prefix" class="mb-2 block text-xs text-gray-500">{{ t('admin.accounts.modelMappingPrefix') }}</label>
    <p class="mb-2 text-xs text-gray-500">{{ t('admin.accounts.modelMappingBulkHint') }}</p>
    <p v-if="prefix.includes('*')" class="text-xs text-rose-600">{{ t('admin.accounts.wildcardOnlyAtEnd') }}</p>
    <p class="mb-2 text-xs text-gray-500">{{ t('admin.accounts.modelMappingCleanupHint') }}</p>
    <div class="mb-2 flex flex-wrap gap-2">
      <input
        v-model="removePrefix"
        data-testid="remove-upstream-prefix"
        type="text"
        class="input min-w-[10rem] flex-1"
        :placeholder="t('admin.accounts.modelMappingRemovePrefix')"
      />
      <input
        v-model="removeSuffix"
        data-testid="remove-upstream-suffix"
        type="text"
        class="input min-w-[10rem] flex-1"
        :placeholder="t('admin.accounts.modelMappingRemoveSuffix')"
      />
      <button
        type="button"
        data-testid="cleanup-mapping-targets"
        class="btn btn-secondary text-sm"
        :disabled="!prefix.trim() || (!removePrefix.trim() && !removeSuffix.trim())"
        @click="cleanup"
      >
        {{ t('admin.accounts.modelMappingCleanup') }}
      </button>
    </div>
    <div v-if="cleanupPreviewRows.length" data-testid="cleanup-preview" class="mb-2 max-h-32 overflow-y-auto rounded border border-dashed border-gray-300 p-2 text-xs text-gray-600 dark:border-dark-500 dark:text-gray-300">
      <div class="mb-1 font-medium">{{ t('admin.accounts.modelMappingCleanupPreview', { count: cleanupPreview.changedCount, collisions: cleanupPreview.collisionCount }) }}</div>
      <div v-for="row in cleanupPreviewRows" :key="row.index" class="truncate">
        <span>{{ row.from }}</span>: <span class="text-gray-400 line-through">{{ row.before }}</span> → <span>{{ row.after }}</span>
      </div>
    </div>
    <div class="flex flex-wrap gap-2">
      <input
        v-model="prefix"
        data-testid="model-mapping-prefix"
        id="model-mapping-prefix"
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
