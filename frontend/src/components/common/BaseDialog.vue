<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="modal-overlay brand-overlay"
        :class="{ 'modal-neutral': neutralAppearance }"
        :style="zIndexStyle"
        :aria-labelledby="dialogId"
        role="dialog"
        aria-modal="true"
        @click.self="handleClose"
      >
        <!-- Modal panel -->
        <div
          ref="dialogRef"
          :class="['modal-content', 'brand-floating-panel', 'admin-surface', 'admin-dialog-panel', widthClasses]"
          @click.stop
        >
          <!-- Header -->
          <div class="modal-header brand-floating-header admin-dialog-header">
            <h3 :id="dialogId" class="modal-title">
              {{ title }}
            </h3>
            <button
              v-if="showCloseButton"
              @click="emit('close')"
              class="brand-floating-close -mr-2 h-10 w-10 rounded-xl p-0 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:ring-offset-2 dark:text-dark-500 dark:hover:bg-dark-700 dark:hover:text-dark-300 dark:focus-visible:ring-offset-dark-900"
              aria-label="Close modal"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <!-- Body -->
          <div ref="modalBodyRef" class="modal-body admin-dialog-body modal-neutral-body modal-body-scroll">
            <slot></slot>
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="modal-footer admin-dialog-footer modal-neutral-footer modal-footer-wrap">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
let dialogIdCounter = 0
const openDialogs = new Map<string, () => number>()

function topDialogId(): string | undefined {
  let topId: string | undefined
  let highestZIndex = -Infinity
  for (const [id, getZIndex] of openDialogs) {
    const zIndex = getZIndex()
    if (zIndex >= highestZIndex) {
      highestZIndex = zIndex
      topId = id
    }
  }
  return topId
}
</script>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAdminAppearance } from '@/composables/useAdminAppearance'

// 生成唯一ID以避免多个对话框时ID冲突
const dialogId = `modal-title-${++dialogIdCounter}`

// 焦点管理
const dialogRef = ref<HTMLElement | null>(null)
const modalBodyRef = ref<HTMLElement | null>(null)
let previousActiveElement: HTMLElement | null = null

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

interface Props {
  show: boolean
  title: string
  width?: DialogWidth
  appearance?: 'default' | 'neutral'
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
}

interface Emits {
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true,
  zIndex: 50
})

const emit = defineEmits<Emits>()
const adminAppearance = useAdminAppearance()
const neutralAppearance = computed(() => props.appearance === 'neutral' || (props.appearance === undefined && adminAppearance.value))

// Custom z-index style (overrides the default z-50 from CSS)
const zIndexStyle = computed(() => {
  return props.zIndex !== 50 ? { zIndex: props.zIndex } : undefined
})

const widthClasses = computed(() => {
  // Width guidance: narrow=confirm/short prompts, normal=standard forms,
  // wide=multi-section forms or rich content, extra-wide=analytics/tables,
  // full=full-screen or very dense layouts.
  const widths: Record<DialogWidth, string> = {
    narrow: 'max-w-md',
    normal: 'max-w-lg',
    wide: 'w-full sm:max-w-2xl md:max-w-3xl lg:max-w-4xl',
    'extra-wide': 'w-full sm:max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl',
    full: 'w-full sm:max-w-4xl md:max-w-5xl lg:max-w-6xl xl:max-w-7xl'
  }
  return widths[props.width]
})

const handleClose = () => {
  if (props.closeOnClickOutside) {
    emit('close')
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (props.show && props.closeOnEscape && event.key === 'Escape' && !event.defaultPrevented && topDialogId() === dialogId) {
    event.preventDefault()
    emit('close')
  }
}

const unregisterDialog = () => {
  const wasTopmost = topDialogId() === dialogId
  openDialogs.delete(dialogId)
  document.body.classList.toggle('modal-open', openDialogs.size > 0)
  if (wasTopmost && previousActiveElement?.isConnected) {
    previousActiveElement.focus()
  }
  previousActiveElement = null
}

// Prevent body scroll when modal is open and manage focus
watch(
  () => props.show,
  async (isOpen) => {
    if (isOpen) {
      // 保存当前焦点元素
      previousActiveElement = document.activeElement as HTMLElement
      openDialogs.set(dialogId, () => props.zIndex)
      document.body.classList.add('modal-open')

      // 等待DOM更新后设置焦点到对话框
      await nextTick()
      if (!props.show) return
      if (modalBodyRef.value) {
        modalBodyRef.value.scrollTop = 0
      }
      if (dialogRef.value && topDialogId() === dialogId) {
        const firstFocusable = dialogRef.value.querySelector<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )
        firstFocusable?.focus()
      }
    } else {
      unregisterDialog()
    }
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  unregisterDialog()
})
</script>

<style scoped>
.modal-neutral {
  --dialog-surface: rgb(252 252 253 / 98%);
  --dialog-control: rgb(255 255 255 / 88%);
  --dialog-rule: rgb(100 116 139 / 18%);
  --dialog-highlight: rgb(255 255 255 / 90%);
  align-items: center;
  padding: max(0.75rem, env(safe-area-inset-top)) max(0.75rem, env(safe-area-inset-right)) max(0.75rem, env(safe-area-inset-bottom)) max(0.75rem, env(safe-area-inset-left));
  background: rgb(20 23 29 / 32%);
  backdrop-filter: blur(4px);
}
.dark .modal-neutral {
  --dialog-surface: rgb(31 33 38 / 98%);
  --dialog-control: rgb(255 255 255 / 3%);
  --dialog-rule: rgb(255 255 255 / 10%);
  --dialog-highlight: rgb(255 255 255 / 5%);
  background: rgb(0 0 0 / 48%);
}
.modal-neutral .modal-content {
  min-height: 0;
  max-height: calc(100dvh - 1.5rem - env(safe-area-inset-top) - env(safe-area-inset-bottom));
  border: 1px solid var(--dialog-rule);
  border-radius: 8px;
  background: linear-gradient(125deg, var(--dialog-highlight), transparent 60%), var(--dialog-surface);
  box-shadow: 0 20px 64px rgb(0 0 0 / 18%), inset 0 1px 0 var(--dialog-highlight);
}
.modal-neutral .modal-content::before { display: none; }
.modal-neutral .modal-header {
  min-height: 3.5rem;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 1rem;
  border-color: var(--dialog-rule);
  background: transparent;
}
.modal-neutral .modal-title { font-size: 15px; line-height: 1.5; }
.modal-neutral .brand-floating-close {
  flex-shrink: 0;
  margin-right: -0.25rem;
  border: 0;
  border-radius: 6px;
  background: transparent;
  box-shadow: none;
}
.modal-neutral .brand-floating-close:hover { background: rgb(148 163 184 / 12%); }
.modal-neutral .modal-body { min-height: 0; padding: 1rem; overscroll-behavior: contain; }
.modal-neutral .modal-body-scroll { overflow-y: auto; }
.modal-neutral .modal-footer {
  padding: 0.75rem 1rem;
  border-color: var(--dialog-rule);
  background: transparent;
}
.modal-neutral .modal-footer-wrap { flex-wrap: wrap; }
.modal-neutral :deep(.input),
.modal-neutral :deep(.select-trigger),
.modal-neutral :deep(.btn-secondary) {
  border-color: var(--dialog-rule);
  border-radius: 6px;
  background: var(--dialog-control);
  box-shadow: inset 0 1px 0 var(--dialog-highlight), 0 1px 2px rgb(0 0 0 / 2%);
}
.modal-neutral :deep(.input:focus),
.modal-neutral :deep(.select-trigger-open) { border-color: rgb(var(--brand-rgb) / 50%); }
.modal-neutral :deep(.btn) { min-height: 2.5rem; border-radius: 6px; font-size: 13px; }
.modal-neutral :deep(.btn-primary) {
  background: linear-gradient(125deg, rgb(255 255 255 / 10%), transparent), rgb(var(--brand-rgb));
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 18%), 0 2px 4px rgb(var(--brand-rgb) / 12%);
}
.modal-neutral :deep(.input-label) { font-size: 13px; }
.modal-neutral :deep(.input-hint) { font-size: 11px; line-height: 1.6; }
.modal-neutral.modal-enter-active,
.modal-neutral.modal-enter-active .modal-content { transition-duration: 180ms; }
.modal-neutral.modal-leave-active,
.modal-neutral.modal-leave-active .modal-content { transition-duration: 120ms; }
.modal-neutral.modal-enter-from .modal-content,
.modal-neutral.modal-leave-to .modal-content { transform: translateY(8px); }
@media (min-width: 640px) {
  .modal-neutral .modal-content { max-height: min(90dvh, 58rem); }
  .modal-neutral .modal-header,
  .modal-neutral .modal-footer { padding-left: 1.25rem; padding-right: 1.25rem; }
  .modal-neutral .modal-body { padding: 1.25rem; }
}
@media (max-width: 639px) {
  .modal-neutral .modal-body { padding: 0.875rem; }
  .modal-neutral .modal-footer { align-items: stretch; gap: 0.5rem; }
  .modal-neutral .modal-footer-wrap > * { min-width: min(8rem, 100%); flex: 1 1 auto; }
}
@media (prefers-reduced-motion: reduce) {
  .modal-neutral.modal-enter-active,
  .modal-neutral.modal-leave-active,
  .modal-neutral.modal-enter-active .modal-content,
  .modal-neutral.modal-leave-active .modal-content { transition: none; }
  .modal-neutral.modal-enter-from .modal-content,
  .modal-neutral.modal-leave-to .modal-content { transform: none; }
}
</style>
