<template>
  <Teleport to="body">
    <div class="toast-viewport">
      <TransitionGroup
        name="toast"
        tag="div"
        class="toast-stack"
        aria-live="polite"
        aria-atomic="true"
      >
        <div
          v-for="toast in toasts"
          :key="toast.id"
          :class="[
            'toast-panel',
            getToneClass(toast.type),
            { 'toast-has-title': toast.title, 'toast-persistent': toast.duration === undefined }
          ]"
        >
          <div class="toast-body">
            <div class="toast-icon-shell">
              <Icon :name="getToastIconName(toast.type)" size="md" :stroke-width="1.75" aria-hidden="true" />
            </div>

            <div class="toast-content">
              <p v-if="toast.title" class="toast-title">
                {{ toast.title }}
              </p>
              <p class="toast-message">
                {{ toast.message }}
              </p>
            </div>

            <button
              v-if="toast.duration === undefined"
              type="button"
              @click="removeToast(toast.id)"
              class="toast-close"
              :aria-label="t('common.close')"
              :title="t('common.close')"
            >
              <Icon name="x" size="sm" aria-hidden="true" />
            </button>
          </div>

          <div v-if="toast.duration" class="toast-progress-track" aria-hidden="true">
            <div
              class="toast-progress"
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
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()
const { t } = useI18n()

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

const getToneClass = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'toast-success',
    error: 'toast-error',
    warning: 'toast-warning',
    info: 'toast-info'
  }
  return colors[type] || colors.info
}

const removeToast = (id: string) => {
  appStore.hideToast(id)
}
</script>

<style scoped>
.toast-viewport {
  position: fixed;
  inset: 0;
  z-index: 9999;
  overflow: hidden;
  pointer-events: none;
}

.toast-stack {
  --toast-edge: max(1.25rem, env(safe-area-inset-right));
  position: absolute;
  top: max(1rem, env(safe-area-inset-top));
  right: var(--toast-edge);
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.625rem;
  width: min(23rem, calc(100vw - 2.5rem));
  pointer-events: none;
}

.toast-panel {
  --toast-accent: #2563eb;
  position: relative;
  width: fit-content;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  border: 1px solid rgb(100 116 139 / 18%);
  border-radius: 7px;
  background: linear-gradient(115deg, color-mix(in srgb, var(--toast-accent) 5%, transparent), transparent 68%), rgb(252 252 253 / 97%);
  color: #374151;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 90%), 0 2px 5px rgb(15 23 42 / 4%), 0 10px 24px rgb(15 23 42 / 8%);
  backdrop-filter: blur(20px);
  pointer-events: auto;
}

.toast-panel::before {
  position: absolute;
  inset: 0.625rem auto 0.625rem 0;
  width: 2px;
  border-radius: 0 2px 2px 0;
  background: var(--toast-accent);
  content: '';
  opacity: 0.85;
}

.dark .toast-panel {
  --toast-accent: #93c5fd;
  border-color: rgb(255 255 255 / 12%);
  background: linear-gradient(115deg, color-mix(in srgb, var(--toast-accent) 9%, transparent), transparent 68%), rgb(38 40 44 / 97%);
  color: #f4f4f5;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 5%), 0 3px 6px rgb(0 0 0 / 12%), 0 10px 24px rgb(0 0 0 / 24%);
}

.toast-success { --toast-accent: #059669; }
.toast-error { --toast-accent: #e11d48; }
.toast-warning { --toast-accent: #b45309; }
.dark .toast-success { --toast-accent: #6ee7b7; }
.dark .toast-error { --toast-accent: #fda4af; }
.dark .toast-warning { --toast-accent: #fde68a; }

.toast-body {
  display: grid;
  grid-template-columns: 1.375rem minmax(0, 1fr);
  align-items: center;
  gap: 0.625rem;
  padding: 0.6875rem 0.875rem 0.6875rem 1rem;
}

.toast-persistent .toast-body {
  grid-template-columns: 1.375rem minmax(0, 1fr) 1.875rem;
  padding-block: 0.5rem;
  padding-right: 0.375rem;
}

.toast-icon-shell {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.375rem;
  height: 1.375rem;
  border: 1px solid color-mix(in srgb, var(--toast-accent) 32%, transparent);
  border-radius: 50%;
  background: color-mix(in srgb, var(--toast-accent) 8%, transparent);
  color: var(--toast-accent);
  align-self: start;
}

.toast-content {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.8125rem;
  line-height: 1.2rem;
}

.toast-title { font-size: 0.8125rem; font-weight: 600; }
.toast-title + .toast-message { margin-top: 0.25rem; color: #6b7280; font-size: 0.8125rem; }
.dark .toast-title + .toast-message { color: #c4c6cb; }
.toast-persistent .toast-icon-shell { align-self: center; }
.toast-has-title .toast-body { padding-block: 0.8125rem; align-items: start; }
.toast-has-title .toast-icon-shell { align-self: start; }
.toast-has-title .toast-close { margin-block: -0.375rem; }

.toast-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 6px;
  color: #6b7280;
  transition: background-color 160ms ease, color 160ms ease, transform 160ms ease;
}

.dark .toast-close { color: #b5b8be; }
.toast-close:hover { color: var(--toast-accent); background: color-mix(in srgb, var(--toast-accent) 8%, transparent); }
.toast-close:active { transform: scale(0.92); }
.toast-close:focus-visible { outline: 2px solid var(--toast-accent); outline-offset: -2px; }

.toast-progress-track {
  position: absolute;
  inset: auto 0.875rem 0 1rem;
  height: 1px;
  overflow: hidden;
  border-radius: 2px;
  background: transparent;
}

.toast-progress {
  height: 100%;
  background: var(--toast-accent);
  opacity: 0.3;
  transform-origin: left;
  animation: toast-progress-shrink linear forwards;
}

@keyframes toast-progress-shrink {
  to { transform: scaleX(0); }
}

.toast-enter-active { transition: opacity 180ms ease, transform 280ms cubic-bezier(0.22, 1, 0.36, 1); }
.toast-move { transition: transform 220ms cubic-bezier(0.22, 1, 0.36, 1); }
.toast-leave-active { transition: opacity 160ms ease, transform 200ms cubic-bezier(0.4, 0, 1, 1); }
.toast-enter-from,
.toast-leave-to { opacity: 0; transform: translateX(calc(100% + var(--toast-edge))); }

@media (max-width: 767px) {
  .toast-stack {
    --toast-edge: max(0.75rem, env(safe-area-inset-right));
    top: calc(4.5rem + env(safe-area-inset-top));
    width: min(24rem, calc(100vw - 1.5rem - env(safe-area-inset-left) - env(safe-area-inset-right)));
  }
}

@media (prefers-reduced-motion: reduce) {
  .toast-panel,
  .toast-close { transition: none; }
  .toast-enter-from,
  .toast-leave-to,
  .toast-close:active { transform: none; }
  .toast-progress { animation: none; }
  .toast-progress-track { display: none; }
}
</style>
