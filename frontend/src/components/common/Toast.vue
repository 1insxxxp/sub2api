<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed right-3 top-3 z-[9999] space-y-2 sm:right-4 sm:top-4"
      aria-live="polite"
      aria-atomic="true"
    >
      <TransitionGroup
        enter-active-class="transition ease-out duration-200"
        enter-from-class="opacity-0 translate-x-4 scale-[0.98]"
        enter-to-class="opacity-100 translate-x-0"
        leave-active-class="transition ease-in duration-150"
        leave-from-class="opacity-100 translate-x-0"
        leave-to-class="opacity-0 translate-x-4 scale-[0.98]"
      >
        <div
          v-for="toast in toasts"
          :key="toast.id"
          :class="[
            'toast-panel admin-toast-panel pointer-events-auto relative min-w-[280px] max-w-[min(24rem,calc(100vw-1.5rem))] overflow-hidden rounded-xl border border-slate-200 bg-white/95 shadow-lg shadow-slate-950/10 ring-1 ring-slate-950/5 backdrop-blur-sm dark:border-white/10 dark:bg-slate-950/95 dark:shadow-black/25 dark:ring-white/10',
            getToneClass(toast.type)
          ]"
        >
          <div class="toast-body relative px-3 py-2.5">
            <div class="flex items-start gap-2.5">
              <!-- Icon -->
              <div :class="['toast-icon-shell mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg border', getIconShellClass(toast.type)]">
                <Icon :name="getToastIconName(toast.type)" size="sm" class="text-current" aria-hidden="true" />
              </div>

              <!-- Content -->
              <div class="min-w-0 flex-1">
                <p v-if="toast.title" class="text-sm font-semibold text-slate-950 dark:text-white">
                  {{ toast.title }}
                </p>
                <p
                  :class="[
                    'text-[13px] leading-5',
                    toast.title
                      ? 'mt-1 text-slate-600 dark:text-slate-300'
                      : 'text-slate-900 dark:text-slate-100'
                  ]"
                >
                  {{ toast.message }}
                </p>
              </div>

              <!-- Close button -->
              <button
                @click="removeToast(toast.id)"
                class="toast-close -mr-1 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-lg text-slate-400 transition-colors duration-150 hover:bg-slate-100 hover:text-slate-700 focus:outline-none focus:ring-2 focus:ring-blue-500/20 dark:text-slate-500 dark:hover:bg-white/[0.06] dark:hover:text-slate-200"
                aria-label="Close notification"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>

          <!-- Progress bar -->
          <div v-if="toast.duration" class="toast-progress-track h-0.5">
            <div
              :class="['h-full toast-progress', getProgressBarColor(toast.type)]"
              :style="{ animationDuration: `${toast.duration}ms` }"
            ></div>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

const toasts = computed(() => appStore.toasts)

const getToastIconName = (type: string): 'checkCircle' | 'xCircle' | 'exclamationTriangle' | 'infoCircle' => {
  switch (type) {
    case 'success':
      return 'checkCircle'
    case 'error':
      return 'xCircle'
    case 'warning':
      return 'exclamationTriangle'
    case 'info':
    default:
      return 'infoCircle'
  }
}

const getIconShellClass = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'text-emerald-600 bg-emerald-50 border-emerald-200/80 dark:text-emerald-200 dark:bg-emerald-500/12 dark:border-emerald-400/20',
    error: 'text-rose-600 bg-rose-50 border-rose-200/80 dark:text-rose-200 dark:bg-rose-500/12 dark:border-rose-400/20',
    warning: 'text-amber-600 bg-amber-50 border-amber-200/80 dark:text-amber-200 dark:bg-amber-500/12 dark:border-amber-400/20',
    info: 'text-blue-600 bg-blue-50 border-blue-200/80 dark:text-blue-200 dark:bg-blue-500/12 dark:border-blue-400/20'
  }
  return colors[type] || colors.info
}

const getToneClass = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'toast-success',
    error: 'toast-error',
    warning: 'toast-warning',
    info: 'toast-info'
  }
  return colors[type] || colors.info
}

const getProgressBarColor = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'bg-emerald-500',
    error: 'bg-rose-500',
    warning: 'bg-amber-500',
    info: 'bg-blue-500'
  }
  return colors[type] || colors.info
}

const removeToast = (id: string) => {
  appStore.hideToast(id)
}
</script>

<style scoped>
.toast-icon-shell {
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

.toast-progress-track {
  background: rgba(148, 163, 184, 0.16);
}

:global(.dark) .toast-progress-track {
  background: rgba(255, 255, 255, 0.08);
}

.toast-success::after,
.toast-error::after,
.toast-warning::after,
.toast-info::after {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
}

.toast-success::after {
  background: #10b981;
}

.toast-error::after {
  background: #f43f5e;
}

.toast-warning::after {
  background: #f59e0b;
}

.toast-info::after {
  background: #2563eb;
}

.toast-progress {
  width: 100%;
  animation-name: toast-progress-shrink;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

@keyframes toast-progress-shrink {
  from {
    width: 100%;
  }
  to {
    width: 0%;
  }
}
</style>
