<template>
  <div
    data-testid="default-user-avatar"
    :class="['default-user-avatar', sizeClass]"
    aria-hidden="true"
  >
    <span class="default-user-avatar__glow" />
    <svg
      class="default-user-avatar__icon"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M15.75 7.25a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z"
        stroke="currentColor"
        stroke-width="1.65"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M4.75 19.25c.92-3.35 3.55-5.25 7.25-5.25s6.33 1.9 7.25 5.25"
        stroke="currentColor"
        stroke-width="1.65"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  size?: 'sm' | 'md' | 'lg' | 'xl' | '2xl'
}>(), {
  size: 'md'
})

const sizeClass = computed(() => `default-user-avatar--${props.size}`)
</script>

<style scoped>
.default-user-avatar {
  position: relative;
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid rgba(191, 219, 254, 0.72);
  background:
    radial-gradient(circle at 28% 16%, rgba(255, 255, 255, 0.34), transparent 30%),
    linear-gradient(135deg, var(--brand-700, #1d4ed8), var(--brand-500, #3b82f6) 52%, var(--brand-cyan, #06b6d4));
  color: rgba(255, 255, 255, 0.94);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.32) inset,
    0 12px 26px rgba(37, 99, 235, 0.22);
}

.default-user-avatar__glow {
  position: absolute;
  inset: 12%;
  border-radius: 9999px;
  background: radial-gradient(circle, rgba(255, 255, 255, 0.18), transparent 68%);
  filter: blur(1px);
}

.default-user-avatar__icon {
  position: relative;
  z-index: 1;
  width: 58%;
  height: 58%;
  filter: drop-shadow(0 1px 2px rgba(15, 23, 42, 0.16));
}

.default-user-avatar--sm {
  width: 2rem;
  height: 2rem;
  border-radius: 0.75rem;
}

.default-user-avatar--md {
  width: 2.75rem;
  height: 2.75rem;
  border-radius: 1rem;
}

.default-user-avatar--lg {
  width: 4rem;
  height: 4rem;
  border-radius: 1.15rem;
}

.default-user-avatar--xl {
  width: 5rem;
  height: 5rem;
  border-radius: 1.35rem;
}

.default-user-avatar--2xl {
  width: 6rem;
  height: 6rem;
  border-radius: 1.5rem;
}

:global(.dark) .default-user-avatar {
  border-color: rgba(96, 165, 250, 0.26);
  background:
    radial-gradient(circle at 28% 16%, rgba(255, 255, 255, 0.18), transparent 30%),
    linear-gradient(135deg, #1e40af, #2563eb 48%, #0891b2);
  color: rgba(239, 246, 255, 0.96);
  box-shadow:
    0 1px 0 rgba(255, 255, 255, 0.1) inset,
    0 14px 30px rgba(37, 99, 235, 0.28);
}
</style>
