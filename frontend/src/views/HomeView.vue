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
    class="min-h-screen overflow-x-hidden bg-white text-gray-950 dark:bg-dark-950 dark:text-white"
  >
    <div class="relative">
      <div
        class="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(15,23,42,0.04)_1px,transparent_1px),linear-gradient(90deg,rgba(15,23,42,0.04)_1px,transparent_1px)] bg-[size:48px_48px] dark:bg-[linear-gradient(rgba(148,163,184,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(148,163,184,0.08)_1px,transparent_1px)]"
      ></div>
      <div
        class="pointer-events-none absolute inset-x-0 top-0 h-[34rem] bg-[linear-gradient(180deg,rgba(240,253,250,0.9),rgba(255,255,255,0))] dark:bg-[linear-gradient(180deg,rgba(8,47,73,0.35),rgba(2,6,23,0))]"
      ></div>

      <header class="relative z-20 border-b border-gray-200/70 bg-white/85 backdrop-blur-xl dark:border-dark-800/70 dark:bg-dark-950/80">
        <nav class="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
          <router-link to="/" class="flex min-w-0 items-center gap-3">
            <span
              class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900"
            >
              <img
                :src="siteLogo || '/logo.png'"
                :alt="siteName"
                class="h-full w-full object-contain"
              />
            </span>
            <span class="min-w-0 truncate text-sm font-semibold text-gray-950 dark:text-white">
              {{ siteName }}
            </span>
          </router-link>

          <div class="hidden items-center gap-6 lg:flex">
            <a
              v-for="item in navItems"
              :key="item.href"
              :href="item.href"
              class="text-sm font-medium text-gray-600 transition-colors hover:text-gray-950 dark:text-dark-300 dark:hover:text-white"
            >
              {{ item.label }}
            </a>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm font-medium text-gray-600 transition-colors hover:text-gray-950 dark:text-dark-300 dark:hover:text-white"
            >
              {{ t('home.docs') }}
            </a>
          </div>

          <div class="flex items-center gap-2 sm:gap-3">
            <LocaleSwitcher />

            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="hidden rounded-lg border border-gray-200 bg-white p-2 text-gray-600 transition-colors hover:border-gray-300 hover:text-gray-950 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300 dark:hover:border-dark-600 dark:hover:text-white sm:inline-flex lg:hidden"
              :title="t('home.viewDocs')"
            >
              <Icon name="book" size="sm" />
            </a>

            <button
              type="button"
              class="rounded-lg border border-gray-200 bg-white p-2 text-gray-600 transition-colors hover:border-gray-300 hover:text-gray-950 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300 dark:hover:border-dark-600 dark:hover:text-white"
              :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
              @click="toggleTheme"
            >
              <Icon v-if="isDark" name="sun" size="sm" />
              <Icon v-else name="moon" size="sm" />
            </button>

            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-950 px-3 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-200 sm:px-4"
            >
              <span v-if="isAuthenticated && userInitial" class="hidden sm:inline">{{ userInitial }}</span>
              <span>
                {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.login') }}
              </span>
            </router-link>
          </div>
        </nav>
      </header>

      <main class="relative z-10">
        <section class="mx-auto grid max-w-7xl items-center gap-10 px-4 pb-14 pt-12 sm:px-6 sm:pb-20 sm:pt-16 lg:grid-cols-[minmax(0,1fr),minmax(420px,0.9fr)] lg:gap-14 lg:px-8 lg:pb-24 lg:pt-24">
          <div class="min-w-0">
            <div
              class="mb-5 inline-flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-sm font-semibold text-emerald-800 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200"
            >
              <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
              {{ t('home.hero.eyebrow') }}
            </div>

            <h1
              class="max-w-3xl break-words text-4xl font-semibold leading-[1.08] text-gray-950 dark:text-white sm:text-5xl lg:text-6xl"
            >
              {{ t('home.hero.title') }}
            </h1>

            <p class="mt-6 max-w-2xl text-base leading-8 text-gray-600 dark:text-dark-300 sm:text-lg">
              {{ t('home.hero.subtitle') }}
            </p>

            <div class="mt-8 flex flex-col gap-3 sm:flex-row">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="inline-flex h-11 items-center justify-center gap-2 rounded-lg bg-primary-600 px-5 text-sm font-semibold text-white shadow-sm shadow-primary-600/20 transition-colors hover:bg-primary-700"
              >
                {{ isAuthenticated ? t('home.hero.dashboardCta') : t('home.hero.primaryCta') }}
                <Icon name="arrowRight" size="sm" />
              </router-link>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex h-11 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-5 text-sm font-semibold text-gray-800 transition-colors hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-100 dark:hover:border-dark-600 dark:hover:bg-dark-800"
              >
                <Icon name="book" size="sm" />
                {{ t('home.hero.secondaryCta') }}
              </a>
            </div>
          </div>

          <section
            class="relative min-w-0 rounded-lg border border-gray-900 bg-gray-950 p-4 text-white shadow-2xl shadow-gray-950/20 dark:border-dark-700"
            aria-labelledby="routing-panel-title"
          >
            <div class="flex items-start justify-between gap-4 border-b border-white/10 pb-4">
              <div class="min-w-0">
                <div class="mb-2 inline-flex items-center gap-2 rounded-lg bg-emerald-400/10 px-2.5 py-1 text-xs font-semibold text-emerald-200">
                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>
                  {{ t('home.hero.statusBadge') }}
                </div>
                <h2 id="routing-panel-title" class="text-lg font-semibold">
                  {{ t('home.hero.panelTitle') }}
                </h2>
                <p class="mt-1 text-sm leading-6 text-slate-400">
                  {{ t('home.hero.panelSubtitle') }}
                </p>
              </div>
              <Icon name="server" size="lg" class="shrink-0 text-emerald-300" />
            </div>

            <div class="grid gap-3 py-4 sm:grid-cols-2">
              <div
                v-for="channel in channelRows"
                :key="channel.name"
                class="rounded-lg border border-white/10 bg-white/[0.04] p-3"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="min-w-0 truncate text-sm font-semibold">{{ channel.name }}</span>
                  <span
                    class="h-2 w-2 shrink-0 rounded-full"
                    :class="channel.active ? 'bg-emerald-400' : 'bg-amber-300'"
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
                v-for="row in requestRows"
                :key="row.label"
                class="flex items-center justify-between gap-4 rounded-lg bg-white/[0.04] px-3 py-2 text-sm"
              >
                <span class="text-slate-400">{{ row.label }}</span>
                <span class="min-w-0 truncate font-semibold" :class="row.valueClass">
                  {{ row.value }}
                </span>
              </div>
            </div>

            <pre
              class="mt-4 overflow-x-auto rounded-lg border border-white/10 bg-black/35 p-4 text-xs leading-6 text-slate-300"
            ><code>curl {{ apiBaseUrl }}/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -d '{ "model": "gpt-4.1-mini" }'</code></pre>
          </section>
        </section>

        <section class="border-y border-gray-200 bg-gray-50/80 dark:border-dark-800 dark:bg-dark-900/50">
          <div class="mx-auto grid max-w-7xl gap-4 px-4 py-6 sm:grid-cols-2 sm:px-6 lg:grid-cols-4 lg:px-8">
            <div
              v-for="item in trustItems"
              :key="item.title"
              class="flex min-w-0 gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
            >
              <div
                class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg"
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
          <div class="max-w-3xl">
            <p class="text-sm font-semibold text-primary-700 dark:text-primary-300">
              {{ t('home.sections.capabilitiesEyebrow') }}
            </p>
            <h2 class="mt-3 text-3xl font-semibold text-gray-950 dark:text-white sm:text-4xl">
              {{ t('home.sections.capabilitiesTitle') }}
            </h2>
            <p class="mt-4 text-base leading-8 text-gray-600 dark:text-dark-300">
              {{ t('home.sections.capabilitiesSubtitle') }}
            </p>
          </div>

          <div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <article
              v-for="item in capabilityItems"
              :key="item.title"
              class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition-colors hover:border-gray-300 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600"
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
              <p class="text-sm font-semibold text-emerald-300">{{ t('home.integration.eyebrow') }}</p>
              <h2 class="mt-3 text-3xl font-semibold sm:text-4xl">{{ t('home.integration.title') }}</h2>
              <p class="mt-4 text-base leading-8 text-slate-300">{{ t('home.integration.subtitle') }}</p>
              <div class="mt-8 grid gap-3">
                <div
                  v-for="point in integrationPoints"
                  :key="point"
                  class="flex items-center gap-3 rounded-lg border border-white/10 bg-white/[0.04] px-4 py-3 text-sm font-semibold text-slate-100"
                >
                  <Icon name="check" size="sm" class="text-emerald-300" />
                  <span>{{ point }}</span>
                </div>
              </div>
            </div>

            <div class="min-w-0 rounded-lg border border-white/10 bg-white/[0.04] p-4 shadow-xl">
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
          <div class="mx-auto max-w-3xl text-center">
            <p class="text-sm font-semibold text-primary-700 dark:text-primary-300">
              {{ t('home.workflow.eyebrow') }}
            </p>
            <h2 class="mt-3 text-3xl font-semibold text-gray-950 dark:text-white sm:text-4xl">
              {{ t('home.workflow.title') }}
            </h2>
            <p class="mt-4 text-base leading-8 text-gray-600 dark:text-dark-300">
              {{ t('home.workflow.subtitle') }}
            </p>
          </div>

          <div class="mt-10 grid gap-4 md:grid-cols-3">
            <article
              v-for="step in workflowSteps"
              :key="step.title"
              class="rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
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
            class="rounded-lg border border-gray-200 bg-gray-950 px-5 py-8 text-center text-white shadow-xl shadow-gray-950/10 dark:border-dark-700 sm:px-8 lg:px-12"
          >
            <h2 class="text-2xl font-semibold sm:text-3xl">{{ t('home.cta.title') }}</h2>
            <p class="mx-auto mt-3 max-w-2xl text-sm leading-7 text-slate-300">
              {{ t('home.footer.tagline') }}
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
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Passion')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || '/api')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => {
  const user = authStore.user
  if (!user?.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const currentYear = computed(() => new Date().getFullYear())

const navItems = computed(() => [
  { href: '#features', label: t('home.nav.features') },
  { href: '#integration', label: t('home.nav.integration') },
  { href: '#workflow', label: t('home.nav.workflow') },
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
    iconClass: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200',
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
    iconClass: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200',
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
    active: true,
  },
  {
    name: t('home.hero.channels.gemini'),
    status: 'standby',
    latency: '246 ms',
    active: false,
  },
  {
    name: t('home.hero.channels.anthropic'),
    status: 'healthy',
    latency: '211 ms',
    active: true,
  },
  {
    name: t('home.hero.channels.grok'),
    status: 'limited',
    latency: '308 ms',
    active: false,
  },
])

const requestRows = computed(() => [
  {
    label: t('home.hero.routeLabel'),
    value: 'gpt-4.1-mini -> openai-compatible-03',
    valueClass: 'text-emerald-200',
  },
  {
    label: t('home.hero.billingLabel'),
    value: '-$0.0042',
    valueClass: 'text-amber-200',
  },
  {
    label: t('home.hero.latencyLabel'),
    value: '182 ms',
    valueClass: 'text-sky-200',
  },
  {
    label: t('home.hero.successLabel'),
    value: '99.9%',
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
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
