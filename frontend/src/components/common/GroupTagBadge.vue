<template>
  <span
    v-if="tag && themes[tag]"
    data-test="group-tag"
    class="group-tag-badge relative inline-flex shrink-0 items-center overflow-hidden whitespace-nowrap rounded-bl-md rounded-tl-md rounded-tr-none rounded-br-none border border-white/45 px-2.5 py-1 text-[11px] font-bold leading-4 shadow-md backdrop-blur-md transition-[transform,box-shadow,background-color,border-color] duration-200 ease-out after:pointer-events-none after:absolute after:inset-y-0 after:-left-1/2 after:z-10 after:w-1/3 after:-skew-x-12 after:bg-white/25 after:opacity-0 after:transition-[transform,opacity] after:duration-500 after:ease-out hover:-translate-y-0.5 hover:shadow-lg hover:after:translate-x-[360%] hover:after:opacity-100 motion-reduce:transition-none motion-reduce:hover:translate-y-0 motion-reduce:after:transition-none"
    :class="themes[tag]"
  >
    {{ t(`common.groupTags.${tag}`) }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { GroupTag } from '@/types'

defineProps<{ tag?: GroupTag | null }>()
const { t } = useI18n()
const themes = {
  chat: 'bg-sky-500/80 text-white shadow-sky-900/10 hover:bg-sky-400/90 hover:shadow-sky-500/25 dark:bg-sky-400/75 dark:text-sky-950 dark:hover:bg-sky-300/85',
  image: 'bg-emerald-500/80 text-white shadow-emerald-900/10 hover:bg-emerald-400/90 hover:shadow-emerald-500/25 dark:bg-emerald-400/75 dark:text-emerald-950 dark:hover:bg-emerald-300/85',
  airp: 'bg-rose-500/80 text-white shadow-rose-900/10 hover:bg-rose-400/90 hover:shadow-rose-500/25 dark:bg-rose-400/75 dark:text-rose-950 dark:hover:bg-rose-300/85'
}
</script>

<style scoped>
.group-tag-badge {
  animation: group-tag-in 360ms cubic-bezier(0.22, 1, 0.36, 1) both;
  transform-origin: top right;
}

@keyframes group-tag-in {
  from {
    opacity: 0;
    transform: translate3d(0.35rem, -0.35rem, 0) scale(0.9);
  }
  to {
    opacity: 1;
    transform: translate3d(0, 0, 0) scale(1);
  }
}

@media (prefers-reduced-motion: reduce) {
  .group-tag-badge {
    animation: none;
  }
}
</style>
