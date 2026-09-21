<template>
  <fieldset>
    <legend class="input-label">{{ t('common.groupTags.label') }}</legend>
    <div class="mb-3 grid min-w-0 gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
      <div class="min-w-0">
        <label :for="`${name}-text`" class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('common.groupTags.text') }}</label>
        <input
          :id="`${name}-text`"
          data-test="group-tag-text"
          :value="displayText"
          type="text"
          class="input min-h-11 w-full"
          :placeholder="t('common.groupTags.placeholder')"
          maxlength="20"
          @input="updateText(($event.target as HTMLInputElement).value)"
        />
      </div>
      <div>
        <label :for="`${name}-color-hex`" class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('common.groupTags.color') }}</label>
        <div class="flex items-center gap-2">
          <input
            :id="`${name}-color`"
            type="color"
            :value="resolvedColor"
            :aria-label="t('common.groupTags.color')"
            class="h-11 w-11 shrink-0 cursor-pointer rounded-md border border-gray-200 bg-white p-1 disabled:cursor-not-allowed disabled:opacity-40 dark:border-dark-600 dark:bg-dark-800"
            :disabled="!modelValue.trim()"
            @input="emit('update:color', ($event.target as HTMLInputElement).value.toUpperCase())"
          />
          <input
            :id="`${name}-color-hex`"
            data-test="group-tag-color-hex"
            :value="color || resolvedColor"
            type="text"
            class="input min-h-11 w-28 font-mono uppercase"
            pattern="#[0-9a-fA-F]{6}"
            maxlength="7"
            :disabled="!modelValue.trim()"
            :aria-label="t('common.groupTags.hexColor')"
            placeholder="#0EA5E9"
            @input="emit('update:color', ($event.target as HTMLInputElement).value.toUpperCase())"
          />
        </div>
      </div>
    </div>
    <div class="flex flex-wrap gap-2">
      <label
        v-for="tag in tags"
        :key="tag"
        class="inline-flex min-h-10 cursor-pointer items-center gap-2 rounded-md border px-3 py-2 transition-colors"
        :class="modelValue === tag
          ? 'border-primary-500 bg-primary-50 dark:border-primary-400 dark:bg-primary-500/10'
          : 'border-gray-200 bg-white hover:border-gray-400 dark:border-dark-600 dark:bg-dark-800'"
      >
        <input
          type="radio"
          :name="name"
          :value="tag"
          :checked="modelValue === tag"
          :aria-label="t(`common.groupTags.${tag || 'none'}`)"
          class="h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500"
          @change="selectPreset(tag)"
        />
        <GroupTagBadge v-if="tag" :tag="tag" />
        <span v-else class="text-sm text-gray-600 dark:text-gray-300">{{ t('common.groupTags.none') }}</span>
      </label>
    </div>
    <div v-if="reusableTags.length" data-test="group-tag-reusable" class="mt-4 space-y-2">
      <div class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('common.groupTags.reusable') }}</div>
      <div class="flex flex-wrap gap-2">
        <label
          v-for="option in reusableTags"
          :key="option.tag"
          class="inline-flex min-h-10 cursor-pointer items-center gap-2 rounded-md border px-3 py-2 transition-colors"
          :class="modelValue === option.tag
            ? 'border-primary-500 bg-primary-50 dark:border-primary-400 dark:bg-primary-500/10'
            : 'border-gray-200 bg-white hover:border-gray-400 dark:border-dark-600 dark:bg-dark-800'"
        >
          <input
            data-test="group-tag-reusable-option"
            type="radio"
            :name="name"
            :value="option.tag"
            :checked="modelValue === option.tag"
            :aria-label="option.tag"
            class="h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500"
            @change="selectReusableTag(option)"
          />
          <GroupTagBadge :tag="option.tag" :color="option.color" />
          <span data-test="group-tag-reusable-count" class="text-[11px] text-gray-500 dark:text-gray-400">
            {{ t('common.groupTags.usage', { count: option.count }) }}
          </span>
        </label>
      </div>
    </div>
    <div class="mt-3 flex min-w-0 flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-center gap-1.5">
        <button
          v-for="swatch in GROUP_TAG_COLORS"
          :key="swatch"
          type="button"
          class="flex h-9 w-9 items-center justify-center rounded-md border border-transparent transition-colors hover:border-gray-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:opacity-40 dark:hover:border-dark-500"
          :aria-label="`${t('common.groupTags.color')} ${swatch}`"
          :title="swatch"
          :aria-pressed="resolvedColor === swatch"
          :disabled="!modelValue.trim()"
          @click="emit('update:color', swatch)"
        >
          <span class="flex h-6 w-6 items-center justify-center rounded-full border border-black/10" :style="groupTagStyle('', swatch)">
            <Icon v-if="resolvedColor === swatch" name="check" size="xs" />
          </span>
        </button>
      </div>
      <div data-test="group-tag-preview" class="relative flex h-11 min-w-40 items-center rounded-md border border-dashed border-gray-200 pl-3 pr-32 dark:border-dark-600">
        <span class="text-xs text-gray-400">{{ t('common.groupTags.preview') }}</span>
        <GroupTagBadge :tag="modelValue" :color="color" class="!absolute right-0 top-0" />
      </div>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import GroupTagBadge from '@/components/common/GroupTagBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import type { GroupTag } from '@/types'
import type { ReusableGroupTagOption } from '@/utils/groupTagOptions'
import { GROUP_TAG_COLORS, GROUP_TAG_PRESETS, groupTagColor, groupTagStyle, isGroupTagPreset } from '@/utils/groupTag'

const props = withDefaults(defineProps<{ modelValue: GroupTag; color?: string; name: string; reusableTags?: ReusableGroupTagOption[] }>(), {
  color: '',
  reusableTags: () => [],
})
const emit = defineEmits<{ 'update:modelValue': [tag: GroupTag]; 'update:color': [color: string] }>()
const { t } = useI18n()
const tags = ['', ...GROUP_TAG_PRESETS]
const displayText = computed(() => isGroupTagPreset(props.modelValue) ? t(`common.groupTags.${props.modelValue}`) : props.modelValue)
const resolvedColor = computed(() => groupTagColor(props.modelValue, props.color))
const updateText = (value: string) => {
  emit('update:modelValue', value)
  if (!value.trim()) emit('update:color', '')
}
const selectPreset = (tag: string) => {
  emit('update:modelValue', tag)
  emit('update:color', '')
}
const selectReusableTag = (option: ReusableGroupTagOption) => {
  emit('update:modelValue', option.tag)
  emit('update:color', option.color.toUpperCase())
}
</script>
