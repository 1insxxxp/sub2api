<template>
  <div class="admin-shell min-h-screen bg-slate-50 text-slate-900 dark:bg-dark-950 dark:text-slate-100">
    <div class="pointer-events-none fixed inset-0 bg-[linear-gradient(180deg,rgba(59,130,246,0.08)_0%,rgba(248,250,252,0)_30rem)] dark:bg-[linear-gradient(180deg,rgba(37,99,235,0.18)_0%,rgba(2,6,23,0)_32rem)]"></div>
    <div class="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_top_right,rgba(14,165,233,0.08),transparent_24rem)] dark:bg-[radial-gradient(circle_at_top_right,rgba(96,165,250,0.14),transparent_26rem)]"></div>
    <div class="pointer-events-none fixed inset-y-0 right-0 hidden w-[34rem] bg-[radial-gradient(circle_at_center,rgba(37,99,235,0.05),transparent_72%)] lg:block dark:bg-[radial-gradient(circle_at_center,rgba(59,130,246,0.08),transparent_72%)]"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="app-shell-main relative min-h-screen min-w-0 transition-all duration-300 ease-out"
      :class="{ 'app-shell-main-collapsed': sidebarCollapsed }"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="app-shell-content">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
.app-shell-main {
  margin-left: 0;
  min-width: 0;
}

.app-shell-content {
  --app-content-padding-y: clamp(1rem, 0.7rem + 0.8vw, 2rem);
  --app-content-padding-x: clamp(0.875rem, 0.55rem + 1.05vw, 2rem);
  --app-content-padding-total-y: calc(
    var(--app-content-padding-y) + var(--app-content-padding-y)
  );

  box-sizing: border-box;
  position: relative;
  width: 100%;
  max-width: none;
  min-width: 0;
  padding: var(--app-content-padding-y) var(--app-content-padding-x);
  transition: padding 200ms ease;
}

@media (min-width: 1024px) {
  .app-shell-main {
    margin-left: 16rem;
  }

  .app-shell-main-collapsed {
    margin-left: 72px;
  }
}
</style>
