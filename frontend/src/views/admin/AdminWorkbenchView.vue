<template>
  <AppLayout>
    <div class="admin-workbench-page admin-workbench-console min-w-0 space-y-5 sm:space-y-6">
      <div data-test="workbench-navigation" class="workbench-navigation">
        <nav
          data-test="admin-workbench-tabs"
          role="tablist"
          :aria-label="t('adminWorkbench.tabs.label')"
          class="workbench-tabs grid min-w-0 grid-cols-3 sm:flex"
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
            @click="activeTab = tab.id"
            @keydown="handleTabKeydown($event, tab.id)"
          >
            <Icon :name="tab.icon" size="sm" />
            <span class="min-w-0 break-words leading-4">{{ tab.label }}</span>
          </button>
        </nav>
      </div>

      <section
        v-if="activeTab === 'commission'"
        id="workbench-panel-commission"
        data-test="workbench-commission-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-commission"
        class="min-w-0"
      >
        <SubAdminCommissionPanel />
      </section>

      <section
        v-else-if="activeTab === 'affiliate-leaderboard'"
        id="workbench-panel-affiliate-leaderboard"
        data-test="workbench-affiliate-leaderboard-panel"
        role="tabpanel"
        aria-labelledby="workbench-tab-affiliate-leaderboard"
        class="min-w-0"
      >
        <AdminAffiliateLeaderboardPanel />
      </section>

      <KeepAlive>
        <BalanceTransferPanel
          v-if="activeTab === 'balance-transfer'"
          id="workbench-panel-balance-transfer"
          data-test="workbench-balance-transfer-panel"
          role="tabpanel"
          aria-labelledby="workbench-tab-balance-transfer"
        />
      </KeepAlive>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BalanceTransferPanel from '@/components/redeem/BalanceTransferPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import SubAdminCommissionPanel from '@/components/admin/workbench/SubAdminCommissionPanel.vue'
import AdminAffiliateLeaderboardPanel from '@/components/admin/workbench/AdminAffiliateLeaderboardPanel.vue'

const { t } = useI18n()

type WorkbenchTab = 'balance-transfer' | 'commission' | 'affiliate-leaderboard'

const activeTab = ref<WorkbenchTab>('commission')
const workbenchTabIds: WorkbenchTab[] = ['commission', 'balance-transfer', 'affiliate-leaderboard']
const workbenchTabs = computed<Array<{ id: WorkbenchTab; label: string; icon: 'gift' | 'chartBar' | 'users' }>>(() => [
  {
    id: 'commission',
    label: t('adminWorkbench.tabs.commission'),
    icon: 'chartBar'
  },
  {
    id: 'balance-transfer',
    label: t('adminWorkbench.tabs.balanceTransfer'),
    icon: 'gift'
  },
  {
    id: 'affiliate-leaderboard',
    label: t('adminWorkbench.tabs.affiliateLeaderboard'),
    icon: 'users'
  }
])

function handleTabKeydown(event: KeyboardEvent, currentTab: WorkbenchTab) {
  const currentIndex = workbenchTabIds.indexOf(currentTab)
  let nextIndex = currentIndex

  if (event.key === 'ArrowRight') {
    nextIndex = (currentIndex + 1) % workbenchTabIds.length
  } else if (event.key === 'ArrowLeft') {
    nextIndex = (currentIndex - 1 + workbenchTabIds.length) % workbenchTabIds.length
  } else if (event.key === 'Home') {
    nextIndex = 0
  } else if (event.key === 'End') {
    nextIndex = workbenchTabIds.length - 1
  } else {
    return
  }

  event.preventDefault()
  const nextTab = workbenchTabIds[nextIndex]
  activeTab.value = nextTab
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
