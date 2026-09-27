<template>
  <div class="space-y-4">
    <!-- Quick Amount Buttons -->
    <div>
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.quickAmounts') }}
      </label>
      <div class="grid grid-cols-3 gap-2">
        <button
          v-for="amt in filteredAmounts"
          :key="amt"
          type="button"
          :class="[
            'min-h-[58px] rounded-lg border-2 px-4 py-2 text-center font-medium transition-colors',
            modelValue === amt
              ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/40 dark:text-primary-300'
              : 'border-gray-200 bg-white text-gray-700 hover:border-primary-300 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:border-primary-500/40',
          ]"
          :aria-label="getAmountAriaLabel(amt)"
          @click="selectAmount(amt)"
        >
          <template v-if="getAmountDisplay(amt)?.original">
            <span class="block text-xs font-normal text-gray-400 line-through dark:text-dark-500">
              <span class="sr-only">{{ t('payment.rechargePromotionOriginalPrice') }}</span>
              {{ getAmountDisplay(amt)?.original }}
            </span>
            <span class="block text-base font-semibold leading-5">{{ getAmountDisplay(amt)?.current ?? amt }}</span>
            <span v-if="getAmountDisplay(amt)?.savings" class="mt-0.5 block text-[11px] font-medium leading-4 text-green-600 dark:text-green-400">
              {{ getAmountDisplay(amt)?.savings }}
            </span>
          </template>
          <template v-else>{{ getAmountDisplay(amt)?.current ?? amt }}</template>
        </button>
      </div>
    </div>

    <!-- Custom Amount Input -->
    <div v-if="props.allowCustom">
      <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('payment.customAmount') }}
      </label>
      <div class="relative">
        <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-dark-500">
          $
        </span>
        <input
          type="text"
          inputmode="decimal"
          :value="customText"
          :placeholder="placeholderText"
          class="input w-full py-3 pl-8 pr-4"
          @input="handleInput"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  amounts?: number[]
  amountDisplay?: Record<number, {
    original?: string
    current?: string
    savings?: string
  }>
  modelValue: number | null
  min?: number
  max?: number
  allowCustom?: boolean
}>(), {
  amounts: () => [10, 20, 50, 100, 200, 500, 1000, 2000, 5000],
  min: 0,
  max: 0,
  allowCustom: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const customText = ref('')

// 0 = no limit
const filteredAmounts = computed(() =>
  props.amounts.filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

function selectAmount(amt: number) {
  customText.value = String(amt)
  emit('update:modelValue', amt)
}

function getAmountDisplay(amt: number) {
  return props.amountDisplay?.[amt]
}

function getAmountAriaLabel(amt: number): string {
  const display = getAmountDisplay(amt)
  if (!display?.original) return display?.current ?? String(amt)
  return `${t('payment.rechargePromotionOriginalPrice')}: ${display.original}; ${display.current ?? amt}; ${display.savings ?? ''}`.trim()
}

function handleInput(e: Event) {
  const input = e.target as HTMLInputElement
  const val = input.value
  if (!AMOUNT_PATTERN.test(val)) {
    input.value = customText.value
    return
  }
  customText.value = val
  if (val === '') {
    emit('update:modelValue', null)
    return
  }
  const num = parseFloat(val)
  if (!isNaN(num) && num > 0) {
    emit('update:modelValue', num)
  } else {
    emit('update:modelValue', null)
  }
}

watch(() => props.modelValue, (v) => {
  if (v !== null && String(v) !== customText.value) {
    customText.value = String(v)
  }
}, { immediate: true })
</script>
