<template>
  <AppLayout>
    <div class="mx-auto flex min-h-[calc(100vh-8rem)] max-w-6xl flex-col px-4 py-6 sm:px-6">
      <section class="flex min-h-0 flex-1 flex-col rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800 sm:p-6">
        <CustomGroupsManager />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import CustomGroupsManager from '@/components/custom-groups/CustomGroupsManager.vue'
</script>
