<template>
  <section class="space-y-3" data-testid="model-aliases-field">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.groups.form.modelAliases') }}</h4>
        <p class="input-hint">{{ t('admin.groups.form.modelAliasesHint') }}</p>
      </div>
      <button type="button" class="btn btn-secondary shrink-0" @click="addAlias">{{ t('admin.groups.form.addModelAlias') }}</button>
    </div>
    <div v-for="(row, index) in modelValue" :key="index" class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/40 p-3 dark:border-dark-600 dark:bg-dark-800/40">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_1fr_auto]">
        <div>
          <label class="input-label" :for="`${idPrefix}-${index}-alias`">{{ t('admin.groups.form.modelAlias') }}</label>
          <input :id="`${idPrefix}-${index}-alias`" v-model="row.alias" class="input font-mono" maxlength="200" autocomplete="off" />
        </div>
        <div>
          <label class="input-label" :for="`${idPrefix}-${index}-upstream`">{{ t('admin.groups.form.modelAliasUpstream') }}</label>
          <input :id="`${idPrefix}-${index}-upstream`" v-model="row.upstream_model" class="input font-mono" maxlength="200" autocomplete="off" />
        </div>
        <button type="button" class="btn btn-secondary self-end" :aria-label="t('admin.groups.form.removeModelAlias')" @click="removeAlias(index)">{{ t('admin.groups.form.removeModelAlias') }}</button>
      </div>
      <label class="flex min-h-11 items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="row.anti_truncation.enabled" type="checkbox" />
        {{ t('admin.groups.form.antiTruncationEnabled') }}
      </label>
      <div v-if="row.anti_truncation.enabled" class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <label v-for="limit in limits" :key="limit.key" class="block">
          <span class="input-label">{{ t(limit.label) }}</span>
          <input v-model.number="row.anti_truncation[limit.key]" type="number" min="1" step="1" class="input" />
        </label>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { createModelAliasFormRow, type ModelAliasFormRow } from '@/utils/groupModelAliases'

const props = defineProps<{ modelValue: ModelAliasFormRow[]; idPrefix: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: ModelAliasFormRow[]] }>()
const { t } = useI18n()
const modelValue = computed({
  get: () => props.modelValue,
  set: (value: ModelAliasFormRow[]) => emit('update:modelValue', value),
})
const limits = [
  { key: 'max_rounds', label: 'admin.groups.form.antiTruncationMaxRounds' },
  { key: 'max_output_tokens', label: 'admin.groups.form.antiTruncationMaxOutputTokens' },
  { key: 'max_duration_seconds', label: 'admin.groups.form.antiTruncationMaxDuration' },
] as const
const addAlias = () => { modelValue.value = [...modelValue.value, createModelAliasFormRow()] }
const removeAlias = (index: number) => { modelValue.value = modelValue.value.filter((_, i) => i !== index) }
</script>
