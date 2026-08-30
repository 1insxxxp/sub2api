<template>
  <section
    data-test="sub-admin-commission-management"
    class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-5"
  >
    <div class="mb-5">
      <h2 class="text-base font-semibold text-gray-950 dark:text-white">
        {{ t('adminWorkbench.commission.settings') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
        {{ t('adminWorkbench.commission.subtitle') }}
      </p>
    </div>

    <div class="grid gap-4 lg:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)]">
      <div class="space-y-4">
        <label class="block">
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
          class="btn btn-primary w-full justify-center"
          :disabled="savingSettings"
          @click="saveSettings"
        >
          <Icon v-if="savingSettings" name="refresh" size="sm" class="animate-spin" />
          <span>{{ t('adminWorkbench.commission.saveSettings') }}</span>
        </button>

        <p class="rounded-lg bg-blue-50 px-3 py-2 text-sm leading-6 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200">
          {{ t('adminWorkbench.commission.sharedGrantsHint') }}
        </p>
      </div>

      <div class="rounded-lg border border-gray-100 p-4 dark:border-dark-800">
        <div class="mb-3 flex flex-col gap-3 min-[420px]:flex-row min-[420px]:items-center min-[420px]:justify-between">
          <h3 class="text-sm font-semibold text-gray-950 dark:text-white">
            {{ t('adminWorkbench.commission.assignedGroups') }}
          </h3>
          <button
            type="button"
            data-test="sub-admin-commission-save-grants"
            class="btn btn-secondary w-full justify-center min-[420px]:w-auto"
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
            class="flex min-w-0 items-center gap-3 rounded-lg border border-gray-200 px-3 py-2 text-sm dark:border-dark-700"
          >
            <input
              v-model="assignedGroupIDs"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
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
