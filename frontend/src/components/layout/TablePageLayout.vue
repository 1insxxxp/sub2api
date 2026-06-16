<template>
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <div v-if="$slots.actions" class="layout-section-fixed">
      <slot name="actions" />
    </div>

    <div v-if="$slots.filters" class="layout-section-fixed">
      <slot name="filters" />
    </div>

    <div class="layout-section-scrollable">
      <div class="card table-scroll-container">
        <slot name="table" />
      </div>
    </div>

    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.table-page-layout {
  @apply flex flex-col gap-5;
  height: calc(100vh - 64px - 4rem);
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.layout-section-scrollable {
  @apply flex min-h-0 flex-1 flex-col;
}

.table-scroll-container {
  @apply flex h-full flex-col overflow-hidden rounded-lg border border-slate-200/90 bg-white shadow-card dark:border-dark-700 dark:bg-dark-900/80;
}

.table-scroll-container :deep(.table-wrapper) {
  @apply flex-1 overflow-x-auto overflow-y-auto;
  scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content;
  display: table;
}

.table-scroll-container :deep(thead) {
  @apply sticky top-0 z-10 bg-slate-50/95 backdrop-blur-sm dark:bg-dark-800/95;
}

.table-scroll-container :deep(th) {
  @apply border-b border-slate-200 px-5 py-3.5 text-left text-xs font-semibold text-slate-500 dark:border-dark-700 dark:text-dark-300;
}

.table-scroll-container :deep(td) {
  @apply border-b border-slate-100 px-5 py-3.5 text-sm text-slate-700 dark:border-dark-800 dark:text-slate-300;
}

.table-scroll-container :deep(tbody tr) {
  @apply transition-colors duration-150 hover:bg-primary-50/50 dark:hover:bg-primary-900/10;
}

.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible border-none bg-transparent shadow-none;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply min-h-fit flex-none;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(.table-wrapper) {
  @apply overflow-visible;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
