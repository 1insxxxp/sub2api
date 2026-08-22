<template>
  <section
    v-if="canViewCommission"
    data-test="sub-admin-commission-panel"
    class="space-y-4"
  >
    <div class="rounded-lg border border-cyan-100 bg-cyan-50/70 px-5 py-4 dark:border-cyan-500/20 dark:bg-cyan-500/10">
      <h2 class="text-base font-semibold text-cyan-950 dark:text-cyan-50">
        {{ t('adminWorkbench.commission.title') }}
      </h2>
      <p class="mt-1 text-sm text-cyan-700 dark:text-cyan-200">
        {{ t('adminWorkbench.commission.subtitle') }}
      </p>
    </div>

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
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import SubAdminCommissionManagement from './SubAdminCommissionManagement.vue'
import SubAdminCommissionCalendar from './SubAdminCommissionCalendar.vue'
import SubAdminCommissionDayDrawer from './SubAdminCommissionDayDrawer.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const selectedDate = ref<string | null>(null)

const role = computed(() => authStore.user?.role)
const isAdmin = computed(() => role.value === 'admin')
const canViewCommission = computed(() => role.value === 'admin' || role.value === 'sub_admin')
</script>
