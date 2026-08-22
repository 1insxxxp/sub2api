<template>
  <section
    v-if="canViewCommission"
    data-test="sub-admin-commission-panel"
    class="space-y-4"
  >
    <SubAdminCommissionManagement v-if="isAdmin" />

    <div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.48fr)]">
      <SubAdminCommissionCalendar @select-day="selectedDate = $event" />
      <SubAdminCommissionDayDrawer
        :date="selectedDate"
        @close="selectedDate = null"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import SubAdminCommissionManagement from './SubAdminCommissionManagement.vue'
import SubAdminCommissionCalendar from './SubAdminCommissionCalendar.vue'
import SubAdminCommissionDayDrawer from './SubAdminCommissionDayDrawer.vue'

const authStore = useAuthStore()
const selectedDate = ref<string | null>(null)

const role = computed(() => authStore.user?.role)
const isAdmin = computed(() => role.value === 'admin')
const canViewCommission = computed(() => role.value === 'admin' || role.value === 'sub_admin')
</script>
