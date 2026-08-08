<template>
  <div class="xl:hidden">
    <button
      ref="triggerRef"
      type="button"
      data-testid="channel-picker-trigger"
      class="flex min-h-11 w-full min-w-0 items-center justify-between gap-3 rounded-2xl border border-gray-200 bg-white px-4 py-3 text-left shadow-sm transition-colors motion-reduce:transition-none dark:border-dark-600 dark:bg-dark-800 xl:hidden"
      :aria-expanded="open"
      aria-haspopup="dialog"
      @click="openPicker"
    >
      <span class="min-w-0">
        <span class="block text-xs font-medium text-gray-500 dark:text-gray-400">{{ t(`${catalogKey}.selectChannel`) }}</span>
        <span class="mt-0.5 block truncate text-sm font-semibold text-gray-900 dark:text-white">{{ selectedChannel?.name }}</span>
        <span v-if="selectedChannel" class="mt-1 flex flex-wrap gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ selectedChannel.platforms.join(' · ') }}</span>
          <span>{{ t(`${catalogKey}.groupsCount`, { count: selectedChannel.groupCount }) }}</span>
          <span>{{ t(`${catalogKey}.modelsCount`, { count: selectedChannel.modelCount }) }}</span>
        </span>
      </span>
      <svg class="h-5 w-5 shrink-0 text-gray-400" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m6 8 4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" /></svg>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        data-testid="channel-picker-dialog"
        class="fixed inset-0 z-[70] flex items-end bg-black/45"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        @click.self="closePicker"
      >
        <section data-testid="channel-picker-panel" class="flex max-h-[calc(100dvh-3rem)] w-full flex-col rounded-t-3xl bg-white pb-[max(1rem,env(safe-area-inset-bottom))] shadow-2xl dark:bg-dark-800">
          <header data-testid="channel-picker-header" class="sticky top-0 z-10 rounded-t-3xl border-b border-gray-100 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
            <div class="flex items-center justify-between gap-3">
              <h2 :id="titleId" class="text-base font-semibold text-gray-900 dark:text-white">{{ t(`${catalogKey}.channelPickerTitle`) }}</h2>
              <button type="button" data-testid="channel-picker-close" class="flex h-11 w-11 items-center justify-center rounded-xl text-gray-500 transition-colors motion-reduce:transition-none hover:bg-gray-100 dark:hover:bg-dark-700" :aria-label="t('common.close')" @click="closePicker">×</button>
            </div>
            <input
              ref="searchRef"
              v-model="query"
              data-testid="channel-picker-search"
              type="search"
              class="mt-3 h-11 w-full rounded-xl border border-gray-200 bg-gray-50 px-3 text-sm text-gray-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:text-white"
              :placeholder="t(`${catalogKey}.channelPickerSearch`)"
            >
          </header>
          <div data-testid="channel-picker-options" class="min-h-0 flex-1 overflow-y-auto p-3" role="listbox" :aria-label="t(`${catalogKey}.channelPickerTitle`)">
            <p v-if="filteredChannels.length === 0" class="px-3 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t(`${catalogKey}.channelPickerNoResults`) }}</p>
            <button
              v-for="channel in filteredChannels"
              :key="channel.key"
              type="button"
              role="option"
              data-testid="channel-picker-option"
              class="mb-2 min-h-11 w-full rounded-xl border px-3 py-3 text-left transition-colors motion-reduce:transition-none"
              :class="channel.key === modelValue ? 'border-primary-300 bg-primary-50 dark:border-primary-500/40 dark:bg-primary-500/10' : 'border-gray-200 dark:border-dark-600'"
              :aria-selected="channel.key === modelValue"
              @click="select(channel.key)"
            >
              <span class="block font-semibold text-gray-900 dark:text-white">{{ channel.name }}</span>
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ channel.platforms.join(' · ') }}</span>
              <span class="mt-1 flex gap-3 text-xs text-gray-500 dark:text-gray-400"><span>{{ t(`${catalogKey}.groupsCount`, { count: channel.groupCount }) }}</span><span>{{ t(`${catalogKey}.modelsCount`, { count: channel.modelCount }) }}</span></span>
            </button>
          </div>
        </section>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CatalogChannelEntry } from './availableChannelCatalog'

const catalogKey = 'availableChannels.catalog'
const props = defineProps<{ channels: CatalogChannelEntry[]; modelValue: string | null }>()
const emit = defineEmits<{ 'update:modelValue': [key: string] }>()
const { t } = useI18n()
const open = ref(false)
const query = ref('')
const triggerRef = ref<HTMLButtonElement | null>(null)
const searchRef = ref<HTMLInputElement | null>(null)
const previousOverflow = ref('')
const openedSelectionKey = ref<string | null>(null)
const titleId = `available-channel-picker-${getCurrentInstance()?.uid ?? 0}`
const selectedChannel = computed(() => props.channels.find(channel => channel.key === props.modelValue) ?? null)
const filteredChannels = computed(() => {
  const term = query.value.trim().toLocaleLowerCase()
  if (!term) return props.channels
  return props.channels.filter(channel => `${channel.name} ${channel.platforms.join(' ')}`.toLocaleLowerCase().includes(term))
})

function onKeydown(event: KeyboardEvent) { if (event.key === 'Escape') closePicker() }
async function openPicker() {
  query.value = ''
  openedSelectionKey.value = props.modelValue
  previousOverflow.value = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  open.value = true
  window.addEventListener('keydown', onKeydown)
  await nextTick()
  searchRef.value?.focus()
}
async function closePicker() {
  if (!open.value) return
  open.value = false
  query.value = ''
  document.body.style.overflow = previousOverflow.value
  window.removeEventListener('keydown', onKeydown)
  await nextTick()
  triggerRef.value?.focus()
}
function select(key: string) { emit('update:modelValue', key); void closePicker() }

watch(() => props.channels.map(channel => channel.key), keys => {
  if (open.value && (keys.length === 0 || (openedSelectionKey.value != null && !keys.includes(openedSelectionKey.value)))) void closePicker()
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  if (open.value) document.body.style.overflow = previousOverflow.value
})
</script>
