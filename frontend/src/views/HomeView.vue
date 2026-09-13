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
    <div class="home-minimal-grid pointer-events-none absolute inset-0" aria-hidden="true"></div>
    <div class="home-minimal-orb pointer-events-none absolute left-1/2 top-[18%] h-[30rem] w-[30rem] -translate-x-1/2 rounded-full" aria-hidden="true">
      <div class="home-orb-sphere">
        <div class="home-orb-surface"></div>
      </div>
      <div class="home-orb-orbit">
        <div class="home-orb-track"></div>
      </div>
      <div class="home-orb-orbit home-orb-orbit-cross">
        <div class="home-orb-track"></div>
      </div>
    </div>

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

    <main class="relative z-10">
      <section class="home-minimal-hero mx-auto flex min-h-[calc(100vh-8rem)] w-full max-w-4xl flex-col items-center justify-center px-4 pb-16 pt-28 text-center sm:px-6 sm:pb-20 sm:pt-32">
        <p class="home-minimal-reveal mb-6 inline-flex items-center gap-2 rounded-full border border-blue-200/80 bg-white/80 px-3.5 py-1.5 text-[11px] font-bold uppercase tracking-[0.22em] text-blue-700 shadow-sm shadow-blue-100/80 dark:border-blue-400/20 dark:bg-dark-900/70 dark:text-blue-300 dark:shadow-none">
          <span class="h-1.5 w-1.5 rounded-full bg-cyan-400 shadow-[0_0_0_4px_rgba(34,211,238,0.14)]"></span>
          {{ t('home.hero.eyebrow') }}
        </p>
        <h1 class="home-minimal-reveal max-w-4xl text-5xl font-semibold leading-[1.03] tracking-[-0.045em] sm:text-7xl lg:text-[5.9rem]" style="--motion-index: 1">
          <span class="block">{{ t('home.hero.titleLead') }}</span>
        </h1>
        <p class="home-minimal-reveal mt-7 max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-300 sm:text-xl sm:leading-9" style="--motion-index: 2">
          {{ siteSubtitle || t('home.hero.subtitle') }}
        </p>
        <div class="home-minimal-reveal mt-9 flex w-full flex-col justify-center gap-3 sm:w-auto sm:flex-row" style="--motion-index: 3">
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="home-minimal-primary inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-xl px-7 text-sm font-semibold text-white sm:w-auto"
          >
            {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.hero.primaryCta') }}
            <Icon name="arrowRight" size="sm" />
          </router-link>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex min-h-12 w-full items-center justify-center rounded-xl border border-slate-200 bg-white/80 px-7 text-sm font-semibold text-slate-700 transition-all hover:-translate-y-0.5 hover:border-blue-300 hover:bg-blue-50 sm:w-auto dark:border-dark-700 dark:bg-dark-900/70 dark:text-slate-100 dark:hover:border-blue-500/50 dark:hover:bg-blue-500/10"
          >
            {{ t('home.hero.secondaryCta') }}
          </a>
        </div>
        <div class="home-minimal-reveal mt-12 flex flex-wrap items-center justify-center gap-x-6 gap-y-3 text-xs font-medium text-slate-500 dark:text-slate-400" style="--motion-index: 4">
          <span v-for="item in valueItems" :key="item.title" class="home-minimal-proof inline-flex items-center gap-2">
            <span class="flex h-6 w-6 items-center justify-center rounded-full" :class="item.iconClass">
              <Icon :name="item.icon" size="xs" />
            </span>
            {{ item.title }}
          </span>
        </div>
        <div class="home-minimal-reveal mt-16 w-full max-w-3xl" style="--motion-index: 5">
          <div class="home-minimal-connection mx-auto flex max-w-2xl items-center justify-center gap-3 rounded-2xl border border-blue-100/90 bg-white/65 px-4 py-3 text-xs text-slate-500 shadow-lg shadow-blue-100/40 backdrop-blur-sm dark:border-blue-400/15 dark:bg-dark-900/60 dark:text-slate-400 dark:shadow-none sm:gap-4 sm:px-6">
            <span class="home-minimal-connection-dot h-2 w-2 shrink-0 rounded-full bg-cyan-400"></span>
            <span class="min-w-0 truncate font-mono">{{ apiEndpoint }}</span>
            <span class="h-4 w-px bg-slate-200 dark:bg-dark-700"></span>
            <span class="whitespace-nowrap">{{ t('home.integration.replaceBaseUrl') }}</span>
          </div>
        </div>
      </section>

      <section class="home-minimal-showcase home-scroll-reveal border-y border-slate-100/90 bg-white/55 px-4 py-20 dark:border-dark-800/80 dark:bg-dark-900/25 sm:px-6 lg:py-28">
        <div class="mx-auto max-w-6xl">
          <div class="max-w-2xl">
            <p class="text-xs font-bold uppercase tracking-[0.22em] text-blue-600 dark:text-blue-300">{{ t('home.sections.capabilitiesEyebrow') }}</p>
            <h2 class="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">{{ t('home.sections.capabilitiesTitle') }}</h2>
            <p class="mt-4 text-sm leading-7 text-slate-600 dark:text-slate-300 sm:text-base">{{ t('home.sections.capabilitiesSubtitle') }}</p>
          </div>
          <div class="mt-10 grid gap-4 md:grid-cols-3">
            <article v-for="(item, index) in valueItems" :key="item.title" class="home-minimal-card home-scroll-reveal rounded-2xl border border-slate-200/80 bg-white/75 p-6 shadow-sm shadow-blue-100/40 dark:border-dark-700 dark:bg-dark-900/60 dark:shadow-none" :style="{ '--motion-index': index }">
              <span class="flex h-10 w-10 items-center justify-center rounded-xl" :class="item.iconClass"><Icon :name="item.icon" size="sm" /></span>
              <h3 class="mt-5 text-base font-semibold">{{ item.title }}</h3>
              <p class="mt-2 text-sm leading-6 text-slate-500 dark:text-slate-400">{{ item.desc }}</p>
            </article>
          </div>
          <div class="home-minimal-steps home-scroll-reveal mt-5 grid gap-3 rounded-2xl border border-blue-100/90 bg-blue-50/50 p-4 dark:border-blue-500/15 dark:bg-blue-500/[0.05] sm:grid-cols-3 sm:p-5">
            <div v-for="(step, index) in workflowItems" :key="step.title" class="flex gap-3 rounded-xl p-3">
              <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-white text-xs font-bold text-blue-600 shadow-sm dark:bg-dark-900 dark:text-blue-300">{{ index + 1 }}</span>
              <div>
                <h3 class="text-sm font-semibold">{{ step.title }}</h3>
                <p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">{{ step.desc }}</p>
              </div>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer class="relative z-10 border-t border-slate-100 px-4 py-6 dark:border-dark-800 sm:px-6">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-2 text-xs text-slate-400 sm:flex-row">
        <p>&copy; {{ currentYear }} {{ siteName }}</p>
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="transition-colors hover:text-blue-600 dark:hover:text-blue-300">{{ t('home.docs') }}</a>
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
const siteSubtitle = computed(() => {
  const configured = appStore.cachedPublicSettings?.site_subtitle?.trim() || ''
  return configured && configured !== 'Subscription to API Conversion Platform'
    ? configured
    : t('home.heroSubtitle')
})
const siteLogo = computed(() => sanitizeUrl(appStore.effectiveSiteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const apiEndpoint = computed(() => {
  const configured = appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || ''
  const fallback = typeof window !== 'undefined' ? window.location.origin : ''
  const base = (configured || fallback).trim().replace(/\/+$/, '')
  if (base.endsWith('/v1')) return base
  if (base.endsWith('/api')) return `${base.slice(0, -4)}/v1`
  return `${base}/v1`
})
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const isHeaderScrolled = ref(false)
let revealObserver: IntersectionObserver | null = null

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

const workflowItems = computed(() => [
  { title: t('home.workflow.step1.title'), desc: t('home.workflow.step1.desc') },
  { title: t('home.workflow.step2.title'), desc: t('home.workflow.step2.desc') },
  { title: t('home.workflow.step3.title'), desc: t('home.workflow.step3.desc') },
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
  if ('IntersectionObserver' in window) {
    revealObserver = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add('is-visible')
            revealObserver?.unobserve(entry.target)
          }
        })
      },
      { threshold: 0.16, rootMargin: '0px 0px -8% 0px' },
    )
    document.querySelectorAll<HTMLElement>('.home-scroll-reveal').forEach((element) => {
      revealObserver?.observe(element)
    })
  } else {
    document.querySelectorAll<HTMLElement>('.home-scroll-reveal').forEach((element) => {
      element.classList.add('is-visible')
    })
  }
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', syncHeaderScrolled)
  revealObserver?.disconnect()
  revealObserver = null
})
</script>

<style scoped>
.home-minimal-root {
  --home-motion-ease: cubic-bezier(0.16, 1, 0.3, 1);
  position: relative;
  isolation: isolate;
}

.home-minimal-root::before,
.home-minimal-root::after {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 44rem;
  pointer-events: none;
  z-index: 0;
}

.home-minimal-root::before {
  background:
    radial-gradient(ellipse at 50% 8%, rgba(59, 130, 246, 0.16), transparent 56%),
    radial-gradient(circle at 8% 34%, rgba(6, 182, 212, 0.11), transparent 30%);
}

.home-minimal-root::after {
  background: radial-gradient(circle at 94% 18%, rgba(129, 140, 248, 0.11), transparent 28%);
  animation: home-spotlight-drift 16s ease-in-out infinite alternate;
}

.dark .home-minimal-root::before {
  background:
    radial-gradient(ellipse at 50% 8%, rgba(37, 99, 235, 0.24), transparent 56%),
    radial-gradient(circle at 8% 34%, rgba(6, 182, 212, 0.14), transparent 30%);
}

.dark .home-minimal-root::after {
  background: radial-gradient(circle at 94% 18%, rgba(99, 102, 241, 0.16), transparent 28%);
}

.home-minimal-grid {
  background-image:
    linear-gradient(rgba(37, 99, 235, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(37, 99, 235, 0.045) 1px, transparent 1px);
  --home-grid-size: 72px;
  background-size: var(--home-grid-size) var(--home-grid-size);
  animation: home-grid-pan 22s linear infinite;
  mask-image: linear-gradient(to bottom, black 0%, transparent 72%);
  opacity: 0.65;
}

.dark .home-minimal-grid {
  background-image:
    linear-gradient(rgba(96, 165, 250, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(96, 165, 250, 0.07) 1px, transparent 1px);
  opacity: 0.4;
}

.home-minimal-orb {
  transform: translate(-50%, -3%) scale(1);
  animation: home-orb-float 10s ease-in-out infinite alternate;
}

.home-orb-sphere {
  position: absolute;
  inset: 9%;
  overflow: hidden;
  border-radius: 50%;
  background: #e0f2fe;
  box-shadow: 0 0 64px rgba(56, 189, 248, 0.18);
}

.home-orb-surface {
  position: absolute;
  inset: -12%;
  border-radius: 50%;
  background: conic-gradient(
    from 25deg,
    #effaff 0deg,
    #60a5fa 75deg,
    #cffafe 125deg,
    #f0f9ff 185deg,
    #38bdf8 250deg,
    #bfdbfe 300deg,
    #effaff 360deg
  );
  opacity: 0.72;
  filter: blur(8px);
  animation: home-orb-spin 8s linear infinite;
}

.home-orb-sphere::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background:
    radial-gradient(circle at 32% 22%, rgba(255, 255, 255, 0.85), transparent 52%),
    radial-gradient(circle at 50% 45%, transparent 45%, rgba(37, 99, 235, 0.14) 85%, rgba(255, 255, 255, 0.75));
}

.home-orb-orbit {
  position: absolute;
  inset: -8%;
  transform: rotate(-24deg) scaleY(0.48);
}

.home-orb-orbit-cross {
  inset: -1%;
  transform: rotate(58deg) scaleY(0.62);
}

.home-orb-track {
  position: absolute;
  inset: 0;
  border: 1px solid rgba(59, 130, 246, 0.25);
  border-top: 2px solid rgba(14, 165, 233, 0.7);
  border-radius: 50%;
  animation: home-orb-spin 6s linear infinite;
}

.home-orb-track::before {
  content: '';
  position: absolute;
  top: 0;
  left: 50%;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #0ea5e9;
  box-shadow: 0 0 0 5px rgba(56, 189, 248, 0.14), 0 0 22px rgba(14, 165, 233, 0.65);
  transform: translate(-50%, -50%);
}

.home-orb-orbit-cross .home-orb-track {
  border-top-color: rgba(6, 182, 212, 0.7);
  animation-duration: 10s;
  animation-direction: reverse;
}

.home-orb-orbit-cross .home-orb-track::before {
  background: #06b6d4;
}

.dark .home-orb-sphere {
  background: #0c2449;
  box-shadow: 0 0 64px rgba(37, 99, 235, 0.15);
}

.dark .home-orb-surface {
  opacity: 0.4;
}

.dark .home-orb-sphere::after {
  background: radial-gradient(circle at 32% 22%, rgba(147, 197, 253, 0.14), transparent 48%, rgba(2, 6, 23, 0.6));
}

.home-minimal-header {
  background: rgba(255, 255, 255, 0.68);
  backdrop-filter: blur(16px);
}

.dark .home-minimal-header {
  background: rgba(2, 6, 23, 0.68);
}

.home-minimal-header-scrolled {
  border-color: rgba(147, 197, 253, 0.4);
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.07);
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

.home-minimal-proof {
  transition: color 180ms ease, transform 180ms ease;
}

.home-minimal-proof:hover {
  color: #2563eb;
  transform: translateY(-2px);
}

.home-minimal-connection {
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease;
}

.home-minimal-connection:hover {
  border-color: rgba(96, 165, 250, 0.6);
  box-shadow: 0 16px 34px rgba(37, 99, 235, 0.1);
  transform: translateY(-2px);
}

.home-minimal-connection-dot {
  box-shadow: 0 0 0 5px rgba(34, 211, 238, 0.13);
  animation: home-status-glow 2.4s ease-out infinite;
}

.home-minimal-card {
  transition: transform 220ms ease, border-color 220ms ease, box-shadow 220ms ease;
}

.home-minimal-card:hover {
  transform: translateY(-6px);
  border-color: rgba(96, 165, 250, 0.65);
  box-shadow: 0 18px 36px rgba(37, 99, 235, 0.11);
}

.home-minimal-steps {
  animation: home-steps-breathe 8s ease-in-out infinite paused;
}

.home-scroll-reveal {
  opacity: 0;
  transform: translateY(32px) scale(0.985);
  transition: opacity 700ms var(--home-motion-ease), transform 700ms var(--home-motion-ease);
  transition-delay: calc(var(--motion-index, 0) * 100ms);
}

.home-scroll-reveal.is-visible {
  opacity: 1;
  transform: translateY(0) scale(1);
}

.home-minimal-steps.is-visible {
  animation-play-state: running;
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

@keyframes home-grid-pan {
  from { background-position: 0 0, 0 0; }
  to { background-position: var(--home-grid-size) var(--home-grid-size), calc(var(--home-grid-size) * -1) var(--home-grid-size); }
}

@keyframes home-spotlight-drift {
  from { transform: translate3d(-2%, -1%, 0); }
  to { transform: translate3d(2%, 1%, 0); }
}

@keyframes home-orb-float {
  from { transform: translate(-50%, -3%) scale(0.98); }
  to { transform: translate(-50%, 3%) scale(1.02); }
}

@keyframes home-orb-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes home-status-glow {
  70% { box-shadow: 0 0 0 10px rgba(34, 211, 238, 0); }
  100% { box-shadow: 0 0 0 0 rgba(34, 211, 238, 0); }
}

@keyframes home-steps-breathe {
  0%, 100% { box-shadow: 0 0 0 rgba(59, 130, 246, 0); }
  50% { box-shadow: 0 14px 40px rgba(59, 130, 246, 0.08); }
}

@media (max-width: 640px) {
  .home-minimal-header {
    padding-top: 0.65rem;
    padding-bottom: 0.65rem;
  }

  .home-minimal-orb {
    top: 22%;
    height: 21rem;
    width: 21rem;
  }

  .home-minimal-grid {
    --home-grid-size: 48px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-minimal-grid,
  .home-minimal-root::after,
  .home-minimal-orb,
  .home-orb-surface,
  .home-orb-track,
  .home-orb-orbit-cross .home-orb-track,
  .home-minimal-reveal,
  .home-minimal-connection-dot {
    animation: none;
  }

  .home-minimal-reveal {
    opacity: 1;
  }

  .home-minimal-primary,
  .home-minimal-proof,
  .home-minimal-connection,
  .home-minimal-card,
  .home-minimal-steps,
  .home-scroll-reveal {
    transition: none;
  }

  .home-scroll-reveal {
    opacity: 1;
    transform: none;
  }

  .home-minimal-steps {
    animation: none;
  }
}
</style>
