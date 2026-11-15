<template>
  <button
    type="button"
    :aria-label="paused ? 'Play animation' : 'Pause animation'"
    class="group fixed bottom-6 right-6 z-50 flex h-12 w-12 items-center justify-center rounded-full border border-white/20 bg-slate-900/60 text-cyan-300 shadow-lg shadow-black/20 backdrop-blur-md transition-all hover:scale-105 hover:border-cyan-300/50 hover:bg-slate-900/80 hover:text-cyan-200"
    @click="$emit('toggle')"
  >
    <svg v-if="paused" class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
      <path d="M8 5v14l11-7z" />
    </svg>
    <svg v-else class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24">
      <path d="M6 4h4v16H6zM14 4h4v16h-4z" />
    </svg>
  </button>
</template>

<script setup lang="ts">
defineProps<{ paused: boolean }>()
defineEmits<{ toggle: [] }>()
</script>
