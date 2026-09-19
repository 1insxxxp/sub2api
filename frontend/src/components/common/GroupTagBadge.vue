<template>
  <span
    v-if="tag"
    data-test="group-tag"
    class="group-tag-badge relative inline-flex shrink-0 items-center overflow-hidden whitespace-nowrap rounded-bl-md rounded-tl-md rounded-tr-none rounded-br-none border border-white/45 px-2.5 py-1 text-[11px] font-bold leading-4 shadow-md backdrop-blur-md transition-[transform,box-shadow,background-color,border-color] duration-200 ease-out after:pointer-events-none after:absolute after:inset-y-0 after:-left-1/2 after:z-10 after:w-1/3 after:-skew-x-12 after:bg-white/25 after:opacity-0 after:transition-[transform,opacity] after:duration-500 after:ease-out hover:-translate-y-0.5 hover:shadow-lg hover:after:translate-x-[360%] hover:after:opacity-100 motion-reduce:transition-none motion-reduce:hover:translate-y-0 motion-reduce:after:transition-none"
    :class="variant === 'soft' ? 'group-tag-soft' : legacyTheme"
    :style="variant === 'soft' ? groupTagSoftStyle(tag, color || '') : legacyTheme ? undefined : groupTagStyle(tag, color || '')"
    :title="label"
  >
    <span class="relative z-20 min-w-0 truncate">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import type { GroupTag } from '@/types'
import { groupTagStyle, groupTagSoftStyle, isGroupTagPreset } from '@/utils/groupTag'

const props = withDefaults(defineProps<{ tag?: GroupTag | null; color?: string | null; variant?: 'solid' | 'soft' }>(), { variant: 'solid' })
const { t } = useI18n()
const label = computed(() => props.tag && isGroupTagPreset(props.tag) ? t(`common.groupTags.${props.tag}`) : props.tag || '')
const legacyTheme = computed(() => !props.color && props.tag && isGroupTagPreset(props.tag) ? themes[props.tag] : '')
const themes = {
  chat: 'bg-sky-500/80 text-white shadow-sky-900/10 hover:bg-sky-400/90 hover:shadow-sky-500/25 dark:bg-sky-400/75 dark:text-sky-950 dark:hover:bg-sky-300/85',
  image: 'bg-emerald-500/80 text-white shadow-emerald-900/10 hover:bg-emerald-400/90 hover:shadow-emerald-500/25 dark:bg-emerald-400/75 dark:text-emerald-950 dark:hover:bg-emerald-300/85',
  airp: 'bg-rose-500/80 text-white shadow-rose-900/10 hover:bg-rose-400/90 hover:shadow-rose-500/25 dark:bg-rose-400/75 dark:text-rose-950 dark:hover:bg-rose-300/85'
}
</script>

<style scoped>
.group-tag-badge {
  max-width: 7rem;
  animation: group-tag-in 360ms cubic-bezier(0.22, 1, 0.36, 1) both;
  transform-origin: top right;
}

.group-tag-soft {
  color: var(--tag-text);
  background: linear-gradient(135deg, rgb(255 255 255 / 62%), transparent 65%), var(--tag-background);
  border-color: var(--tag-border);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 70%), inset 1px 0 0 rgb(255 255 255 / 40%), -1px 2px 6px var(--tag-shadow);
  animation: group-tag-reveal 320ms cubic-bezier(0.22, 1, 0.36, 1) both;
  transform: none;
  backdrop-filter: none;
}

.group-tag-soft::after {
  display: block;
  left: 0;
  width: 45%;
  background: linear-gradient(90deg, transparent, rgb(255 255 255 / 65%), transparent);
  opacity: 0;
  transform: translateX(-180%) skewX(-20deg);
  animation: group-tag-shine 7s 1.2s ease-in-out infinite;
  transition: none;
}

.group-tag-soft:hover { transform: none; }

@media (hover: hover) {
  .group-option:hover .group-tag-soft,
  .group-tag-soft:hover {
    background-color: var(--tag-hover);
    border-color: var(--tag-edge);
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 80%), inset 1px 0 0 rgb(255 255 255 / 45%), -1px 3px 10px var(--tag-shadow);
  }
}

.dark .group-tag-soft {
  color: var(--tag-text-dark);
  background-image: linear-gradient(135deg, rgb(255 255 255 / 10%), transparent 65%);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 14%), inset 1px 0 0 rgb(255 255 255 / 8%), -1px 2px 8px rgb(0 0 0 / 12%);
}
.dark .group-tag-soft::after { background: linear-gradient(90deg, transparent, rgb(255 255 255 / 24%), transparent); }

@keyframes group-tag-reveal {
  from { opacity: 0; transform: translate3d(4px, -2px, 0); }
  to { opacity: 1; transform: translate3d(0, 0, 0); }
}

@keyframes group-tag-shine {
  0% { opacity: 0; transform: translateX(-180%) skewX(-20deg); }
  5% { opacity: 0.8; }
  19% { opacity: 0; transform: translateX(340%) skewX(-20deg); }
  100% { opacity: 0; transform: translateX(340%) skewX(-20deg); }
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
  .group-tag-soft,
  .group-tag-soft::after {
    animation: none;
    transition: none;
    transform: none;
  }
  .group-tag-soft::after,
  .group-tag-soft:hover::after { opacity: 0; }
}
</style>
