<template>
  <section
    v-if="canViewCommission"
    data-test="sub-admin-commission-panel"
    class="min-w-0 space-y-4"
  >
    <div
      data-test="commission-calendar-layout"
      class="commission-calendar-layout grid min-w-0 items-start gap-4"
    >
      <SubAdminCommissionCalendar :refresh-key="refreshKey" @select-day="handleDaySelect" @scope-change="closeDayDrawer">
        <template v-if="isAdmin" #actions>
          <SubAdminCommissionManagement @changed="refreshCommission" />
        </template>
      </SubAdminCommissionCalendar>
      <SubAdminCommissionDayDrawer
        :date="selectedDate"
        :day="selectedDay"
        :group-id="selectedGroupID"
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
const refreshKey = ref(0)
const selectedDate = ref<string | null>(null)
const selectedDay = ref<SubAdminCommissionCalendarDay | null>(null)
const selectedGroupID = ref<number | null>(null)

const role = computed(() => authStore.user?.role)
const isAdmin = computed(() => role.value === 'admin')
const canViewCommission = computed(() => role.value === 'admin' || role.value === 'sub_admin')

function handleDaySelect(date: string, day: SubAdminCommissionCalendarDay, groupID: number | null = null) {
  selectedDate.value = date
  selectedDay.value = day
  selectedGroupID.value = groupID
}

function refreshCommission() {
  closeDayDrawer()
  refreshKey.value += 1
}

function closeDayDrawer() {
  selectedDate.value = null
  selectedDay.value = null
  selectedGroupID.value = null
}
</script>
