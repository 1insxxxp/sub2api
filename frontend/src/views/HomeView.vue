<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>

    <!-- 微信客服悬浮按钮 -->
    <WechatServiceButton />
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="home-minimal-root min-h-screen overflow-x-hidden bg-white text-slate-950 dark:bg-dark-950 dark:text-white"
  >
    <div class="home-minimal-ambient pointer-events-none absolute inset-x-0 top-0 h-[32rem]"></div>

    <header
      class="home-minimal-header fixed inset-x-0 top-0 z-30 border-b border-transparent px-4 py-3 transition-all duration-300 sm:px-6"
      :class="{ 'home-minimal-header-scrolled': isHeaderScrolled }"
    >
      <nav class="mx-auto flex h-12 w-full max-w-6xl items-center justify-between gap-3">
        <router-link to="/" class="group flex min-w-0 items-center gap-2.5">
          <span class="home-minimal-logo flex h-10 w-10 shrink-0 items-center justify-center rounded-xl p-0.5">
            <span class="flex h-full w-full items-center justify-center overflow-hidden rounded-[0.6rem] bg-white dark:bg-dark-950">
              <img :src="siteLogo || '/logo.png'" :alt="siteName" class="h-full w-full object-contain" />
            </span>
          </span>
          <span class="hidden min-w-0 truncate text-sm font-semibold sm:block">{{ siteName }}</span>
        </router-link>

        <div class="hidden items-center gap-1 md:flex">
          <a
            v-for="item in navItems"
            :key="item.href"
            :href="item.href"
            class="rounded-lg px-3 py-2 text-sm font-medium text-slate-500 transition-colors hover:bg-blue-50 hover:text-blue-700 dark:text-slate-300 dark:hover:bg-blue-500/10 dark:hover:text-blue-200"
          >
            {{ item.label }}
          </a>
        </div>

        <div class="flex items-center gap-1.5 sm:gap-2">
          <LocaleSwitcher />
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="hidden h-10 w-10 items-center justify-center rounded-lg text-slate-500 transition-colors hover:bg-blue-50 hover:text-blue-700 dark:text-slate-300 dark:hover:bg-blue-500/10 dark:hover:text-blue-200 sm:inline-flex"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="sm" />
          </router-link>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="hidden h-10 w-10 items-center justify-center rounded-lg text-slate-500 transition-colors hover:bg-blue-50 hover:text-blue-700 dark:text-slate-300 dark:hover:bg-blue-500/10 dark:hover:text-blue-200 sm:inline-flex"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="sm" />
          </a>
          <button
            type="button"
            class="flex h-10 w-10 items-center justify-center rounded-lg text-slate-500 transition-colors hover:bg-blue-50 hover:text-blue-700 dark:text-slate-300 dark:hover:bg-blue-500/10 dark:hover:text-blue-200"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="home-minimal-login inline-flex min-h-10 items-center justify-center rounded-lg px-3.5 text-sm font-semibold text-white transition-all hover:-translate-y-0.5 sm:px-4"
          >
            {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>
    <div class="h-[4.5rem]" aria-hidden="true"></div>

    <main class="relative z-10">
      <section class="mx-auto grid min-h-[calc(100vh-4.5rem)] max-w-6xl items-center gap-12 px-4 py-14 sm:px-6 sm:py-20 lg:grid-cols-[1.05fr,0.95fr] lg:gap-20 lg:px-8 lg:py-24">
        <div class="home-minimal-reveal min-w-0">
          <p class="mb-5 flex items-center gap-3 text-xs font-bold uppercase tracking-[0.24em] text-blue-700 dark:text-blue-300">
            <span class="h-px w-8 bg-gradient-to-r from-blue-600 to-cyan-400"></span>
            {{ t('home.hero.eyebrow') }}
          </p>
          <h1 class="max-w-3xl text-4xl font-semibold leading-[1.08] tracking-tight sm:text-6xl lg:text-[4.35rem]">
            <span class="block">{{ t('home.hero.titleLead') }}</span>
            <span class="mt-2 block bg-gradient-to-r from-blue-700 via-blue-600 to-cyan-500 bg-clip-text text-transparent dark:from-blue-300 dark:via-blue-200 dark:to-cyan-300">
              {{ t('home.hero.titleAccent') }}
            </span>
          </h1>
          <p class="mt-6 max-w-xl text-base leading-8 text-slate-600 dark:text-slate-300 sm:text-lg">
            {{ siteSubtitle || t('home.hero.subtitle') }}
          </p>
          <div class="mt-8 flex flex-col gap-3 sm:flex-row">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="home-minimal-primary inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-xl px-6 text-sm font-semibold text-white sm:w-auto"
            >
              {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.hero.primaryCta') }}
              <Icon name="arrowRight" size="sm" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-slate-200 bg-white px-6 text-sm font-semibold text-slate-700 transition-colors hover:border-blue-300 hover:bg-blue-50 sm:w-auto dark:border-dark-700 dark:bg-dark-900 dark:text-slate-100 dark:hover:border-blue-500/50 dark:hover:bg-blue-500/10"
            >
              {{ t('home.hero.secondaryCta') }}
            </a>
          </div>
          <div class="mt-8 grid max-w-xl gap-3 sm:grid-cols-3">
            <div v-for="(item, index) in valueItems" :key="item.title" class="home-minimal-value home-minimal-reveal rounded-xl border border-blue-100 bg-white/75 p-3.5 dark:border-blue-500/20 dark:bg-dark-900/70" :style="{ '--motion-index': index }">
              <span class="mb-2 flex h-8 w-8 items-center justify-center rounded-lg" :class="item.iconClass">
                <Icon :name="item.icon" size="xs" />
              </span>
              <h2 class="text-sm font-semibold">{{ item.title }}</h2>
              <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ item.desc }}</p>
            </div>
          </div>
        </div>

        <div class="home-minimal-visual home-minimal-reveal relative mx-auto w-full max-w-md lg:max-w-none" aria-label="API access preview">
          <div class="home-minimal-orbit absolute -inset-5 rounded-[2rem] border border-blue-200/50 dark:border-blue-500/20"></div>
          <div class="relative overflow-hidden rounded-2xl border border-slate-200 bg-slate-950 p-5 text-white shadow-2xl shadow-blue-900/15 dark:border-blue-500/20">
            <div class="mb-8 flex items-center justify-between">
              <div>
                <p class="text-xs font-medium uppercase tracking-[0.2em] text-cyan-300">{{ t('home.integration.eyebrow') }}</p>
                <p class="mt-2 text-lg font-semibold">{{ t('home.integration.title') }}</p>
              </div>
              <span class="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-cyan-400 shadow-lg shadow-cyan-500/20">
                <Icon name="terminal" size="sm" />
              </span>
            </div>
            <div class="space-y-3 rounded-xl border border-white/10 bg-white/[0.04] p-4 font-mono text-xs leading-6 text-slate-300">
              <p><span class="text-cyan-300">base_url</span> = <span class="text-emerald-300">"{{ apiBaseUrl }}/v1"</span></p>
              <p><span class="text-cyan-300">model</span> = <span class="text-emerald-300">"your-model"</span></p>
              <p><span class="text-cyan-300">messages</span> = <span class="text-emerald-300">[{ role: "user", ... }]</span></p>
            </div>
            <div class="mt-5 flex items-center gap-2 text-sm text-slate-300">
              <span class="home-minimal-status h-2 w-2 rounded-full bg-cyan-300"></span>
              {{ t('home.integration.replaceBaseUrl') }}
            </div>
          </div>
        </div>
      </section>

      <section id="benefits" class="border-y border-blue-100 bg-blue-50/45 dark:border-blue-500/10 dark:bg-blue-500/[0.04]">
        <div class="mx-auto flex max-w-6xl flex-col gap-5 px-4 py-10 sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8">
          <div>
            <p class="text-sm font-semibold text-blue-700 dark:text-blue-300">{{ t('home.sections.capabilitiesEyebrow') }}</p>
            <h2 class="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{{ t('home.sections.capabilitiesTitle') }}</h2>
          </div>
          <p class="max-w-xl text-sm leading-7 text-slate-600 dark:text-slate-300">{{ t('home.sections.capabilitiesSubtitle') }}</p>
        </div>
      </section>

      <section id="quickstart" class="mx-auto max-w-6xl px-4 py-14 sm:px-6 lg:px-8 lg:py-20">
        <div class="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-8">
          <div class="flex flex-col gap-6 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <p class="text-sm font-semibold text-blue-700 dark:text-blue-300">{{ t('home.workflow.eyebrow') }}</p>
              <h2 class="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{{ t('home.workflow.title') }}</h2>
              <p class="mt-3 max-w-2xl text-sm leading-7 text-slate-600 dark:text-slate-300">{{ t('home.integration.subtitle') }}</p>
            </div>
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-minimal-primary inline-flex min-h-11 shrink-0 items-center justify-center rounded-xl px-5 text-sm font-semibold text-white">
              {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.hero.primaryCta') }}
            </router-link>
          </div>
        </div>
      </section>
    </main>

    <footer class="border-t border-slate-200 px-4 py-7 dark:border-dark-800 sm:px-6 lg:px-8">
      <div class="mx-auto flex max-w-6xl flex-col gap-3 text-sm text-slate-500 dark:text-slate-400 sm:flex-row sm:items-center sm:justify-between">
        <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
        <div class="flex flex-wrap items-center gap-4">
          <span>{{ t('home.footer.tagline') }}</span>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="font-medium text-slate-700 hover:text-blue-700 dark:text-slate-200 dark:hover:text-blue-300">{{ t('home.docs') }}</a>
        </div>
      </div>
    </footer>
  </div>
</template>


<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Passion')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const siteLogo = computed(() => sanitizeUrl(appStore.effectiveSiteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || '/api')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const isHeaderScrolled = ref(false)

const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

const currentYear = computed(() => new Date().getFullYear())

const navItems = computed(() => [
  { href: '#benefits', label: t('home.nav.features') },
  { href: '#quickstart', label: t('home.nav.integration') },
])

const valueItems = computed(() => [
  {
    title: t('home.capabilities.unifiedApi.title'),
    desc: t('home.capabilities.unifiedApi.desc'),
    icon: 'terminal' as const,
    iconClass: 'bg-slate-950 text-white dark:bg-white dark:text-slate-950',
  },
  {
    title: t('home.capabilities.accountPool.title'),
    desc: t('home.capabilities.accountPool.desc'),
    icon: 'swap' as const,
    iconClass: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200',
  },
  {
    title: t('home.capabilities.wallet.title'),
    desc: t('home.capabilities.wallet.desc'),
    icon: 'creditCard' as const,
    iconClass: 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200',
  },
])

function toggleTheme() {
  isDark.value = !isDark.value
  appStore.setTheme(isDark.value)
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
  isDark.value = shouldUseDark
  appStore.syncThemeFromDocument()
}

function syncHeaderScrolled() {
  isHeaderScrolled.value = window.scrollY > 8
}

onMounted(() => {
  initTheme()
  syncHeaderScrolled()
  window.addEventListener('scroll', syncHeaderScrolled, { passive: true })
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', syncHeaderScrolled)
})
</script>

<style scoped>
.home-minimal-root {
  --home-motion-ease: cubic-bezier(0.16, 1, 0.3, 1);
  position: relative;
  isolation: isolate;
}

.home-minimal-ambient {
  background:
    radial-gradient(circle at 18% 18%, rgba(37, 99, 235, 0.16), transparent 34%),
    radial-gradient(circle at 82% 10%, rgba(6, 182, 212, 0.14), transparent 30%),
    linear-gradient(180deg, rgba(239, 246, 255, 0.94), rgba(255, 255, 255, 0));
  animation: home-ambient-drift 18s ease-in-out infinite alternate;
}

.dark .home-minimal-ambient {
  background:
    radial-gradient(circle at 18% 18%, rgba(37, 99, 235, 0.22), transparent 34%),
    radial-gradient(circle at 82% 10%, rgba(6, 182, 212, 0.18), transparent 30%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.9), rgba(2, 6, 23, 0));
}

.home-minimal-header {
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(14px);
}

.dark .home-minimal-header {
  background: rgba(2, 6, 23, 0.72);
}

.home-minimal-header-scrolled {
  border-color: rgba(147, 197, 253, 0.42);
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.08);
}

.dark .home-minimal-header-scrolled {
  border-color: rgba(96, 165, 250, 0.18);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.18);
}

.home-minimal-logo {
  background: linear-gradient(135deg, #2563eb, #3b82f6 55%, #06b6d4);
  box-shadow: 0 8px 20px rgba(37, 99, 235, 0.2);
}

.home-minimal-login,
.home-minimal-primary {
  background: linear-gradient(100deg, #2563eb, #3b82f6 55%, #06b6d4);
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.2);
}

.home-minimal-login:hover,
.home-minimal-primary:hover {
  box-shadow: 0 14px 30px rgba(37, 99, 235, 0.3);
}

.home-minimal-primary {
  transition: transform 180ms ease, box-shadow 180ms ease;
}

.home-minimal-reveal {
  --motion-index: 0;
  animation: home-minimal-rise 760ms var(--home-motion-ease) both;
  animation-delay: calc(80ms + (var(--motion-index) * 90ms));
}

.home-minimal-value {
  transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
}

.home-minimal-value:hover {
  transform: translateY(-3px);
  border-color: rgba(96, 165, 250, 0.58);
  box-shadow: 0 12px 26px rgba(37, 99, 235, 0.1);
}

.home-minimal-visual {
  animation-delay: 220ms;
}

.home-minimal-orbit {
  animation: home-orbit-breathe 6s ease-in-out infinite;
}

.home-minimal-status {
  box-shadow: 0 0 0 0 rgba(103, 232, 249, 0.45);
  animation: home-status-glow 2.4s ease-out infinite;
}

@keyframes home-minimal-rise {
  from {
    opacity: 0;
    filter: blur(5px);
    transform: translateY(22px);
  }

  to {
    opacity: 1;
    filter: blur(0);
    transform: translateY(0);
  }
}

@keyframes home-ambient-drift {
  from { transform: translate3d(-1%, -1%, 0) scale(1); }
  to { transform: translate3d(1%, 1%, 0) scale(1.03); }
}

@keyframes home-orbit-breathe {
  0%, 100% { opacity: 0.45; transform: scale(0.98); }
  50% { opacity: 0.9; transform: scale(1.01); }
}

@keyframes home-status-glow {
  70% { box-shadow: 0 0 0 8px rgba(103, 232, 249, 0); }
  100% { box-shadow: 0 0 0 0 rgba(103, 232, 249, 0); }
}

@media (max-width: 640px) {
  .home-minimal-header {
    padding-top: 0.65rem;
    padding-bottom: 0.65rem;
  }

  .home-minimal-ambient {
    height: 24rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-minimal-ambient,
  .home-minimal-reveal,
  .home-minimal-orbit,
  .home-minimal-status {
    animation: none;
  }

  .home-minimal-reveal {
    opacity: 1;
  }

  .home-minimal-primary,
  .home-minimal-value {
    transition: none;
  }
}
</style>
