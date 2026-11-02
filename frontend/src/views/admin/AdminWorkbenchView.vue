<template>
  <AppLayout>
    <div class="admin-workbench-page admin-workbench-console min-w-0 space-y-5 sm:space-y-6">
      <div data-test="workbench-navigation" class="workbench-navigation">
        <nav
          data-test="admin-workbench-tabs"
          role="tablist"
          :aria-label="t('adminWorkbench.tabs.label')"
          class="workbench-tabs grid min-w-0 sm:flex sm:flex-wrap"
          :class="workbenchTabs.length === 1 ? 'grid-cols-1' : workbenchTabs.length === 3 || workbenchTabs.length > 4 ? 'grid-cols-3' : 'grid-cols-2'"
        >
          <button
            v-for="tab in workbenchTabs"
            :id="`workbench-tab-${tab.id}`"
            :key="tab.id"
            type="button"
            role="tab"
            :data-test="`workbench-tab-${tab.id}`"
            :aria-controls="`workbench-panel-${tab.id}`"
            :aria-selected="activeTab === tab.id"
            :tabindex="activeTab === tab.id ? 0 : -1"
            class="workbench-tab inline-flex min-h-11 min-w-0 items-center justify-center gap-1.5 px-1.5 py-2 text-center text-xs font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-inset sm:gap-2 sm:px-4 sm:text-sm"
            @click="selectTab(tab.id)"
            @keydown="handleTabKeydown($event, tab.id)"
          >
            <Icon :name="tab.icon" size="sm" />
            <span class="min-w-0 break-words leading-4">{{ tab.label }}</span>
          </button>
        </nav>
      </div>

      <div v-if="accessFailed" role="alert" class="flex flex-wrap items-center gap-3 text-sm text-red-600 dark:text-red-400">
        <span>{{ t('userSupport.errors.accessUnavailable') }}</span>
        <button type="button" class="btn btn-secondary min-h-11 text-xs" @click="refreshWorkbenchAccess">{{ t('userSupport.permission.retry') }}</button>
      </div>

      <section
        v-if="activeTab === 'commission' && workbenchAccess?.commission"
        id="workbench-panel-commission"
        data-test="workbench-commission-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-commission"
        class="min-w-0"
      >
        <SubAdminCommissionPanel />
      </section>

      <section
        v-else-if="activeTab === 'affiliate-leaderboard' && workbenchAccess?.affiliate_leaderboard"
        id="workbench-panel-affiliate-leaderboard"
        data-test="workbench-affiliate-leaderboard-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-affiliate-leaderboard"
        class="min-w-0"
      >
        <AdminAffiliateLeaderboardPanel />
      </section>

      <KeepAlive :key="actorIdentity">
        <BalanceTransferPanel
          v-if="activeTab === 'balance-transfer' && workbenchAccess?.balance_transfer"
          id="workbench-panel-balance-transfer"
          data-test="workbench-balance-transfer-panel"
          role="tabpanel"
          aria-labelledby="workbench-tab-balance-transfer"
        />
      </KeepAlive>

      <section
        v-if="activeTab === 'user-support' && canUseUserSupport"
        id="workbench-panel-user-support"
        data-test="workbench-user-support-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-user-support"
        class="min-w-0"
      >
        <UserSupportPanel @access-denied="revokeUserSupport" />
      </section>

      <section
        v-if="activeTab === 'model-probes' && canUseUserSupport"
        id="workbench-panel-model-probes"
        data-test="workbench-model-probes-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-model-probes"
        class="min-w-0"
      >
        <ModelProbePanel @access-denied="revokeUserSupport" />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getWorkbenchAccess, type WorkbenchAccess } from '@/api/admin/userSupport'
import AppLayout from '@/components/layout/AppLayout.vue'
import BalanceTransferPanel from '@/components/redeem/BalanceTransferPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import SubAdminCommissionPanel from '@/components/admin/workbench/SubAdminCommissionPanel.vue'
import AdminAffiliateLeaderboardPanel from '@/components/admin/workbench/AdminAffiliateLeaderboardPanel.vue'
import UserSupportPanel from '@/components/admin/workbench/UserSupportPanel.vue'
import ModelProbePanel from '@/components/admin/workbench/ModelProbePanel.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const actorIdentity = computed(() => `${auth.user?.id}:${auth.user?.role}:${auth.token ?? ''}`)
const workbenchAccess = ref<WorkbenchAccess | null>(null)
const accessFailed = ref(false)
const canUseUserSupport = computed(() => ['admin', 'sub_admin'].includes(auth.user?.role ?? '') && workbenchAccess.value?.user_support === true)
let accessVersion = 0

type WorkbenchTab = 'balance-transfer' | 'commission' | 'affiliate-leaderboard' | 'user-support' | 'model-probes'

const activeTab = ref<WorkbenchTab>('commission')
const workbenchTabs = computed<Array<{ id: WorkbenchTab; label: string; icon: 'gift' | 'chartBar' | 'users' | 'play' }>>(() => [
  ...(workbenchAccess.value?.commission ? [
  {
    id: 'commission',
    label: t('adminWorkbench.tabs.commission'),
    icon: 'chartBar'
  } as const] : []),
  ...(workbenchAccess.value?.balance_transfer ? [
  {
    id: 'balance-transfer',
    label: t('adminWorkbench.tabs.balanceTransfer'),
    icon: 'gift'
  } as const] : []),
  ...(workbenchAccess.value?.affiliate_leaderboard ? [
  {
    id: 'affiliate-leaderboard',
    label: t('adminWorkbench.tabs.affiliateLeaderboard'),
    icon: 'users'
  } as const] : []),
  ...(canUseUserSupport.value ? [
    { id: 'user-support' as const, label: t('userSupport.title'), icon: 'users' as const },
    { id: 'model-probes' as const, label: t('modelProbes.title'), icon: 'play' as const },
  ] : []),
])
const workbenchTabIds = computed(() => workbenchTabs.value.map(tab => tab.id))

async function refreshWorkbenchAccess() {
  const requestVersion = ++accessVersion
  workbenchAccess.value = null
  accessFailed.value = false
  if (!['admin', 'sub_admin'].includes(auth.user?.role ?? '')) return
  try {
    const value = await getWorkbenchAccess()
    if (requestVersion !== accessVersion) return
    workbenchAccess.value = { commission: value.commission === true, affiliate_leaderboard: value.affiliate_leaderboard === true, user_support: value.user_support === true, balance_transfer: value.balance_transfer === true }
  } catch {
    if (requestVersion !== accessVersion) return
    workbenchAccess.value = { commission: false, affiliate_leaderboard: false, user_support: false, balance_transfer: false }
    accessFailed.value = true
  }
}
watch(() => `${actorIdentity.value}:${route.fullPath}`, refreshWorkbenchAccess, { immediate: true })
watch([() => route.query.tab, workbenchTabIds], ([requested, available]) => {
  activeTab.value = typeof requested === 'string' && available.includes(requested as WorkbenchTab)
    ? requested as WorkbenchTab
    : available[0] ?? 'balance-transfer'
}, { immediate: true })
onBeforeUnmount(() => { accessVersion++ })

function revokeUserSupport() {
  if (workbenchAccess.value) workbenchAccess.value = { ...workbenchAccess.value, user_support: false }
}

function selectTab(tab: WorkbenchTab) {
  if (!workbenchTabIds.value.includes(tab)) return
  activeTab.value = tab
  void router.replace({ query: { ...route.query, tab } })
}

function handleTabKeydown(event: KeyboardEvent, currentTab: WorkbenchTab) {
  const ids = workbenchTabIds.value
  const currentIndex = ids.indexOf(currentTab)
  let nextIndex = currentIndex

  if (event.key === 'ArrowRight') {
    nextIndex = (currentIndex + 1) % ids.length
  } else if (event.key === 'ArrowLeft') {
    nextIndex = (currentIndex - 1 + ids.length) % ids.length
  } else if (event.key === 'Home') {
    nextIndex = 0
  } else if (event.key === 'End') {
    nextIndex = ids.length - 1
  } else {
    return
  }

  event.preventDefault()
  const nextTab = ids[nextIndex]
  selectTab(nextTab)
  void nextTick(() => document.getElementById(`workbench-tab-${nextTab}`)?.focus())
}
</script>

<style scoped>
.admin-workbench-console .workbench-navigation {
  border-bottom: 1px solid var(--workspace-rule);
}

.admin-workbench-console .workbench-tab {
  position: relative;
  border-bottom: 2px solid transparent;
  border-radius: 6px 6px 0 0;
  color: var(--workspace-muted);
  transition: color 150ms ease, background-color 150ms ease;
}
.admin-workbench-console .workbench-tab:hover { background: var(--workspace-hover); }
.admin-workbench-console .workbench-tab[aria-selected='true'] {
  border-bottom-color: rgb(var(--brand-rgb));
  background: rgb(var(--brand-rgb) / 0.045);
  color: rgb(var(--brand-rgb));
}
@media (prefers-reduced-motion: reduce) {
  .admin-workbench-console .workbench-tab { transition: none; }
}
</style>
