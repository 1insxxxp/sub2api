<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed right-4 top-4 z-[9999] space-y-3"
      aria-live="polite"
      aria-atomic="true"
    >
      <TransitionGroup
        enter-active-class="transition ease-out duration-300"
        enter-from-class="opacity-0 translate-x-full"
        enter-to-class="opacity-100 translate-x-0"
        leave-active-class="transition ease-in duration-200"
        leave-from-class="opacity-100 translate-x-0"
        leave-to-class="opacity-0 translate-x-full"
      >
        <div
          v-for="toast in toasts"
          :key="toast.id"
          :class="[
            'brand-floating-panel pointer-events-auto min-w-[320px] max-w-[min(30rem,calc(100vw-2rem))] overflow-hidden',
            getToneClass(toast.type)
          ]"
        >
          <div class="relative p-4">
            <div class="flex items-start gap-3">
              <!-- Icon -->
              <div :class="['toast-icon-shell mt-0.5 flex-shrink-0', getIconShellClass(toast.type)]">
                <Icon :name="getToastIconName(toast.type)" size="md" class="text-current" aria-hidden="true" />
              </div>

              <!-- Content -->
              <div class="min-w-0 flex-1">
                <p v-if="toast.title" class="text-sm font-semibold text-slate-950 dark:text-white">
                  {{ toast.title }}
                </p>
                <p
                  :class="[
                    'text-sm leading-relaxed',
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
                class="brand-floating-close -m-1 h-9 w-9 flex-shrink-0 rounded-xl p-0"
                aria-label="Close notification"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>

          <!-- Progress bar -->
          <div v-if="toast.duration" class="toast-progress-track">
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
    success: 'bg-green-500',
    error: 'bg-red-500',
    warning: 'bg-yellow-500',
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
  display: inline-flex;
  height: 2.5rem;
  width: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.95rem;
  border-width: 1px;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.08);
}

.toast-progress-track {
  height: 4px;
  background: rgba(148, 163, 184, 0.14);
}

.dark .toast-progress-track {
  background: rgba(51, 65, 85, 0.46);
}

.toast-success::after,
.toast-error::after,
.toast-warning::after,
.toast-info::after {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 4px;
}

.toast-success::after {
  background: linear-gradient(180deg, #10b981, #34d399);
}

.toast-error::after {
  background: linear-gradient(180deg, #f43f5e, #fb7185);
}

.toast-warning::after {
  background: linear-gradient(180deg, #f59e0b, #fbbf24);
}

.toast-info::after {
  background: linear-gradient(180deg, #2563eb, #06b6d4);
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
