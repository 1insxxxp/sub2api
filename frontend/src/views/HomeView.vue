<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="home-motion-root theme-crisp min-h-screen overflow-x-hidden bg-white text-gray-950 dark:bg-dark-950 dark:text-white"
  >
    <div class="relative">
      <div
        class="home-grid-bg pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(37,99,235,0.07)_1px,transparent_1px),linear-gradient(90deg,rgba(37,99,235,0.06)_1px,transparent_1px)] bg-[size:48px_48px] dark:bg-[linear-gradient(rgba(96,165,250,0.1)_1px,transparent_1px),linear-gradient(90deg,rgba(6,182,212,0.07)_1px,transparent_1px)]"
      ></div>
      <div
        class="pointer-events-none absolute inset-x-0 top-0 h-[34rem] bg-[linear-gradient(180deg,rgba(219,234,254,0.92),rgba(255,255,255,0))] dark:bg-[linear-gradient(180deg,rgba(30,64,175,0.26),rgba(2,6,23,0))]"
      ></div>

      <header
        class="home-site-header home-header-flat fixed inset-x-0 top-0 z-30 border-b border-transparent px-3 py-2 backdrop-blur-xl transition-all duration-300 sm:px-4"
        :class="{ 'home-site-header-scrolled': isHeaderScrolled }"
      >
        <nav class="home-nav-shell home-nav-unified mx-auto flex h-16 max-w-7xl items-center justify-between gap-3 px-3 sm:px-4 lg:px-5">
          <router-link to="/" class="home-brand-link group flex min-w-0 items-center gap-3 px-1.5 py-1 transition-colors duration-200">
            <span
              class="home-brand-mark flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-[linear-gradient(135deg,#2563eb,#3b82f6,#06b6d4)] p-0.5 shadow-[0_8px_18px_rgba(37,99,235,0.16)] ring-1 ring-blue-300/35 transition-transform duration-200 group-hover:scale-[1.03] dark:ring-cyan-300/20"
            >
              <span class="flex h-full w-full items-center justify-center overflow-hidden rounded-[0.48rem] bg-white/95 dark:bg-dark-950/90">
                <img
                  :src="siteLogo || '/logo.png'"
                  :alt="siteName"
                  class="h-full w-full object-contain"
                />
              </span>
            </span>
            <span class="home-brand-name min-w-0 truncate text-base font-semibold text-slate-950 dark:text-white">
              {{ siteName }}
            </span>
          </router-link>

          <div class="home-nav-rail hidden items-center gap-1.5 lg:flex">
            <a
              v-for="item in navItems"
              :key="item.href"
              :href="item.href"
              class="home-nav-link px-3.5 py-2 text-sm font-semibold text-slate-600 transition-all duration-200 hover:text-blue-700 dark:text-slate-300 dark:hover:text-blue-100"
            >
              {{ item.label }}
            </a>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="home-nav-doc-link px-3.5 py-2 text-sm font-semibold text-slate-600 transition-all duration-200 hover:text-blue-700 dark:text-slate-300 dark:hover:text-blue-100"
            >
              {{ t('home.docs') }}
            </a>
          </div>

          <div class="home-header-actions flex items-center gap-1.5 sm:gap-2">
            <LocaleSwitcher class="home-locale-switcher" />

            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="home-icon-control hidden h-11 w-11 items-center justify-center rounded-xl text-slate-600 transition-all duration-200 hover:text-blue-700 dark:text-slate-300 dark:hover:text-blue-100 sm:inline-flex lg:hidden"
              :title="t('home.viewDocs')"
            >
              <Icon name="book" size="sm" />
            </a>

            <button
              type="button"
              class="home-icon-control flex h-11 w-11 items-center justify-center rounded-xl text-slate-600 transition-all duration-200 hover:text-blue-700 dark:text-slate-300 dark:hover:text-blue-100"
              :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
              @click="toggleTheme"
            >
              <Icon v-if="isDark" name="sun" size="sm" />
              <Icon v-else name="moon" size="sm" />
            </button>

            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="home-dashboard-cta inline-flex h-11 shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-xl bg-[linear-gradient(90deg,#2563eb,#3b82f6,#06b6d4)] px-3 text-sm font-semibold text-white shadow-[0_14px_28px_rgba(37,99,235,0.22)] ring-1 ring-white/25 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[0_16px_34px_rgba(37,99,235,0.34)] dark:text-white sm:px-5"
            >
              <span>
                {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.login') }}
              </span>
            </router-link>
          </div>
        </nav>
      </header>
      <div class="home-site-header-spacer h-[4.5rem]" aria-hidden="true"></div>

      <main class="relative z-10">
        <section class="mx-auto grid max-w-7xl items-center gap-10 px-4 pb-14 pt-12 sm:px-6 sm:pb-20 sm:pt-16 lg:grid-cols-[minmax(0,1fr),minmax(420px,0.9fr)] lg:gap-14 lg:px-8 lg:pb-24 lg:pt-24">
          <div class="min-w-0">
            <div
              class="home-hero-overline home-reveal home-reveal-1 mb-5 flex items-center gap-3 text-sm font-semibold text-blue-700 dark:text-blue-200"
            >
              <span class="h-px w-10 shrink-0 bg-[linear-gradient(90deg,#2563eb,#06b6d4)] dark:bg-[linear-gradient(90deg,#60a5fa,#67e8f9)]"></span>
              <span>{{ t('home.hero.eyebrow') }}</span>
            </div>

            <h1
              class="home-balanced-title home-reveal home-reveal-2 max-w-3xl break-words text-4xl font-semibold leading-[1.08] text-gray-950 dark:text-white sm:text-5xl lg:text-[3.7rem]"
            >
              <span class="block">{{ t('home.hero.titleLead') }}</span>
              <span
                class="block bg-[linear-gradient(90deg,#1d4ed8,#2563eb,#06b6d4)] bg-clip-text text-transparent dark:bg-[linear-gradient(90deg,#93c5fd,#60a5fa,#67e8f9)]"
              >
                {{ t('home.hero.titleAccent') }}
              </span>
            </h1>

            <div class="home-reveal home-reveal-3 mt-6 flex flex-wrap gap-2">
              <span
                v-for="item in heroProofItems"
                :key="item.label"
                class="home-proof-chip inline-flex min-h-10 items-center gap-2 rounded-lg px-2.5 pr-3 text-sm font-medium text-slate-700 backdrop-blur dark:text-slate-200"
              >
                <span class="home-proof-icon" :class="item.iconClass">
                  <Icon :name="item.icon" size="xs" />
                </span>
                <span class="min-w-0">{{ item.label }}</span>
              </span>
            </div>

            <div class="home-reveal home-reveal-4 mt-7 flex flex-col gap-3 sm:flex-row">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="home-action-button inline-flex h-11 items-center justify-center gap-2 rounded-lg bg-[linear-gradient(90deg,#2563eb,#3b82f6,#06b6d4)] px-5 text-sm font-semibold text-white shadow-[0_10px_26px_rgba(37,99,235,0.14)] transition-all hover:shadow-[0_12px_30px_rgba(37,99,235,0.28)] dark:text-white"
              >
                {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.hero.primaryCta') }}
                <Icon name="arrowRight" size="sm" />
              </router-link>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="home-action-button inline-flex h-11 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-5 text-sm font-semibold text-gray-800 transition-colors hover:border-primary-300 hover:bg-primary-50/60 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-100 dark:hover:border-primary-500/40 dark:hover:bg-primary-500/10"
              >
                <Icon name="book" size="sm" />
                {{ t('home.hero.secondaryCta') }}
              </a>
            </div>
          </div>

          <section
            class="home-reveal home-reveal-panel home-routing-panel relative min-w-0 rounded-lg border border-blue-400/30 bg-gray-950 p-4 text-white shadow-2xl shadow-blue-950/25 dark:border-blue-400/25"
            aria-labelledby="routing-panel-title"
          >
            <div class="flex items-start justify-between gap-4 border-b border-white/10 pb-4">
              <div class="min-w-0">
                <div class="mb-2 inline-flex items-center gap-2 rounded-lg bg-blue-400/10 px-2.5 py-1 text-xs font-semibold text-blue-100">
                  <span class="home-status-dot home-status-dot-live home-status-pulse"></span>
                  {{ t('home.hero.statusBadge') }}
                </div>
                <h2 id="routing-panel-title" class="text-lg font-semibold">
                  {{ t('home.hero.panelTitle') }}
                </h2>
                <p class="mt-1 text-sm leading-6 text-slate-400">
                  {{ t('home.hero.panelSubtitle') }}
                </p>
              </div>
              <span class="home-panel-icon">
                <Icon name="server" size="lg" />
              </span>
            </div>

            <div class="grid gap-3 py-4 sm:grid-cols-2">
              <div
                v-for="(channel, index) in channelRows"
                :key="channel.name"
                class="home-motion-card home-channel-card rounded-lg border border-white/10 bg-white/[0.04] p-3"
                :style="{ '--motion-index': index }"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="flex min-w-0 items-center gap-2.5">
                    <span class="home-channel-icon" :class="channel.iconClass">
                      <Icon :name="channel.icon" size="xs" />
                    </span>
                    <span class="min-w-0 truncate text-sm font-semibold">{{ channel.name }}</span>
                  </span>
                  <span
                    class="home-status-dot home-status-pulse shrink-0"
                    :class="channel.statusClass"
                  ></span>
                </div>
                <div class="mt-2 flex items-center justify-between text-xs text-slate-400">
                  <span>{{ channel.status }}</span>
                  <span>{{ channel.latency }}</span>
                </div>
              </div>
            </div>

            <div class="space-y-2 border-t border-white/10 pt-4">
              <div
                v-for="(row, index) in requestRows"
                :key="row.label"
                class="home-motion-card home-metric-row flex items-center justify-between gap-4 rounded-lg bg-white/[0.04] px-3 py-2 text-sm"
                :class="row.rowClass"
                :style="{ '--motion-index': index + 4 }"
              >
                <span class="flex min-w-0 items-center gap-2.5 text-slate-400">
                  <span class="home-metric-icon" :class="row.iconClass">
                    <Icon :name="row.icon" size="xs" />
                  </span>
                  <span class="truncate">{{ row.label }}</span>
                </span>
                <span class="min-w-0 truncate font-semibold" :class="row.valueClass">
                  {{ row.value }}
                </span>
              </div>
            </div>

            <pre
              class="home-code-panel mt-4 overflow-x-auto rounded-lg border border-white/10 bg-black/35 p-4 text-xs leading-6 text-slate-300"
            ><code>curl {{ apiBaseUrl }}/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -d '{ "model": "gpt-4.1-mini" }'</code></pre>
          </section>
        </section>

        <section class="home-trust-strip border-y border-blue-100/70 bg-white/55 backdrop-blur-sm dark:border-blue-400/10 dark:bg-slate-950/50">
          <div class="mx-auto grid max-w-7xl gap-x-8 gap-y-4 px-4 py-5 sm:grid-cols-2 sm:px-6 lg:grid-cols-4 lg:px-8">
            <div
              v-for="(item, index) in trustItems"
              :key="item.title"
              class="home-trust-item home-scroll-reveal flex min-w-0 gap-3 py-3"
              :style="{ '--motion-index': index }"
            >
              <div
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
                :class="item.iconClass"
              >
                <Icon :name="item.icon" size="sm" />
              </div>
              <div class="min-w-0">
                <h3 class="text-sm font-semibold text-gray-950 dark:text-white">{{ item.title }}</h3>
                <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ item.desc }}</p>
              </div>
            </div>
          </div>
        </section>

        <section id="features" class="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
          <div class="home-section-heading grid gap-5 lg:grid-cols-[minmax(0,0.9fr),minmax(320px,0.55fr)] lg:items-end">
            <div class="min-w-0">
              <p class="home-section-reveal home-section-reveal-1 text-sm font-semibold text-blue-700 dark:text-blue-300">
                {{ t('home.sections.capabilitiesEyebrow') }}
              </p>
              <h2 class="home-balanced-title home-section-reveal home-section-reveal-2 mt-3 max-w-2xl text-3xl font-semibold text-gray-950 dark:text-white sm:text-4xl">
                {{ t('home.sections.capabilitiesTitle') }}
              </h2>
            </div>
            <p class="home-copy-measure home-section-reveal home-section-reveal-3 text-base leading-8 text-gray-600 dark:text-dark-300 lg:pb-1">
              {{ t('home.sections.capabilitiesSubtitle') }}
            </p>
          </div>

          <div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <article
              v-for="(item, index) in capabilityItems"
              :key="item.title"
              class="home-motion-card home-scroll-reveal brand-surface p-5 transition-colors"
              :style="{ '--motion-index': index }"
            >
              <div
                class="mb-4 flex h-10 w-10 items-center justify-center rounded-lg"
                :class="item.iconClass"
              >
                <Icon :name="item.icon" size="sm" />
              </div>
              <h3 class="text-base font-semibold text-gray-950 dark:text-white">{{ item.title }}</h3>
              <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ item.desc }}</p>
            </article>
          </div>
        </section>

        <section id="integration" class="bg-gray-950 text-white dark:bg-black">
          <div class="mx-auto grid max-w-7xl gap-8 px-4 py-16 sm:px-6 lg:grid-cols-[0.95fr,1.05fr] lg:px-8 lg:py-24">
            <div class="min-w-0">
              <p class="home-section-reveal home-section-reveal-1 text-sm font-semibold text-cyan-300">{{ t('home.integration.eyebrow') }}</p>
              <h2 class="home-balanced-title home-section-reveal home-section-reveal-2 mt-3 max-w-xl text-3xl font-semibold sm:text-4xl">{{ t('home.integration.title') }}</h2>
              <p class="home-copy-measure home-section-reveal home-section-reveal-3 mt-4 max-w-xl text-base leading-8 text-slate-300">{{ t('home.integration.subtitle') }}</p>
              <div class="mt-8 grid gap-3">
                <div
                  v-for="(point, index) in integrationPoints"
                  :key="point"
                  class="home-motion-card home-scroll-reveal flex items-center gap-3 rounded-lg border border-white/10 bg-white/[0.04] px-4 py-3 text-sm font-semibold text-slate-100"
                  :style="{ '--motion-index': index }"
                >
                  <Icon name="check" size="sm" class="text-cyan-300" />
                  <span>{{ point }}</span>
                </div>
              </div>
            </div>

            <div class="home-code-panel home-scroll-reveal min-w-0 rounded-lg border border-white/10 bg-white/[0.04] p-4 shadow-xl">
              <div class="mb-4 flex items-center gap-2 border-b border-white/10 pb-3">
                <span class="h-3 w-3 rounded-full bg-red-400"></span>
                <span class="h-3 w-3 rounded-full bg-amber-300"></span>
                <span class="h-3 w-3 rounded-full bg-emerald-400"></span>
                <span class="ml-2 text-xs text-slate-400">openai-compatible.ts</span>
              </div>
              <pre class="overflow-x-auto text-sm leading-7 text-slate-300"><code>import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-your-platform-key",
  baseURL: "{{ apiBaseUrl }}/v1"
});

const completion = await client.chat.completions.create({
  model: "gpt-4.1-mini",
  messages: [{ role: "user", content: "Hello" }]
});</code></pre>
            </div>
          </div>
        </section>

        <section id="workflow" class="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8 lg:py-24">
          <div class="home-section-heading grid gap-5 lg:grid-cols-[minmax(0,0.9fr),minmax(320px,0.55fr)] lg:items-end">
            <div class="min-w-0">
              <p class="home-section-reveal home-section-reveal-1 text-sm font-semibold text-blue-700 dark:text-blue-300">
                {{ t('home.workflow.eyebrow') }}
              </p>
              <h2 class="home-balanced-title home-section-reveal home-section-reveal-2 mt-3 max-w-2xl text-3xl font-semibold text-gray-950 dark:text-white sm:text-4xl">
                {{ t('home.workflow.title') }}
              </h2>
            </div>
            <p class="home-copy-measure home-section-reveal home-section-reveal-3 text-base leading-8 text-gray-600 dark:text-dark-300 lg:pb-1">
              {{ t('home.workflow.subtitle') }}
            </p>
          </div>

          <div class="mt-10 grid gap-4 md:grid-cols-3">
            <article
              v-for="(step, index) in workflowSteps"
              :key="step.title"
              class="home-motion-card home-scroll-reveal brand-surface p-5"
              :style="{ '--motion-index': index }"
            >
              <div class="mb-5 flex h-9 w-9 items-center justify-center rounded-lg bg-gray-950 text-sm font-semibold text-white dark:bg-white dark:text-gray-950">
                {{ step.number }}
              </div>
              <h3 class="text-base font-semibold text-gray-950 dark:text-white">{{ step.title }}</h3>
              <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-300">{{ step.desc }}</p>
            </article>
          </div>
        </section>

        <section class="mx-auto max-w-7xl px-4 pb-16 sm:px-6 lg:px-8 lg:pb-24">
          <div
            class="home-cta-panel home-scroll-reveal rounded-lg border border-blue-400/20 bg-slate-950 px-5 py-8 text-center text-white shadow-xl shadow-blue-950/20 dark:border-blue-400/20 sm:px-8 lg:px-12"
          >
            <h2 class="home-section-reveal home-section-reveal-1 text-2xl font-semibold sm:text-3xl">{{ t('home.cta.title') }}</h2>
            <p class="home-section-reveal home-section-reveal-2 mx-auto mt-3 max-w-2xl text-sm leading-7 text-slate-300">
              {{ t('home.cta.subtitle') }}
            </p>
            <div class="mt-6 flex flex-col justify-center gap-3 sm:flex-row">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="inline-flex h-11 items-center justify-center rounded-lg bg-white px-5 text-sm font-semibold text-gray-950 transition-colors hover:bg-gray-200"
              >
                {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.hero.primaryCta') }}
              </router-link>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex h-11 items-center justify-center rounded-lg border border-white/15 px-5 text-sm font-semibold text-white transition-colors hover:bg-white/10"
              >
                {{ t('home.hero.secondaryCta') }}
              </a>
            </div>
          </div>
        </section>
      </main>

      <footer class="relative z-10 border-t border-gray-200 bg-white px-4 py-8 dark:border-dark-800 dark:bg-dark-950 sm:px-6 lg:px-8">
        <div class="mx-auto flex max-w-7xl flex-col gap-4 text-sm text-gray-500 dark:text-dark-400 sm:flex-row sm:items-center sm:justify-between">
          <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
          <div class="flex flex-wrap items-center gap-4">
            <span>{{ t('home.footer.tagline') }}</span>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="font-medium text-gray-700 transition-colors hover:text-gray-950 dark:text-dark-200 dark:hover:text-white"
            >
              {{ t('home.docs') }}
            </a>
          </div>
        </div>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Passion')
const siteLogo = computed(() => appStore.effectiveSiteLogo)
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || '/api')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const isHeaderScrolled = ref(false)

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

const currentYear = computed(() => new Date().getFullYear())

const navItems = computed(() => [
  { href: '#features', label: t('home.nav.features') },
  { href: '#integration', label: t('home.nav.integration') },
  { href: '#workflow', label: t('home.nav.workflow') },
])

const heroProofItems = computed(() => [
  {
    label: t('home.hero.proof.compatible'),
    icon: 'terminal' as const,
    iconClass: 'home-proof-icon-blue',
  },
  {
    label: t('home.hero.proof.routing'),
    icon: 'swap' as const,
    iconClass: 'home-proof-icon-cyan',
  },
  {
    label: t('home.hero.proof.billing'),
    icon: 'creditCard' as const,
    iconClass: 'home-proof-icon-amber',
  },
])

const trustItems = computed(() => [
  {
    title: t('home.trust.multiModel.title'),
    desc: t('home.trust.multiModel.desc'),
    icon: 'cube' as const,
    iconClass: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200',
  },
  {
    title: t('home.trust.routing.title'),
    desc: t('home.trust.routing.desc'),
    icon: 'swap' as const,
    iconClass: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200',
  },
  {
    title: t('home.trust.billing.title'),
    desc: t('home.trust.billing.desc'),
    icon: 'dollar' as const,
    iconClass: 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200',
  },
  {
    title: t('home.trust.monitoring.title'),
    desc: t('home.trust.monitoring.desc'),
    icon: 'shield' as const,
    iconClass: 'bg-slate-100 text-slate-700 dark:bg-slate-500/10 dark:text-slate-200',
  },
])

const capabilityItems = computed(() => [
  {
    title: t('home.capabilities.unifiedApi.title'),
    desc: t('home.capabilities.unifiedApi.desc'),
    icon: 'terminal' as const,
    iconClass: 'bg-gray-950 text-white dark:bg-white dark:text-gray-950',
  },
  {
    title: t('home.capabilities.accountPool.title'),
    desc: t('home.capabilities.accountPool.desc'),
    icon: 'users' as const,
    iconClass: 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200',
  },
  {
    title: t('home.capabilities.monitoring.title'),
    desc: t('home.capabilities.monitoring.desc'),
    icon: 'chart' as const,
    iconClass: 'bg-cyan-50 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-200',
  },
  {
    title: t('home.capabilities.wallet.title'),
    desc: t('home.capabilities.wallet.desc'),
    icon: 'creditCard' as const,
    iconClass: 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-200',
  },
  {
    title: t('home.capabilities.keys.title'),
    desc: t('home.capabilities.keys.desc'),
    icon: 'key' as const,
    iconClass: 'bg-cyan-50 text-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-200',
  },
  {
    title: t('home.capabilities.risk.title'),
    desc: t('home.capabilities.risk.desc'),
    icon: 'shield' as const,
    iconClass: 'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-200',
  },
])

const channelRows = computed(() => [
  {
    name: t('home.hero.channels.openai'),
    status: 'healthy',
    latency: '182 ms',
    icon: 'cpu' as const,
    iconClass: 'home-channel-icon-blue',
    statusClass: 'home-status-dot-good',
  },
  {
    name: t('home.hero.channels.gemini'),
    status: 'standby',
    latency: '246 ms',
    icon: 'database' as const,
    iconClass: 'home-channel-icon-cyan',
    statusClass: 'home-status-dot-warn',
  },
  {
    name: t('home.hero.channels.anthropic'),
    status: 'healthy',
    latency: '211 ms',
    icon: 'shield' as const,
    iconClass: 'home-channel-icon-emerald',
    statusClass: 'home-status-dot-good',
  },
  {
    name: t('home.hero.channels.grok'),
    status: 'limited',
    latency: '308 ms',
    icon: 'bolt' as const,
    iconClass: 'home-channel-icon-amber',
    statusClass: 'home-status-dot-warn',
  },
])

const requestRows = computed(() => [
  {
    label: t('home.hero.routeLabel'),
    value: 'gpt-4.1-mini -> openai-compatible-03',
    icon: 'swap' as const,
    rowClass: 'home-metric-route',
    iconClass: 'home-metric-icon-route',
    valueClass: 'text-cyan-200',
  },
  {
    label: t('home.hero.billingLabel'),
    value: '-$0.0042',
    icon: 'creditCard' as const,
    rowClass: 'home-metric-billing',
    iconClass: 'home-metric-icon-billing',
    valueClass: 'text-amber-200',
  },
  {
    label: t('home.hero.latencyLabel'),
    value: '182 ms',
    icon: 'clock' as const,
    rowClass: 'home-metric-latency',
    iconClass: 'home-metric-icon-latency',
    valueClass: 'text-sky-200',
  },
  {
    label: t('home.hero.successLabel'),
    value: '99.9%',
    icon: 'checkCircle' as const,
    rowClass: 'home-metric-success',
    iconClass: 'home-metric-icon-success',
    valueClass: 'text-emerald-200',
  },
])

const integrationPoints = computed(() => [
  t('home.integration.replaceBaseUrl'),
  t('home.integration.useApiKey'),
  t('home.integration.monitorCost'),
])

const workflowSteps = computed(() => [
  {
    number: '01',
    title: t('home.workflow.step1.title'),
    desc: t('home.workflow.step1.desc'),
  },
  {
    number: '02',
    title: t('home.workflow.step2.title'),
    desc: t('home.workflow.step2.desc'),
  },
  {
    number: '03',
    title: t('home.workflow.step3.title'),
    desc: t('home.workflow.step3.desc'),
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
.home-motion-root {
  --motion-distance: 30px;
  --motion-duration: 860ms;
  --motion-ease: cubic-bezier(0.16, 1, 0.3, 1);
  --motion-section-distance: 24px;
  --motion-scale: 0.985;
  --motion-section-scale: 0.99;
  --motion-blur: 6px;
  --motion-section-blur: 4px;
}

.home-grid-bg {
  animation: home-grid-drift 22s linear infinite;
}

.home-reveal {
  animation: home-rise-in var(--motion-duration) var(--motion-ease) both;
}

.home-reveal-1 {
  animation-delay: 80ms;
}

.home-reveal-2 {
  animation-delay: 160ms;
}

.home-reveal-3 {
  animation-delay: 240ms;
}

.home-reveal-4 {
  animation-delay: 320ms;
}

.home-reveal-panel {
  animation-delay: 260ms;
}

.home-action-button {
  transform: translateZ(0);
  transition:
    transform 180ms ease,
    box-shadow 180ms ease,
    background-color 180ms ease,
    border-color 180ms ease,
    color 180ms ease;
}

.home-action-button:hover {
  transform: translateY(-2px);
}

.home-balanced-title {
  text-wrap: balance;
}

.home-copy-measure {
  text-wrap: pretty;
}

.home-proof-chip {
  position: relative;
  border: 1px solid rgba(147, 197, 253, 0.64);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(248, 250, 252, 0.76)),
    rgba(255, 255, 255, 0.72);
  box-shadow: 0 1px 1px rgba(15, 23, 42, 0.04), 0 10px 28px rgba(37, 99, 235, 0.07);
  transition:
    transform 180ms ease,
    border-color 180ms ease,
    box-shadow 180ms ease,
    background-color 180ms ease;
}

.home-proof-chip:hover {
  transform: translateY(-2px);
  border-color: rgba(37, 99, 235, 0.38);
  box-shadow: 0 12px 30px rgba(37, 99, 235, 0.12);
}

.dark .home-proof-chip {
  border-color: rgba(96, 165, 250, 0.2);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.7), rgba(15, 23, 42, 0.45)),
    rgba(255, 255, 255, 0.04);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.22);
}

.home-proof-icon,
.home-channel-icon,
.home-metric-icon {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.5rem;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.68);
}

.home-proof-icon {
  width: 1.75rem;
  height: 1.75rem;
}

.home-proof-icon-blue {
  color: #1d4ed8;
  background: linear-gradient(135deg, rgba(219, 234, 254, 0.96), rgba(191, 219, 254, 0.78));
}

.home-proof-icon-cyan {
  color: #0e7490;
  background: linear-gradient(135deg, rgba(207, 250, 254, 0.96), rgba(186, 230, 253, 0.78));
}

.home-proof-icon-amber {
  color: #b45309;
  background: linear-gradient(135deg, rgba(254, 243, 199, 0.96), rgba(219, 234, 254, 0.72));
}

.dark .home-proof-icon {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.12);
}

.dark .home-proof-icon-blue {
  color: #bfdbfe;
  background: rgba(59, 130, 246, 0.16);
}

.dark .home-proof-icon-cyan {
  color: #a5f3fc;
  background: rgba(6, 182, 212, 0.14);
}

.dark .home-proof-icon-amber {
  color: #fde68a;
  background: rgba(245, 158, 11, 0.14);
}

.home-routing-panel {
  overflow: hidden;
  isolation: isolate;
}

.home-routing-panel::before {
  position: absolute;
  inset: 0;
  z-index: -1;
  content: '';
  background:
    radial-gradient(circle at 12% 20%, rgba(59, 130, 246, 0.24), transparent 30%),
    radial-gradient(circle at 88% 0%, rgba(14, 116, 144, 0.18), transparent 28%),
    radial-gradient(circle at 55% 92%, rgba(6, 182, 212, 0.14), transparent 30%);
  opacity: 0.9;
}

.home-routing-panel::after,
.home-code-panel::after {
  position: absolute;
  inset: 0;
  pointer-events: none;
  content: '';
  background: linear-gradient(
    115deg,
    transparent 0%,
    transparent 34%,
    rgba(255, 255, 255, 0.1) 46%,
    transparent 58%,
    transparent 100%
  );
  transform: translateX(-120%);
  animation: home-sheen 7.2s ease-in-out infinite;
}

.home-code-panel {
  position: relative;
  overflow: hidden;
}

.home-panel-icon {
  position: relative;
  display: inline-flex;
  width: 3rem;
  height: 3rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid rgba(125, 211, 252, 0.34);
  border-radius: 0.75rem;
  color: #67e8f9;
  background:
    radial-gradient(circle at 35% 24%, rgba(255, 255, 255, 0.18), transparent 34%),
    linear-gradient(135deg, rgba(37, 99, 235, 0.34), rgba(6, 182, 212, 0.16));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.18),
    0 18px 34px rgba(6, 182, 212, 0.14);
}

.home-panel-icon::after {
  position: absolute;
  inset: 0.35rem;
  pointer-events: none;
  content: '';
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 0.55rem;
}

.home-motion-card {
  --motion-index: 0;
  transform: translateZ(0);
  transition:
    transform 190ms ease,
    border-color 190ms ease,
    box-shadow 190ms ease,
    background-color 190ms ease;
}

.home-motion-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.12);
}

.home-channel-card {
  position: relative;
  overflow: hidden;
  animation: home-soft-pop 560ms var(--motion-ease) both;
  animation-delay: calc(360ms + (var(--motion-index) * 70ms));
}

.home-channel-card::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
  content: '';
  background: linear-gradient(180deg, rgba(96, 165, 250, 0.9), rgba(6, 182, 212, 0.36));
}

.home-channel-icon {
  width: 1.5rem;
  height: 1.5rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.home-channel-icon-blue {
  color: #bfdbfe;
  background: rgba(59, 130, 246, 0.14);
}

.home-channel-icon-cyan {
  color: #a5f3fc;
  background: rgba(6, 182, 212, 0.12);
}

.home-channel-icon-emerald {
  color: #a7f3d0;
  background: rgba(16, 185, 129, 0.12);
}

.home-channel-icon-amber {
  color: #fde68a;
  background: rgba(245, 158, 11, 0.12);
}

.home-status-dot {
  position: relative;
  display: inline-flex;
  width: 0.55rem;
  height: 0.55rem;
  flex-shrink: 0;
  border-radius: 9999px;
}

.home-status-dot::after {
  position: absolute;
  inset: 2px;
  content: '';
  border-radius: inherit;
  background: rgba(255, 255, 255, 0.72);
}

.home-status-dot-live {
  --status-pulse-color: rgba(103, 232, 249, 0.42);
  --status-pulse-transparent: rgba(103, 232, 249, 0);
  --status-glow-color: rgba(103, 232, 249, 0.72);
  width: 0.45rem;
  height: 0.45rem;
  background: #67e8f9;
}

.home-status-dot-good {
  --status-pulse-color: rgba(52, 211, 153, 0.38);
  --status-pulse-transparent: rgba(52, 211, 153, 0);
  --status-glow-color: rgba(52, 211, 153, 0.48);
  background: #34d399;
}

.home-status-dot-warn {
  --status-pulse-color: rgba(251, 191, 36, 0.34);
  --status-pulse-transparent: rgba(251, 191, 36, 0);
  --status-glow-color: rgba(251, 191, 36, 0.42);
  background: #fbbf24;
}

.home-status-pulse {
  position: relative;
  animation: home-status-pulse 2.2s ease-out infinite;
}

.home-metric-row {
  position: relative;
  overflow: hidden;
  border: 1px solid transparent;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.065), rgba(255, 255, 255, 0.035)),
    rgba(255, 255, 255, 0.035);
}

.home-metric-row::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
  content: '';
  background: var(--metric-accent, rgba(125, 211, 252, 0.75));
  opacity: 0.9;
}

.home-metric-row:hover {
  border-color: rgba(125, 211, 252, 0.16);
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.1), rgba(255, 255, 255, 0.04)),
    rgba(255, 255, 255, 0.05);
}

.home-metric-route {
  --metric-accent: linear-gradient(180deg, #67e8f9, #60a5fa);
}

.home-metric-billing {
  --metric-accent: linear-gradient(180deg, #fbbf24, #60a5fa);
}

.home-metric-latency {
  --metric-accent: linear-gradient(180deg, #93c5fd, #38bdf8);
}

.home-metric-success {
  --metric-accent: linear-gradient(180deg, #34d399, #67e8f9);
}

.home-metric-icon {
  width: 1.5rem;
  height: 1.5rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.home-metric-icon-route {
  color: #a5f3fc;
  background: rgba(6, 182, 212, 0.12);
}

.home-metric-icon-billing {
  color: #fde68a;
  background: rgba(245, 158, 11, 0.12);
}

.home-metric-icon-latency {
  color: #bfdbfe;
  background: rgba(96, 165, 250, 0.12);
}

.home-metric-icon-success {
  color: #a7f3d0;
  background: rgba(16, 185, 129, 0.12);
}

.home-scroll-reveal {
  animation: home-rise-in 860ms var(--motion-ease) both;
  animation-delay: calc(90ms + (var(--motion-index) * 68ms));
  animation-timeline: view();
  animation-range: entry 4% cover 36%;
}

.home-section-reveal {
  animation: home-section-rise 820ms var(--motion-ease) both;
  animation-timeline: view();
  animation-range: entry 6% cover 34%;
}

.home-section-reveal-1 {
  animation-delay: 0ms;
}

.home-section-reveal-2 {
  animation-delay: 90ms;
}

.home-section-reveal-3 {
  animation-delay: 170ms;
}

.home-cta-panel {
  position: relative;
  overflow: hidden;
}

.home-cta-panel.home-scroll-reveal {
  animation-name: home-panel-rise;
  animation-duration: 980ms;
  animation-range: entry 5% cover 42%;
}

.home-code-panel.home-scroll-reveal {
  animation-duration: 960ms;
  animation-range: entry 5% cover 40%;
}

.home-cta-panel::before {
  position: absolute;
  inset: 0;
  content: '';
  background:
    linear-gradient(135deg, rgba(59, 130, 246, 0.24), transparent 32%),
    linear-gradient(315deg, rgba(14, 116, 144, 0.18), transparent 36%),
    radial-gradient(circle at 50% 0%, rgba(6, 182, 212, 0.16), transparent 42%);
  opacity: 0.75;
}

.home-cta-panel > * {
  position: relative;
}

@keyframes home-rise-in {
  from {
    opacity: 0;
    filter: blur(var(--motion-blur));
    transform: translateY(var(--motion-distance)) scale(var(--motion-scale));
  }

  to {
    opacity: 1;
    filter: blur(0);
    transform: translateY(0) scale(1);
  }
}

@keyframes home-section-rise {
  from {
    opacity: 0;
    filter: blur(var(--motion-section-blur));
    transform: translateY(var(--motion-section-distance)) scale(var(--motion-section-scale));
  }

  to {
    opacity: 1;
    filter: blur(0);
    transform: translateY(0) scale(1);
  }
}

@keyframes home-panel-rise {
  from {
    opacity: 0;
    transform: translateY(var(--motion-distance)) scale(var(--motion-scale));
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes home-soft-pop {
  from {
    opacity: 0;
    transform: translateY(10px) scale(0.985);
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes home-status-pulse {
  0% {
    box-shadow:
      0 0 16px var(--status-glow-color, rgba(52, 211, 153, 0.42)),
      0 0 0 0 var(--status-pulse-color, rgba(52, 211, 153, 0.36));
  }

  70% {
    box-shadow:
      0 0 16px var(--status-glow-color, rgba(52, 211, 153, 0.42)),
      0 0 0 8px var(--status-pulse-transparent, rgba(52, 211, 153, 0));
  }

  100% {
    box-shadow:
      0 0 16px var(--status-glow-color, rgba(52, 211, 153, 0.42)),
      0 0 0 0 var(--status-pulse-transparent, rgba(52, 211, 153, 0));
  }
}

@keyframes home-sheen {
  0%,
  58% {
    transform: translateX(-120%);
  }

  82%,
  100% {
    transform: translateX(120%);
  }
}

@keyframes home-grid-drift {
  from {
    background-position: 0 0;
  }

  to {
    background-position: 48px 48px;
  }
}

@media (max-width: 640px) {
  .home-motion-root {
    --motion-distance: 20px;
    --motion-section-distance: 16px;
    --motion-blur: 3px;
    --motion-section-blur: 2px;
  }

  .home-scroll-reveal,
  .home-section-reveal {
    animation-delay: 60ms;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-motion-root *,
  .home-motion-root *::before,
  .home-motion-root *::after {
    scroll-behavior: auto !important;
    transition-duration: 1ms !important;
    animation-duration: 1ms !important;
    animation-iteration-count: 1 !important;
    animation-delay: 0ms !important;
  }

  .home-action-button:hover,
  .home-motion-card:hover {
    transform: none;
  }
}
</style>
