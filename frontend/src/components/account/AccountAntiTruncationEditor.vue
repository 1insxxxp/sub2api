<template>
  <section class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600" data-testid="account-anti-truncation-editor">
    <label class="flex min-h-11 items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
      <input
        type="checkbox"
        :checked="accountEnabled"
        data-testid="anti-truncation-account-enabled"
        @change="setAccountEnabled(($event.target as HTMLInputElement).checked)"
      />
      {{ t('admin.accounts.antiTruncation.title') }}
    </label>
    <p class="input-hint">{{ t('admin.accounts.antiTruncation.hint') }}</p>
    <p v-if="modelValue === null" class="input-hint">{{ t('admin.accounts.antiTruncation.unconfiguredHint') }}</p>
    <template v-if="accountEnabled">
      <p class="input-hint">{{ t('admin.accounts.antiTruncation.scopeHint') }}</p>
      <div v-for="(rule, index) in modelValue?.rules || []" :key="index" class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/40 p-3 dark:border-dark-600 dark:bg-dark-800/40">
        <div class="flex items-end gap-2">
          <label class="min-w-0 flex-1">
            <span class="input-label">{{ t('admin.accounts.antiTruncation.model') }}</span>
            <input :value="rule.model" type="text" class="input font-mono" maxlength="200" :list="listId" :data-testid="`anti-truncation-model-${index}`" @input="updateRule(index, { model: ($event.target as HTMLInputElement).value })" />
          </label>
          <button type="button" class="btn btn-secondary min-h-11" :data-testid="`anti-truncation-remove-rule-${index}`" @click="removeRule(index)">{{ t('common.delete') }}</button>
        </div>
        <label class="flex min-h-11 items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input :checked="rule.enabled" type="checkbox" @change="updateRule(index, { enabled: ($event.target as HTMLInputElement).checked })" />
          {{ t('admin.accounts.antiTruncation.enabled') }}
        </label>
        <div v-if="rule.enabled" class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <label v-for="limit in limits" :key="limit.key">
            <span class="input-label">{{ t(`admin.accounts.antiTruncation.${limit.key}`) }}</span>
            <input :value="rule[limit.key] ?? ''" type="number" min="0" :max="limit.max" step="1" :placeholder="limit.placeholder" class="input" @input="updateLimit(index, limit.key, ($event.target as HTMLInputElement).value)" />
          </label>
        </div>
      </div>
      <p class="input-hint">{{ t('admin.accounts.antiTruncation.durationHint') }}</p>
      <datalist :id="listId"><option v-for="model in uniqueModels" :key="model" :value="model" /></datalist>
      <button type="button" class="btn btn-secondary text-sm" data-testid="anti-truncation-add-rule" @click="addRule">+ {{ t('admin.accounts.antiTruncation.addRule') }}</button>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountAntiTruncationPolicy, AccountAntiTruncationRule } from '@/utils/accountAntiTruncation'

const props = withDefaults(defineProps<{ modelValue: AccountAntiTruncationPolicy | null; models?: string[] }>(), { models: () => [] })
const emit = defineEmits<{ 'update:modelValue': [value: AccountAntiTruncationPolicy] }>()
const { t } = useI18n()
// A local off state lets administrators explicitly save an empty owned policy.
const explicitlyOff = ref(false)
const accountEnabled = computed(() => props.modelValue !== null && !explicitlyOff.value)
const uniqueModels = computed(() => [...new Set(props.models.filter(Boolean))])
const listId = `anti-truncation-models-${useId()}`
watch(() => props.modelValue, value => {
  if (value === null || value.rules.length > 0) explicitlyOff.value = false
})
const limits = [
  { key: 'max_rounds', max: 3, placeholder: '1' },
  { key: 'max_output_tokens', max: 32768, placeholder: '0' },
  { key: 'max_duration_seconds', max: 300, placeholder: '180' },
] as const
const setAccountEnabled = (enabled: boolean) => {
  explicitlyOff.value = !enabled
  emit('update:modelValue', { version: 1, rules: enabled ? props.modelValue?.rules || [] : [] })
}
const updateRule = (index: number, update: Partial<AccountAntiTruncationRule>) => {
  const rules = (props.modelValue?.rules || []).map((rule, i) => i === index ? { ...rule, ...update } : { ...rule })
  emit('update:modelValue', { version: 1, rules })
}
const updateLimit = (index: number, key: typeof limits[number]['key'], value: string) => updateRule(index, { [key]: value === '' ? undefined : Number(value) })
const addRule = () => emit('update:modelValue', { version: 1, rules: [...(props.modelValue?.rules || []), { model: '', enabled: true, max_rounds: 1, max_output_tokens: 0, max_duration_seconds: 180 }] })
const removeRule = (index: number) => emit('update:modelValue', { version: 1, rules: (props.modelValue?.rules || []).filter((_, i) => i !== index) })
</script>
