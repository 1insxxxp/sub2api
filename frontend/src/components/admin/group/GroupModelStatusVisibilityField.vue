<template>
  <section
    class="model-status-visibility-field border-t border-gray-200 pt-4 dark:border-dark-500"
    data-testid="model-status-visibility-field"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0 flex-1">
        <label class="text-sm font-medium text-gray-800 dark:text-gray-100">
          {{ t('admin.groups.modelStatusVisibility.title') }}
        </label>
        <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.modelStatusVisibility.hint') }}
        </p>
      </div>
      <div class="model-status-visibility-toggle-hit flex min-h-11 min-w-11 shrink-0 items-center justify-end">
        <Toggle
          data-testid="model-status-visibility-toggle"
          :aria-label="t('admin.groups.modelStatusVisibility.enable')"
          :model-value="modelValue.enabled"
          @update:model-value="setEnabled"
        />
      </div>
    </div>

    <div v-if="modelValue.enabled" class="mt-3 space-y-3">
      <p class="text-xs text-primary-700 dark:text-primary-300">
        {{ t('admin.groups.modelStatusVisibility.enabledHint') }}
      </p>

      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div class="relative min-w-0 flex-1">
          <Icon
            name="search"
            size="sm"
            class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400"
          />
          <input
            v-model="searchQuery"
            type="search"
            class="input min-h-11 w-full pl-9 text-sm"
            data-testid="model-status-visibility-search"
            :placeholder="t('admin.groups.modelStatusVisibility.searchPlaceholder')"
            :aria-label="t('admin.groups.modelStatusVisibility.searchPlaceholder')"
          />
        </div>
        <div class="flex shrink-0 gap-2">
          <button
            type="button"
            class="min-h-11 rounded-lg border border-gray-200 px-3 text-xs font-medium text-primary-700 transition-colors hover:bg-primary-50 dark:border-dark-500 dark:text-primary-300 dark:hover:bg-primary-900/20"
            data-testid="model-status-visibility-select-all"
            @click="selectVisible"
          >
            {{ t('admin.groups.modelStatusVisibility.selectAll') }}
          </button>
          <button
            type="button"
            class="min-h-11 rounded-lg border border-gray-200 px-3 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 dark:border-dark-500 dark:text-gray-300 dark:hover:bg-dark-700"
            data-testid="model-status-visibility-clear"
            @click="clearSelection"
          >
            {{ t('admin.groups.modelStatusVisibility.clear') }}
          </button>
        </div>
      </div>

      <div class="flex items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span data-testid="model-status-visibility-summary">
          {{ t('admin.groups.modelStatusVisibility.selectedSummary', { selected: selectedCount, total: allModels.length }) }}
        </span>
        <span v-if="loading">{{ t('admin.groups.modelStatusVisibility.loading') }}</span>
      </div>

      <div
        v-if="loadError"
        class="rounded-lg border border-amber-200 bg-amber-50/70 px-3 py-2 text-xs leading-5 text-amber-800 dark:border-amber-900/70 dark:bg-amber-950/20 dark:text-amber-200"
        data-testid="model-status-visibility-load-error"
        role="status"
      >
        {{ t('admin.groups.modelStatusVisibility.loadError') }}
      </div>

      <div
        v-if="visibleModels.length > 0"
        class="max-h-64 overflow-y-auto rounded-xl border border-gray-200 bg-gray-50/60 p-2 dark:border-dark-500 dark:bg-dark-800/40"
        data-testid="model-status-visibility-options"
      >
        <label
          v-for="model in visibleModels"
          :key="model"
          class="flex min-h-11 cursor-pointer items-start gap-3 rounded-lg px-3 py-2 text-sm transition-colors hover:bg-white dark:hover:bg-dark-700"
          :class="isSelected(model) ? 'bg-white/80 dark:bg-dark-700/70' : ''"
        >
          <input
            type="checkbox"
            class="mt-1 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :checked="isSelected(model)"
            :aria-label="model"
            @change="toggleModel(model)"
          />
          <span class="min-w-0 break-words leading-5 text-gray-700 dark:text-gray-200">
            {{ model }}
          </span>
        </label>
      </div>
      <p
        v-else-if="loading"
        class="rounded-xl border border-dashed border-gray-200 px-3 py-5 text-center text-xs text-gray-500 dark:border-dark-500 dark:text-gray-400"
      >
        {{ t('admin.groups.modelStatusVisibility.loading') }}
      </p>
      <p
        v-else
        class="rounded-xl border border-dashed border-gray-200 px-3 py-5 text-center text-xs text-gray-500 dark:border-dark-500 dark:text-gray-400"
        data-testid="model-status-visibility-empty"
      >
        {{ t('admin.groups.modelStatusVisibility.empty') }}
      </p>

      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <input
          v-model="customModel"
          type="text"
          class="input min-h-11 min-w-0 flex-1 text-sm"
          data-testid="model-status-visibility-custom"
          :placeholder="t('admin.groups.modelStatusVisibility.customPlaceholder')"
          @keydown.enter.prevent="addCustomModel"
        />
        <button
          type="button"
          class="min-h-11 rounded-lg bg-primary-600 px-4 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50"
          data-testid="model-status-visibility-add-custom"
          :disabled="!customModel.trim()"
          @click="addCustomModel"
        >
          {{ t('admin.groups.modelStatusVisibility.addCustom') }}
        </button>
      </div>
      <p
        v-if="customError"
        class="text-xs text-red-600 dark:text-red-400"
        data-testid="model-status-visibility-custom-error"
        role="alert"
      >
        {{ t(`admin.groups.modelStatusVisibility.${customError}`) }}
      </p>
    </div>

    <p v-else class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">
      {{ t('admin.groups.modelStatusVisibility.disabledHint') }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import type { ModelStatusVisibility } from '@/types'

const props = withDefaults(defineProps<{
  modelValue: ModelStatusVisibility
  candidates?: string[]
  loading?: boolean
  loadError?: boolean
}>(), {
  candidates: () => [],
  loading: false,
  loadError: false,
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: ModelStatusVisibility): void
}>()

const { t } = useI18n()
const searchQuery = ref('')
const customModel = ref('')
const customError = ref<'duplicate' | 'invalidWildcard' | ''>('')

const allModels = computed(() => {
  const seen = new Set<string>()
  const models: string[] = []
  for (const raw of [...props.candidates, ...props.modelValue.models]) {
    const model = raw.trim()
    const key = model.toLowerCase()
    if (!model || seen.has(key)) continue
    seen.add(key)
    models.push(model)
  }
  return models
})

const visibleModels = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return allModels.value
  return allModels.value.filter(model => model.toLowerCase().includes(query))
})

const selectedModels = computed(() => new Set(props.modelValue.models.map(model => model.toLowerCase())))
const selectedCount = computed(() => props.modelValue.models.length)

const update = (patch: Partial<ModelStatusVisibility>) => {
  emit('update:modelValue', {
    enabled: patch.enabled ?? props.modelValue.enabled,
    models: [...(patch.models ?? props.modelValue.models)],
  })
}

const setEnabled = (enabled: boolean) => update({ enabled })
const isSelected = (model: string) => selectedModels.value.has(model.toLowerCase())

const toggleModel = (model: string) => {
  const key = model.toLowerCase()
  const next = props.modelValue.models.filter(existing => existing.toLowerCase() !== key)
  if (next.length === props.modelValue.models.length) next.push(model)
  update({ models: next })
}

const selectVisible = () => {
  const next = [...props.modelValue.models]
  const known = new Set(next.map(model => model.toLowerCase()))
  visibleModels.value.forEach(model => {
    if (!known.has(model.toLowerCase())) {
      known.add(model.toLowerCase())
      next.push(model)
    }
  })
  update({ models: next })
}

const clearSelection = () => update({ models: [] })

const addCustomModel = () => {
  const model = customModel.value.trim()
  if (!model) return
  if (model.slice(0, -1).includes('*')) {
    customError.value = 'invalidWildcard'
    return
  }
  if (allModels.value.some(candidate => candidate.toLowerCase() === model.toLowerCase())) {
    customError.value = 'duplicate'
    return
  }
  customError.value = ''
  customModel.value = ''
  update({ models: [...props.modelValue.models, model] })
}
</script>

<style scoped>
.model-status-visibility-field :deep(button),
.model-status-visibility-field :deep(input[type='checkbox']) {
  -webkit-tap-highlight-color: transparent;
}

.model-status-visibility-toggle-hit :deep(button)::before {
  position: absolute;
  inset: -12px -2px;
  content: '';
}

@media (prefers-reduced-motion: reduce) {
  .model-status-visibility-field * {
    transition-duration: 0.01ms !important;
  }
}
</style>
