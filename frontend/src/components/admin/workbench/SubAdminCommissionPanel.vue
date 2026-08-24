<template>
  <section
    v-if="canViewCommission"
    data-test="sub-admin-commission-panel"
    class="space-y-4"
  >
    <SubAdminCommissionManagement v-if="isAdmin" />

    <div
      data-test="commission-calendar-layout"
      class="commission-calendar-layout grid items-start gap-4"
    >
      <SubAdminCommissionCalendar @select-day="handleDaySelect" />
      <SubAdminCommissionDayDrawer
        :date="selectedDate"
        :day="selectedDay"
        @close="closeDayDrawer"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { SubAdminCommissionCalendarDay } from '@/api/admin'
import SubAdminCommissionManagement from './SubAdminCommissionManagement.vue'
import SubAdminCommissionCalendar from './SubAdminCommissionCalendar.vue'
import SubAdminCommissionDayDrawer from './SubAdminCommissionDayDrawer.vue'

const authStore = useAuthStore()
const selectedDate = ref<string | null>(null)
const selectedDay = ref<SubAdminCommissionCalendarDay | null>(null)

const role = computed(() => authStore.user?.role)
const isAdmin = computed(() => role.value === 'admin')
const canViewCommission = computed(() => role.value === 'admin' || role.value === 'sub_admin')

function handleDaySelect(date: string, day: SubAdminCommissionCalendarDay) {
  selectedDate.value = date
  selectedDay.value = day
}

function closeDayDrawer() {
  selectedDate.value = null
  selectedDay.value = null
}
</script>
