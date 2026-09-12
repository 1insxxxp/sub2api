<template>
  <div
    class="auth-shell theme-crisp relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-8 sm:px-6 sm:py-10"
    :class="{ 'auth-shell--login': props.appearance === 'login' }"
  >
    <!-- Background -->
    <div
      class="absolute inset-0 bg-white dark:bg-dark-950"
    ></div>

    <!-- Decorative Elements -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="auth-grid-bg absolute inset-0 bg-[linear-gradient(rgba(37,99,235,0.07)_1px,transparent_1px),linear-gradient(90deg,rgba(37,99,235,0.06)_1px,transparent_1px)] bg-[size:48px_48px] dark:bg-[linear-gradient(rgba(96,165,250,0.1)_1px,transparent_1px),linear-gradient(90deg,rgba(6,182,212,0.07)_1px,transparent_1px)]"
      ></div>
      <div
        class="absolute inset-x-0 top-0 h-[32rem] bg-[linear-gradient(180deg,rgba(219,234,254,0.92),rgba(255,255,255,0))] dark:bg-[linear-gradient(180deg,rgba(30,64,175,0.28),rgba(2,6,23,0))]"
      ></div>
      <div
        class="absolute inset-x-0 bottom-0 h-56 bg-[linear-gradient(0deg,rgba(236,254,255,0.72),rgba(255,255,255,0))] dark:bg-[linear-gradient(0deg,rgba(8,47,73,0.18),rgba(2,6,23,0))]"
      ></div>
      <div
        class="absolute left-1/2 top-24 h-px w-[min(42rem,80vw)] -translate-x-1/2 bg-[linear-gradient(90deg,transparent,#2563eb,#06b6d4,transparent)] opacity-50 dark:opacity-70"
      ></div>
      <div v-if="props.appearance === 'login'" class="auth-scanline absolute inset-x-0 top-0 h-px bg-blue-300/40 dark:bg-cyan-300/25"></div>
    </div>

    <!-- Content Container -->
    <div class="auth-panel relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="auth-brand mb-7 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="auth-logo-frame mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-lg bg-[linear-gradient(135deg,#2563eb,#3b82f6,#06b6d4)] p-0.5 shadow-[0_18px_44px_rgba(37,99,235,0.18)] ring-1 ring-blue-300/50"
          >
            <span class="flex h-full w-full items-center justify-center overflow-hidden rounded-[0.42rem] bg-white/95 dark:bg-dark-950/90">
              <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
            </span>
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="mx-auto max-w-xs text-sm leading-6 text-slate-500 dark:text-dark-300">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-card rounded-lg border border-blue-200/70 bg-white p-5 shadow-[0_26px_80px_-44px_rgba(37,99,235,0.42)] ring-1 ring-white/80 backdrop-blur-xl dark:border-blue-400/18 dark:bg-dark-900 dark:ring-white/5 sm:p-8">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="auth-footer mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-slate-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const props = defineProps<{ appearance?: 'default' | 'login' }>()

const siteName = computed(() => appStore.siteName || 'Passion')
const siteLogo = computed(() => sanitizeUrl(appStore.effectiveSiteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-grid-bg {
  animation: auth-grid-drift 24s linear infinite;
}

.auth-panel {
  animation: auth-panel-rise 720ms cubic-bezier(0.16, 1, 0.3, 1) both;
}

.auth-scanline {
  animation: auth-scanline 9s ease-in-out infinite;
  transform: translateX(-100%);
}

.auth-card {
  position: relative;
  overflow: hidden;
}

.auth-shell--login .auth-grid-bg {
  opacity: 0.48;
}

.auth-shell--login .auth-card {
  animation: auth-card-rise 760ms 80ms cubic-bezier(0.16, 1, 0.3, 1) both;
}

.auth-card::before {
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  content: '';
  background: linear-gradient(90deg, #2563eb, #3b82f6, #06b6d4);
}

.auth-logo-frame {
  transform: translateZ(0);
  transition: transform 220ms ease, box-shadow 220ms ease;
}

.auth-shell--login .auth-logo-frame {
  animation: auth-logo-breathe 5s ease-in-out 260ms infinite;
}

.auth-shell--login .auth-logo-frame:hover {
  transform: translateY(-3px) rotate(-2deg) scale(1.03);
  box-shadow: 0 22px 48px rgba(37, 99, 235, 0.24);
}

:slotted(.auth-login-content) {
  animation: auth-content-rise 620ms 180ms cubic-bezier(0.16, 1, 0.3, 1) both;
}

@keyframes auth-scanline {
  0%,
  18% {
    opacity: 0;
    transform: translateX(-100%);
  }

  35%,
  65% {
    opacity: 1;
  }

  82%,
  100% {
    opacity: 0;
    transform: translateX(100%);
  }
}

@keyframes auth-card-rise {
  from {
    opacity: 0;
    transform: translateY(14px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes auth-content-rise {
  from {
    opacity: 0;
    transform: translateY(8px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes auth-logo-breathe {
  0%,
  100% {
    box-shadow: 0 18px 44px rgba(37, 99, 235, 0.18);
  }

  50% {
    box-shadow: 0 22px 52px rgba(6, 182, 212, 0.24);
  }
}

@keyframes auth-grid-drift {
  from {
    background-position: 0 0;
  }

  to {
    background-position: 48px 48px;
  }
}

@keyframes auth-panel-rise {
  from {
    opacity: 0;
    filter: blur(5px);
    transform: translateY(22px) scale(0.99);
  }

  to {
    opacity: 1;
    filter: blur(0);
    transform: translateY(0) scale(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-grid-bg,
  .auth-panel,
  .auth-scanline,
  .auth-card,
  .auth-logo-frame,
  :slotted(.auth-login-content) {
    animation-duration: 1ms;
    animation-iteration-count: 1;
  }

  .auth-shell--login .auth-logo-frame:hover {
    transform: none;
  }
}
</style>
