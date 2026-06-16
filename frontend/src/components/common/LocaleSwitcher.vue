<template>
  <div class="relative" ref="dropdownRef">
    <button
      @click="toggleDropdown"
      :disabled="switching"
      class="group flex min-w-[7.25rem] items-center gap-2 rounded-2xl border border-slate-200/75 bg-white/72 px-3 py-2 text-sm font-semibold text-slate-600 shadow-sm shadow-blue-600/5 ring-1 ring-white/70 transition-all duration-200 hover:border-blue-200/80 hover:bg-blue-50/80 hover:text-blue-700 hover:shadow-brand-soft focus:outline-none focus:ring-2 focus:ring-blue-500/25 disabled:cursor-not-allowed disabled:opacity-70 dark:border-blue-400/15 dark:bg-white/[0.04] dark:text-slate-200 dark:ring-white/5 dark:hover:border-blue-400/25 dark:hover:bg-blue-500/10 dark:hover:text-blue-200"
      :title="currentLocale?.name"
      aria-haspopup="menu"
      :aria-expanded="isOpen"
    >
      <span class="flex h-7 w-7 items-center justify-center rounded-xl bg-[linear-gradient(135deg,#2563eb,#3b82f6,#06b6d4)] text-[13px] text-white shadow-sm shadow-blue-600/25">{{ currentLocale?.flag }}</span>
      <span class="min-w-0 flex-1 truncate text-left">
        <span class="hidden sm:inline">{{ currentLocale?.code.toUpperCase() }}</span>
        <span class="sm:hidden">{{ currentLocale?.code.toUpperCase() }}</span>
      </span>
      <Icon
        name="chevronDown"
        size="xs"
        class="text-slate-400 transition-transform duration-200 group-hover:text-blue-500"
        :class="{ 'rotate-180 text-blue-500': isOpen }"
      />
    </button>

    <transition name="dropdown">
      <div
        v-if="isOpen"
        class="brand-floating-panel absolute right-0 z-50 mt-3 w-[14.5rem] max-w-[calc(100vw-1.5rem)] overflow-hidden"
        role="menu"
      >
        <div class="brand-floating-header px-4 py-3">
          <p class="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-500 dark:text-slate-400">{{ t('admin.settings.emailTemplates.locale') }}</p>
          <p class="mt-1 text-sm font-semibold text-slate-900 dark:text-white">{{ currentLocale?.name }}</p>
        </div>
        <div class="p-2">
          <button
            v-for="locale in availableLocales"
            :key="locale.code"
            :disabled="switching"
            @click="selectLocale(locale.code)"
            class="flex min-h-[48px] w-full items-center gap-3 rounded-2xl px-3 py-2.5 text-sm font-semibold text-slate-700 transition-all duration-200 hover:bg-blue-50/85 hover:text-blue-700 hover:shadow-sm dark:text-slate-200 dark:hover:bg-blue-500/10 dark:hover:text-blue-200"
            :class="{
              'bg-[linear-gradient(135deg,rgba(239,246,255,0.95),rgba(236,254,255,0.88))] text-blue-700 shadow-sm shadow-blue-600/10 dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(6,182,212,0.12))] dark:text-blue-200':
                locale.code === currentLocaleCode
            }"
            role="menuitemradio"
            :aria-checked="locale.code === currentLocaleCode"
          >
            <span class="flex h-8 w-8 items-center justify-center rounded-xl bg-slate-100 text-sm text-slate-600 shadow-inner shadow-white/80 dark:bg-white/[0.06] dark:text-slate-200">{{ locale.flag }}</span>
            <span class="min-w-0 flex-1">
              <span class="block truncate">{{ locale.name }}</span>
              <span class="block text-[11px] font-medium uppercase tracking-[0.12em] text-slate-400 dark:text-slate-500">{{ locale.code }}</span>
            </span>
            <span
              class="flex h-7 w-7 items-center justify-center rounded-full border border-transparent transition-all duration-200"
              :class="locale.code === currentLocaleCode
                ? 'border-blue-200 bg-white/80 text-blue-600 shadow-sm shadow-blue-600/10 dark:border-blue-400/20 dark:bg-white/10 dark:text-blue-200'
                : 'text-slate-300 dark:text-slate-600'"
            >
              <Icon v-if="locale.code === currentLocaleCode" name="check" size="sm" />
            </span>
          </button>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { setLocale, availableLocales } from '@/i18n'

const { locale, t } = useI18n()

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const switching = ref(false)

const currentLocaleCode = computed(() => locale.value)
const currentLocale = computed(() => availableLocales.find((l) => l.code === locale.value))

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

async function selectLocale(code: string) {
  if (switching.value || code === currentLocaleCode.value) {
    isOpen.value = false
    return
  }
  switching.value = true
  try {
    await setLocale(code)
    isOpen.value = false
  } finally {
    switching.value = false
  }
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition:
    opacity 180ms ease,
    transform 180ms ease,
    filter 180ms ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  filter: blur(4px);
  transform: scale(0.96) translateY(-6px);
}
</style>
