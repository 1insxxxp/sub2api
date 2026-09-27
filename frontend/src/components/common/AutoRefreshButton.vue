<template>
  <div class="relative" ref="dropdownRef">
    <button
      type="button"
      data-test="auto-refresh-trigger"
      @click="showDropdown = !showDropdown"
      :class="isAdminVariant
        ? 'btn btn-secondary dashboard-auto-refresh-action'
        : 'inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'"
      :title="t('common.autoRefresh.title')"
      :aria-label="t('common.autoRefresh.title')"
      :aria-expanded="showDropdown"
    >
      <Icon v-if="isAdminVariant" name="clock" size="sm" :class="{ 'text-primary-500': enabled }" />
      <svg
        v-else
        class="h-3.5 w-3.5"
        :class="enabled ? 'animate-spin' : ''"
        xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor"
      >
        <path fill-rule="evenodd" d="M15.312 11.424a5.5 5.5 0 01-9.201 2.466l-.312-.311h2.433a.75.75 0 000-1.5H4.598a.75.75 0 00-.75.75v3.634a.75.75 0 001.5 0v-2.033l.312.312a7 7 0 0011.712-3.138.75.75 0 00-1.449-.39zm-10.624-2.848a5.5 5.5 0 019.201-2.466l.312.311H11.768a.75.75 0 000 1.5h3.634a.75.75 0 00.75-.75V3.537a.75.75 0 00-1.5 0v2.034l-.312-.312A7 7 0 002.628 8.397a.75.75 0 001.449.39z" clip-rule="evenodd" />
      </svg>
      <span :class="{ 'hidden xl:inline': isAdminVariant }">
        {{ enabled
          ? t('common.autoRefresh.countdown', { seconds: countdown })
          : t('common.autoRefresh.title')
        }}
      </span>
    </button>

    <div
      v-if="showDropdown"
      data-test="auto-refresh-menu"
      :class="isAdminVariant
        ? 'admin-action-menu absolute right-0 z-50 mt-2 w-56 origin-top-right'
        : 'absolute right-0 z-20 mt-1 w-44 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800'"
    >
      <div :class="{ 'p-1.5': !isAdminVariant }">
        <button
          type="button"
          data-test="auto-refresh-toggle"
          @click="$emit('update:enabled', !enabled)"
          :class="isAdminVariant
            ? 'admin-action-menu-item justify-between'
            : 'flex w-full items-center justify-between rounded-md px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700'"
        >
          <span>{{ t('common.autoRefresh.enable') }}</span>
          <Icon v-if="isAdminVariant && enabled" name="check" size="sm" class="text-primary-500" />
          <svg v-else-if="enabled" class="h-4 w-4 text-primary-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M16.704 4.153a.75 0 01.143 1.052l-8 10.5a.75 1 0 01-1.127.075l-4.5-4.5a.75.75 0 011.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 011.05-.143z" clip-rule="evenodd" />
          </svg>
        </button>
        <div :class="isAdminVariant ? 'admin-action-menu-divider' : 'my-1 border-t border-gray-100 dark:border-dark-700'"></div>
        <button
          v-for="sec in intervals"
          :key="sec"
          type="button"
          data-test="auto-refresh-interval"
          :data-interval="sec"
          @click="$emit('update:interval', sec)"
          :class="isAdminVariant
            ? 'admin-action-menu-item justify-between'
            : 'flex w-full items-center justify-between rounded-md px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-dark-700'"
        >
          <span>{{ t('common.autoRefresh.seconds', { n: sec }) }}</span>
          <Icon v-if="isAdminVariant && intervalSeconds === sec" name="check" size="sm" class="text-primary-500" />
          <svg v-else-if="intervalSeconds === sec" class="h-4 w-4 text-primary-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M16.704 4.153a.75 0 01.143 1.052l-8 10.5a.75 1 0 01-1.127.075l-4.5-4.5a.75 1 0 011.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 011.05-.143z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  enabled: boolean
  intervalSeconds: number
  countdown: number
  intervals: readonly number[]
  variant?: 'default' | 'admin'
}>(), {
  variant: 'default'
})

defineEmits<{
  (e: 'update:enabled', value: boolean): void
  (e: 'update:interval', value: number): void
}>()

const { t } = useI18n()
const isAdminVariant = computed(() => props.variant === 'admin')
const showDropdown = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    showDropdown.value = false
  }
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))
</script>

<style scoped>
.dashboard-auto-refresh-action {
  width: 2.25rem;
  min-height: 2.25rem;
  padding: 0;
  gap: 0.375rem;
}

@media (min-width: 1280px) {
  .dashboard-auto-refresh-action {
    width: auto;
    padding-inline: 0.625rem;
  }
}
</style>
