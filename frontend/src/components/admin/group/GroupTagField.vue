<template>
  <fieldset>
    <legend class="input-label">{{ t('common.groupTags.label') }}</legend>
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
          @change="emit('update:modelValue', tag)"
        />
        <GroupTagBadge v-if="tag" :tag="tag" />
        <span v-else class="text-sm text-gray-600 dark:text-gray-300">{{ t('common.groupTags.none') }}</span>
      </label>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import GroupTagBadge from '@/components/common/GroupTagBadge.vue'
import type { GroupTag } from '@/types'

defineProps<{ modelValue: GroupTag; name: string }>()
const emit = defineEmits<{ 'update:modelValue': [tag: GroupTag] }>()
const { t } = useI18n()
const tags: GroupTag[] = ['', 'chat', 'image', 'airp']
</script>
