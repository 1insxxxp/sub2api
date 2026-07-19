<template>
  <div class="flex justify-between gap-3">
    <span class="text-gray-500 dark:text-gray-400">{{ label }}</span>
    <span class="min-w-0 text-right">
      <span class="flex items-center justify-end gap-1.5 font-mono">
        <span
          v-if="officialLabel"
          class="rounded border border-gray-200 bg-gray-50 px-1 py-0.5 text-[10px] font-medium leading-none text-gray-500 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300"
        >
          {{ officialLabel }}
        </span>
        <span>{{ display }}</span>
      </span>
      <span
        v-if="convertedDisplay"
        class="mt-1 flex items-center justify-end gap-1.5 font-mono text-[11px] text-primary-600 dark:text-primary-300"
      >
        <span
          v-if="convertedLabel"
          class="rounded border border-primary-200 bg-primary-50 px-1 py-0.5 text-[10px] font-medium leading-none text-primary-700 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200"
        >
          {{ convertedLabel }}
        </span>
        <span>{{ convertedDisplay }}</span>
      </span>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatScaled } from '@/utils/pricing'

function formatScaledWithSymbol(value: number | null, scale: number, symbol: string): string {
  if (value == null) return '-'
  return `${symbol}${(value * scale).toPrecision(10).replace(/\.?0+$/, '')}`
}

const props = withDefaults(
  defineProps<{
    label: string
    value: number | null
    unit: string
    scale: number
    officialLabel?: string
    convertedValue?: number | null
    convertedUnit?: string
    convertedScale?: number
    convertedLabel?: string
    convertedCurrencySymbol?: string
  }>(),
  { value: null, convertedCurrencySymbol: '$' }
)

const display = computed(() =>
  props.value == null ? '-' : `${formatScaled(props.value, props.scale)} ${props.unit}`
)

const convertedDisplay = computed(() => {
  if (props.convertedValue == null || !props.convertedUnit) return ''
  return `${formatScaledWithSymbol(
    props.convertedValue,
    props.convertedScale ?? props.scale,
    props.convertedCurrencySymbol
  )} ${props.convertedUnit}`
})
</script>
