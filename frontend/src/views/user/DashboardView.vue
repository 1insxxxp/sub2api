<template>
  <AppLayout>
    <div class="user-workspace dashboard-workspace">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" :platform-quotas="platformQuotas" />
        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll" />
        <div class="grid min-w-0 grid-cols-1 gap-5 lg:grid-cols-3">
          <div class="lg:col-span-2"><UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" /></div>
          <div class="lg:col-span-1"><UserDashboardQuickActions /></div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import '@/styles/user-workspace.css'
import { ref, computed, onMounted } from 'vue'; import { useAuthStore } from '@/stores/auth'; import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'; import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'; import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'; import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'

const authStore = useAuthStore(); const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null); const loading = ref(false); const loadingUsage = ref(false); const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const modelStats = ref<ModelStat[]>([]); const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000))); const endDate = ref(formatDateLocalInput(new Date())); const granularity = ref('day')

const loadStats = async () => { loading.value = true; try { await authStore.refreshUser(); stats.value = await usageAPI.getDashboardStats() } catch (error) { console.error('Failed to load dashboard stats:', error) } finally { loading.value = false } }
const loadCharts = async () => { loadingCharts.value = true; try { const res = await Promise.all([usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value as any }), usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })]); trendData.value = res[0].trend || []; modelStats.value = res[1].models || [] } catch (error) { console.error('Failed to load charts:', error) } finally { loadingCharts.value = false } }
const loadRecent = async () => { loadingUsage.value = true; try { const res = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = res.items.slice(0, 5) } catch (error) { console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const data = await getMyPlatformQuotas(); platformQuotas.value = data.platform_quotas ?? [] } catch (error) { console.warn('Failed to load platform quotas:', error); platformQuotas.value = [] } }
const refreshAll = () => { loadStats(); loadCharts(); loadRecent(); loadPlatformQuotas() }

onMounted(() => { refreshAll() })
</script>

<style scoped>
.dashboard-workspace :deep(.dashboard-stat) { position: relative; min-height: 8.5rem; }
.dashboard-workspace :deep(.dashboard-stat > .flex) { display: block; }
.dashboard-workspace :deep(.dashboard-stat > .flex > :first-child) {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  display: grid;
  width: 1.875rem;
  height: 1.875rem;
  place-items: center;
  padding: 0;
  border: 1px solid var(--workspace-divider);
  border-radius: 7px;
  background: var(--workspace-hover);
}
.dashboard-workspace :deep(.dashboard-stat > .flex > :first-child svg) { width: 1rem; height: 1rem; }
.dashboard-workspace :deep(.dashboard-stat > .flex > :last-child) { min-width: 0; }
.dashboard-workspace :deep(.dashboard-stat > .flex > :last-child > p:first-child) { min-height: 1.5rem; padding-right: 1.75rem; color: var(--workspace-muted); }
.dashboard-workspace :deep(.dashboard-stat .text-xl) { margin-top: 0.5rem; margin-bottom: 0.375rem; color: var(--workspace-ink); font-size: 1.375rem; font-weight: 600; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.dashboard-workspace :deep(.dashboard-stat .text-xl > span) { display: inline-block; }
.dashboard-workspace :deep(.dashboard-stat .text-xl > .text-sm) { font-size: 0.6875rem; }
.dashboard-workspace :deep(.dashboard-stat .text-xs) { line-height: 1.5; overflow-wrap: anywhere; }
.dashboard-workspace :deep(.dashboard-stat-grid-secondary .dashboard-stat) { min-height: 8rem; }
.dashboard-workspace :deep(.dashboard-platform-card > .flex) { gap: 0.75rem; }
.dashboard-workspace :deep(.dashboard-platform-card .tracking-wide) { letter-spacing: 0; }
.dashboard-workspace :deep(.dashboard-model-content table) { min-width: 20rem; }
.dashboard-workspace :deep(.dashboard-model-content th),
.dashboard-workspace :deep(.dashboard-model-content td) { padding: 0.5rem 0.375rem; }
.dashboard-workspace :deep(.dashboard-recent-row) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  min-height: 4.75rem;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--workspace-divider);
}
.dashboard-workspace :deep(.dashboard-recent-icon) { display: grid; width: 2.125rem; height: 2.125rem; flex-shrink: 0; place-items: center; border: 1px solid var(--workspace-rule); border-radius: 7px; background: var(--workspace-surface); box-shadow: var(--workspace-shadow); }
.dashboard-workspace :deep(.workspace-quick-action) { display: flex; width: 100%; min-height: 4.75rem; align-items: center; gap: 0.75rem; padding: 0.75rem 0.25rem; border-bottom: 1px solid var(--workspace-divider); text-align: left; transition: background-color 160ms ease; }
.dashboard-workspace :deep(.workspace-quick-action > div:first-child) { width: 2.125rem; height: 2.125rem; border: 1px solid var(--workspace-rule); border-radius: 7px; background: var(--workspace-surface); box-shadow: var(--workspace-shadow); transform: none; }
.dashboard-workspace :deep(.workspace-quick-action > div:first-child svg) { width: 1.125rem; height: 1.125rem; }
.dashboard-workspace :deep(.workspace-quick-action:hover) { background: var(--workspace-hover); }
@media (max-width: 639px) {
  .dashboard-workspace :deep(.dashboard-date-control) { flex: 1; }
  .dashboard-workspace :deep(.dashboard-date-control > div),
  .dashboard-workspace :deep(.dashboard-date-control .date-picker-trigger) { width: 100%; }
  .dashboard-workspace :deep(.dashboard-granularity) { width: 100%; justify-content: space-between; }
  .dashboard-workspace :deep(.dashboard-granularity > span) { color: var(--workspace-muted); font-size: 0.75rem; }
  .dashboard-workspace :deep(.dashboard-stat) { padding: 0.875rem; }
  .dashboard-workspace :deep(.dashboard-recent-cost > p:first-child > span) { display: block; }
  .dashboard-workspace :deep(.dashboard-recent-cost .font-normal) { font-size: 0.6875rem; }
}
@media (prefers-reduced-motion: reduce) {
  .dashboard-workspace :deep(.workspace-quick-action),
  .dashboard-workspace :deep(.workspace-quick-action > div:first-child) { transition: none; }
}
</style>
