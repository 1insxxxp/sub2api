<template>
  <div class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <label class="input-label mb-0">{{ t('admin.settings.payment.rechargeTiers') }}</label>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="modelValue.length >= 50" @click="addTier">
        <Icon name="plus" size="sm" />{{ t('admin.settings.payment.addRechargeTier') }}
      </button>
    </div>
    <p class="text-xs text-gray-500">{{ t('admin.settings.payment.rechargeTiersHint') }}</p>
    <div v-for="(tier, index) in displayRows" :key="index" class="grid grid-cols-2 items-end gap-3 border-b border-gray-100 pb-3 sm:grid-cols-[1fr_1fr_1fr_auto] dark:border-dark-700">
      <label class="min-w-0 text-xs text-gray-600 dark:text-gray-300">
        {{ t('admin.settings.payment.tierAmount') }}
        <input class="input mt-1 w-full" type="number" min="0.01" step="0.01" :value="tier.amount" @input="update(index, 'amount', $event)" />
      </label>
      <label class="min-w-0 text-xs text-gray-600 dark:text-gray-300">
        {{ t('admin.settings.payment.balanceRechargeMultiplier') }}
        <input class="input mt-1 w-full" type="number" min="0.000001" step="any" :placeholder="t('admin.settings.payment.tierMultiplierPlaceholder')" :value="tier.multiplier || ''" @input="update(index, 'multiplier', $event)" />
      </label>
      <button type="button" class="btn btn-secondary h-10 w-10 justify-self-end p-2 text-red-500" :title="t('common.delete')" :aria-label="t('common.delete')" @click="emit('update:modelValue', displayRows.filter((_, row) => row !== index))">
        <Icon name="trash" size="sm" />
      </button>
    </div>
    <p v-if="modelValue.length > 0 && !validRechargeTiers(modelValue)" role="alert" class="text-sm text-red-500">{{ t('admin.settings.payment.invalidRechargeTiers') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { BalanceRechargeTier } from '@/types/payment'
import { validRechargeTiers } from './rechargeTiers'

const { t } = useI18n()
const props = defineProps<{ modelValue: BalanceRechargeTier[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: BalanceRechargeTier[]] }>()
const defaultRows: BalanceRechargeTier[] = [
      { amount: 10, multiplier: 0 },
      { amount: 18, multiplier: 0 },
      { amount: 80, multiplier: 0 },
]
const displayRows = computed(() => props.modelValue.length ? props.modelValue : defaultRows)

function update(index: number, field: keyof BalanceRechargeTier, event: Event) {
  const value = Number((event.target as HTMLInputElement).value)
  emit('update:modelValue', displayRows.value.map((tier, row) => row === index ? { ...tier, [field]: value } : tier))
}

function addTier() {
  if (!props.modelValue.length) {
    emit('update:modelValue', defaultRows)
    return
  }
  const last = props.modelValue[props.modelValue.length - 1]
  emit('update:modelValue', [...props.modelValue, { amount: last.amount + 1, multiplier: 0 }])
}
</script>
