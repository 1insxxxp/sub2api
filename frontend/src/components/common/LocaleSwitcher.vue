<template>
  <div class="relative" ref="dropdownRef">
    <button
      @click="toggleDropdown"
      :disabled="switching"
      class="group flex min-w-[6.5rem] items-center gap-2 rounded-xl border border-slate-200/80 bg-white/80 px-2.5 py-1.5 text-sm font-semibold text-slate-600 shadow-sm shadow-slate-950/5 ring-1 ring-white/70 transition-colors duration-200 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500/20 disabled:cursor-not-allowed disabled:opacity-70 dark:border-white/10 dark:bg-white/[0.04] dark:text-slate-200 dark:ring-white/5 dark:hover:border-blue-400/25 dark:hover:bg-blue-500/10 dark:hover:text-blue-200"
      :title="currentLocale?.name"
      aria-haspopup="menu"
      :aria-expanded="isOpen"
    >
      <span class="flex h-6 w-6 items-center justify-center rounded-md bg-blue-50 text-[13px] text-slate-700 ring-1 ring-blue-100 dark:bg-blue-500/10 dark:text-slate-100 dark:ring-blue-400/20">{{ currentLocale?.flag }}</span>
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
        class="locale-menu-panel absolute right-0 z-50 mt-2 w-48 max-w-[calc(100vw-1rem)] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-lg shadow-slate-950/10 ring-1 ring-slate-950/5 dark:border-white/10 dark:bg-slate-950 dark:shadow-black/30 dark:ring-white/10"
        role="menu"
      >
        <div class="locale-menu-header flex items-center justify-between border-b border-slate-100 px-3 py-2 dark:border-white/10">
          <p class="text-[11px] font-medium text-slate-500 dark:text-slate-400">{{ t('admin.settings.emailTemplates.locale') }}</p>
          <p class="text-xs font-semibold text-slate-700 dark:text-slate-200">{{ currentLocale?.name }}</p>
        </div>
        <div class="space-y-0.5 p-1.5">
          <button
            v-for="locale in availableLocales"
            :key="locale.code"
            :disabled="switching"
            @click="selectLocale(locale.code)"
            class="locale-menu-item flex min-h-10 w-full items-center gap-2 rounded-lg px-2.5 py-2 text-sm font-medium text-slate-700 transition-colors duration-150 hover:bg-slate-50 hover:text-slate-950 dark:text-slate-200 dark:hover:bg-white/[0.06] dark:hover:text-white"
            :class="{
              'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200':
                locale.code === currentLocaleCode
            }"
            role="menuitemradio"
            :aria-checked="locale.code === currentLocaleCode"
          >
            <span class="flex h-6 w-6 items-center justify-center rounded-md bg-slate-100 text-[13px] text-slate-600 dark:bg-white/[0.06] dark:text-slate-200">{{ locale.flag }}</span>
            <span class="min-w-0 flex-1">
              <span class="block truncate">{{ locale.name }}</span>
              <span class="block text-[10px] font-medium uppercase tracking-[0.08em] text-slate-400 dark:text-slate-500">{{ locale.code }}</span>
            </span>
            <span
              class="flex h-5 w-5 items-center justify-center rounded-md transition-colors duration-150"
              :class="locale.code === currentLocaleCode
                ? 'text-blue-600 dark:text-blue-200'
                : 'text-transparent'"
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
    opacity 140ms ease,
    transform 140ms ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.98) translateY(-4px);
}
</style>
