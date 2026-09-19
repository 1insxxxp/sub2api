<template>
  <section
    data-test="sub-admin-commission-management"
    class="workbench-commission-management min-w-0 pb-5"
  >
    <div class="mb-3">
      <h2 class="text-base font-semibold text-gray-950 dark:text-white">
        {{ t('adminWorkbench.commission.settings') }}
      </h2>
    </div>

    <div class="grid min-w-0 gap-5 lg:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)]">
      <div class="min-w-0 space-y-2">
        <div data-test="commission-rate-controls" class="workbench-commission-controls">
          <label class="block min-w-0">
            <span class="input-label">{{ t('adminWorkbench.commission.commissionRate') }}</span>
            <input
              v-model.number="commissionRate"
              data-test="sub-admin-commission-rate"
              type="number"
              min="0"
              max="1"
              step="0.01"
              class="input"
            />
          </label>
          <button
            type="button"
            data-test="sub-admin-commission-save-settings"
            class="btn btn-primary justify-center"
            :disabled="savingSettings"
            @click="saveSettings"
          >
            <Icon v-if="savingSettings" name="refresh" size="sm" class="animate-spin" />
            <span>{{ t('adminWorkbench.commission.saveSettings') }}</span>
          </button>
        </div>

        <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
          {{ t('adminWorkbench.commission.sharedGrantsHint') }}
        </p>
      </div>

      <div class="workbench-commission-grants min-w-0">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h3 class="min-w-0 text-sm font-semibold text-gray-950 dark:text-white">
            {{ t('adminWorkbench.commission.assignedGroups') }}
          </h3>
          <button
            type="button"
            data-test="sub-admin-commission-save-grants"
            class="btn btn-secondary justify-center"
            :disabled="savingGrants"
            @click="saveGrants"
          >
            <Icon v-if="savingGrants" name="refresh" size="sm" class="animate-spin" />
            <span>{{ t('adminWorkbench.commission.saveGrants') }}</span>
          </button>
        </div>

        <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="groups.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
          {{ t('common.noGroupsAvailable') }}
        </div>
        <div v-else class="grid max-h-72 gap-2 overflow-y-auto pr-1 sm:grid-cols-2">
          <label
            v-for="group in groups"
            :key="group.id"
            class="workbench-commission-group flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-sm"
          >
            <input
              v-model="assignedGroupIDs"
              type="checkbox"
              class="sub-admin-group-checkbox rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :value="group.id"
              :data-test="`sub-admin-commission-group-${group.id}`"
            />
            <span class="min-w-0 break-words text-gray-900 dark:text-white">{{ group.name }}</span>
          </label>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import type { SubAdminCommissionGrant } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const commissionRate = ref(0)
const groups = ref<AdminGroup[]>([])
const grants = ref<SubAdminCommissionGrant[]>([])
const assignedGroupIDs = ref<number[]>([])
const loading = ref(false)
const savingSettings = ref(false)
const savingGrants = ref(false)

function syncSelectedGrants() {
  assignedGroupIDs.value = grants.value
    .filter((grant) => grant.enabled)
    .map((grant) => grant.group_id)
}

async function loadManagementData() {
  loading.value = true
  try {
    const [settings, groupList, grantList] = await Promise.all([
      adminAPI.subAdminCommission.getSettings(),
      adminAPI.groups.getAll(),
      adminAPI.subAdminCommission.listGrants()
    ])
    commissionRate.value = settings.commission_rate
    groups.value = groupList
    grants.value = grantList
    syncSelectedGrants()
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.commission.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  savingSettings.value = true
  try {
    const saved = await adminAPI.subAdminCommission.updateSettings({
      commission_rate: commissionRate.value
    })
    commissionRate.value = saved.commission_rate
    appStore.showSuccess(t('adminWorkbench.commission.saveSuccess'))
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.commission.saveFailed')))
  } finally {
    savingSettings.value = false
  }
}

async function saveGrants() {
  savingGrants.value = true
  try {
    const updated = await adminAPI.subAdminCommission.replaceGrants({
      group_ids: [...assignedGroupIDs.value]
    })
    grants.value = updated
    syncSelectedGrants()
    appStore.showSuccess(t('adminWorkbench.commission.saveSuccess'))
  } catch (error: any) {
    appStore.showError(extractApiErrorMessage(error, t('adminWorkbench.commission.saveFailed')))
  } finally {
    savingGrants.value = false
  }
}

onMounted(() => {
  void loadManagementData()
})
</script>

<style scoped>
.workbench-commission-management {
  border-bottom: 1px solid var(--workspace-divider);
}
.workbench-commission-controls {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: end;
  gap: 0.5rem;
}
.workbench-commission-group:hover { background: var(--workspace-hover); }
.workbench-commission-group:has(:checked) { background: var(--workspace-hover); }

@media (min-width: 1024px) {
  .workbench-commission-grants {
    padding-left: 1.25rem;
    border-left: 1px solid var(--workspace-divider);
  }
}

@media (max-width: 359px) {
  .workbench-commission-controls { grid-template-columns: minmax(0, 1fr); }
}

.sub-admin-group-checkbox {
  width: 16px !important;
  height: 16px !important;
  min-width: 16px;
  min-height: 16px;
  flex: 0 0 16px;
  margin: 0;
  accent-color: rgb(37 99 235);
}
</style>
